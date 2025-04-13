package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"log/slog"
	"math"
	"net"
	"os"
	sig "os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"slices"

	"github.com/jackpal/gateway"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/iptables"
	"github.com/tunnels-is/tunnels/setcap"
	"github.com/tunnels-is/tunnels/signal"
	"github.com/tunnels-is/tunnels/types"
	"golang.org/x/net/quic"
)

var (
	serverConfigPath = "./server.json"
	slots            int

	publicPath  string
	privatePath string
	// publicSigningCert  *x509.Certificate
	// publicSigningKey   *rsa.PublicKey
	// controlCertificate tls.Certificate
	// quicConfig *quic.Config

	dataSocketFD int
	rawUDPSockFD int
	rawTCPSockFD int
	InterfaceIP  net.IP
	TCPRWC       io.ReadWriteCloser
	UDPRWC       io.ReadWriteCloser

	toUserChannelMonitor   = make(chan int, 200000)
	fromUserChannelMonitor = make(chan int, 200000)

	PortMappingResponseDurations = time.Duration(30 * time.Second)

	ClientCoreMappings [math.MaxUint16 + 1]*UserCoreMapping
	PortToCoreMapping  [math.MaxUint16 + 1]*PortRange
	COREm              = sync.Mutex{}

	VPLNetwork  *net.IPNet
	DHCPMapping [math.MaxUint16 + 1]*types.DHCPRecord

	VPLIPToCore         = make([][][][]*UserCoreMapping, 255)
	LanFirewallDisabled bool
)
var disableLogs bool

func LOG(x ...any) {
	if !disableLogs {
		log.Println(x...)
	}
}

func INFO(x ...any) {
	if !disableLogs {
		log.Println(x...)
	}
}

func WARN(x ...any) {
	if !disableLogs {
		log.Println(x...)
	}
}

func ERR(x ...any) {
	if !disableLogs {
		log.Println(x...)
	}
}

var CTX atomic.Pointer[context.Context]
var Cancel atomic.Pointer[context.CancelFunc]
var Config atomic.Pointer[types.ServerConfig]
var APITLSConfig atomic.Pointer[tls.Config]
var QUICConfig atomic.Pointer[quic.Config]
var PrivKey atomic.Pointer[rsa.PrivateKey]
var PubKey atomic.Pointer[rsa.PublicKey]
var KeyPair atomic.Pointer[tls.Certificate]

var (
	LANEnabled  bool
	VPNEnabled  bool
	AUTHEnabled bool
	DNSEnabled  bool
)
var logger *slog.Logger
var numShards = 5

func main() {

	config := flag.Bool("config", false, "This command creates a new server config and certificates")
	flag.Parse()
	if config != nil && *config {
		fmt.Println("new config")
		makeConfigAndCerts()
		os.Exit(1)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}) // Use LevelInfo in prod
	logger = slog.New(logHandler)
	slog.SetDefault(logger)

	err := LoadServerConfig("./config.json")
	if err != nil {
		panic(err)
	}

	Config := Config.Load()
	AUTHEnabled = slices.Contains(Config.Features, types.AUTH)
	LANEnabled = slices.Contains(Config.Features, types.LAN)
	DNSEnabled = slices.Contains(Config.Features, types.DNS)
	VPNEnabled = slices.Contains(Config.Features, types.VPN)

	err = loadCertificatesAndTLSSettings()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	CTX.Store(&ctx)
	Cancel.Store(&cancel)

	if AUTHEnabled {
		initAuth()
	}

	if DNSEnabled {
		// TODO: in development
	}

	if LANEnabled || VPNEnabled {
		if LANEnabled {
			err = initializeLAN()
			if err != nil {
				ERR("unable to initialize VPL")
				os.Exit(1)
			}
		}

		if VPNEnabled {
			initializeVPN()
		}

		go signal.NewSignal("PING", ctx, cancel, goroutineLogger, ControlSocketListener)
		go signal.NewSignal("PING", ctx, cancel, goroutineLogger, DataSocketListener)
		go signal.NewSignal("TCP", ctx, cancel, goroutineLogger, ExternalTCPListener)
		go signal.NewSignal("UDP", ctx, cancel, goroutineLogger, ExternalUDPListener)
		go signal.NewSignal("PING", ctx, cancel, goroutineLogger, pingActiveUsers)
	}

	go signal.NewSignal("API", ctx, cancel, goroutineLogger, StartAPI)

	go signal.NewSignal("CONFIG", ctx, cancel, goroutineLogger, func() {
		_ = LoadServerConfig("./config.json")
	})

	quit := make(chan os.Signal, 1)
	sig.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	// TODO: log
}

func goroutineLogger(msg string) {
	fmt.Println(msg)
}

func validateConfig(Config *types.ServerConfig) (err error) {
	if Config.UserMaxConnections < 1 {
		Config.UserMaxConnections = 2
	}

	if len(Config.Features) == 0 {
		return fmt.Errorf("no features enbaled")
	}

	return nil
}
func getShardIndex(key string) int {
	checksum := crc32.ChecksumIEEE([]byte(key))
	return int(checksum % uint32(numShards))
}

func initAuth() {
	logger.Info("Initializing databases...")
	if err := initDBs(logger); err != nil {
		logger.Error("Fatal: Failed to initialize databases", slog.Any("error", err))
		os.Exit(1)
	}
	defer closeDBs(logger)
	logger.Info("Setting up Google OAuth...")
	if err := setupGoogleOAuth(); err != nil {
		logger.Error("Fatal: Failed to setup Google OAuth", slog.Any("error", err))
		os.Exit(1)
	}

}

func LoadServerConfig(path string) (err error) {
	var nb []byte
	nb, err = os.ReadFile(path)
	if err != nil {
		return
	}
	C := new(types.ServerConfig)
	err = json.Unmarshal(nb, &C)
	if err != nil {
		return
	}
	err = validateConfig(C)
	if err != nil {
		return
	}
	Config.Store(C)
	return
}

func loadCertificatesAndTLSSettings() (err error) {
	Config := Config.Load()
	priv, privB, err := loadPrivateKey(Config.KeyPem)
	pub, pubB, err := loadPublicKey(Config.CertPem)
	PrivKey.Store(priv)
	PubKey.Store(pub)
	tlscert, err := tls.X509KeyPair(pubB, privB)
	if err != nil {
		log.Fatalf("Failed to load key pair for TLS: %v", err)
	}
	KeyPair.Store(&tlscert)

	APITLSConfig.Store(&tls.Config{
		MinVersion:       tls.VersionTLS13,
		MaxVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.CurveP521},
		Certificates:     []tls.Certificate{*KeyPair.Load()},
	})

	QUICConfig.Store(&quic.Config{
		TLSConfig:                APITLSConfig.Load(),
		RequireAddressValidation: false,
		HandshakeTimeout:         time.Duration(10 * time.Second),
		KeepAlivePeriod:          0,
		MaxUniRemoteStreams:      500,
		MaxBidiRemoteStreams:     500,
		MaxStreamReadBufferSize:  70000,
		MaxStreamWriteBufferSize: 70000,
		MaxConnReadBufferSize:    70000,
		MaxIdleTimeout:           60 * time.Second,
	})

	return nil
}
func initializeVPN() {
	err := setcap.CheckCapabilities()
	if err != nil {
		ERR("Tunnels server is missing capabilities, err:", err)
		os.Exit(1)
	}
	Config := Config.Load()

	var existed bool
	err, existed = iptables.SetIPTablesRSTDropFilter(Config.VPNIP)
	if err != nil {
		ERR("Error applying iptables rule: ", err)
		os.Exit(1)
	}
	if !existed {
		INFO("> added iptables rule")
	}

	InterfaceIP = net.ParseIP(Config.VPNIP)
	if InterfaceIP == nil {
		ERR("Interface IP not parsable")
		os.Exit(1)
	}
	InterfaceIP = InterfaceIP.To4()

	_, _, err = createRawTCPSocket()
	if err != nil {
		panic(err)
	}
	_, _, err = createRawUDPSocket()
	if err != nil {
		panic(err)
	}

	err = GeneratePortAllocation()
	if err != nil {
		panic(err)
	}
	GenerateVPLCoreMappings()
}
func initializeLAN() (err error) {
	err = generateDHCPMap()
	if err != nil {
		return
	}
	Config := Config.Load()

	if Config.Lan != nil {
		LanFirewallDisabled = Config.DisableLanFirewall
	}
	return
}
func GenerateVPLCoreMappings() {
	VPLIPToCore[10] = make([][][]*UserCoreMapping, 11)
	VPLIPToCore[10][0] = make([][]*UserCoreMapping, 256)

	for ii := range 256 {
		VPLIPToCore[10][0][ii] = make([]*UserCoreMapping, 256)

		for iii := range 256 {
			VPLIPToCore[10][0][ii][iii] = nil
		}
	}
}
func GeneratePortAllocation() (err error) {
	Config := Config.Load()
	slots = Config.BandwidthMbps / Config.UserBandwidthMbps
	portPerUser := (Config.EndPort - Config.StartPort) / slots

	defer func() {
		BasicRecover()
		if err != nil {
			panic(err)
		}
	}()

	currentPort := uint16(Config.StartPort)

	for range slots {
		PR := new(PortRange)
		PR.StartPort = uint16(currentPort)
		PR.EndPort = PR.StartPort + uint16(portPerUser)

		for i := PR.StartPort; i <= PR.EndPort; i++ {

			if i < PR.StartPort {
				return errors.New("start port is too small")
			} else if i > PR.EndPort {
				return errors.New("end port is too big")
			}

			if PortToCoreMapping[i] != nil {
				if PortToCoreMapping[i].StartPort < PR.StartPort {
					return errors.New("start port is too small")
				}
				if PortToCoreMapping[i].StartPort < PR.EndPort {
					return errors.New("end port is too big")
				}
			}

			PortToCoreMapping[i] = PR
		}

		currentPort = PR.EndPort + 1
	}

	return nil
}

func makeConfigAndCerts() {
	ep, err := os.Executable()
	if err != nil {
		panic(err)
	}
	eps := strings.Split(ep, "/")
	ep = strings.Join(eps[:len(eps)-1], "/")
	ep += "/"

	IFIP, err := gateway.DiscoverInterface()
	if err != nil {
		panic(err)
	}
	interfaceIP := IFIP.String()
	Config := &types.ServerConfig{
		Features: []types.Feature{
			types.LAN,
			types.VPN,
			types.AUTH,
			types.DNS,
		},
		VPNIP:       interfaceIP,
		VPNPort:     "444",
		CertPem:     "./cert.pem",
		KeyPem:      "./key.pem",
		APIIP:       interfaceIP,
		APIPort:     "443",
		AdminApiKey: "",
		Admins:      []string{},
		NetAdmins:   []string{},
		Hostname:    "tunnels.local",
		Lan: &types.Network{
			Tag:     "lan",
			Network: "10.0.0.0/16",
		},
		Routes: []*types.Route{
			{Address: "10.0.0.0/16", Metric: "0"},
		},
		SubNets:            []*types.Network{},
		DisableLanFirewall: false,
		StartPort:          2000,
		EndPort:            65530,
		UserMaxConnections: 10,
		InternetAccess:     false,
		LocalNetworkAccess: false,
		BandwidthMbps:      1000,
		UserBandwidthMbps:  10,
		DNSAllowCustomOnly: false,
		DNS:                []*types.DNSRecord{},
		DNSServers:         []string{},
	}
	f, err := os.Create(ep + "server.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(Config); err != nil {
		panic(err)
	}

	_, err = certs.MakeCertV2(
		certs.ECDSA,
		filepath.Join(ep, "cert.pem"),
		filepath.Join(ep, "key.pem"),
		[]string{interfaceIP},
		[]string{""},
		"",
		time.Time{},
		true,
	)
	if err != nil {
		panic(err)
	}

}
