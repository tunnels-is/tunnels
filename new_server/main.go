package main

import (
	"context"
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
	"github.com/tunnels-is/tunnels/setcap"
	"github.com/tunnels-is/tunnels/signal"
	"github.com/tunnels-is/tunnels/types"
	"golang.org/x/net/quic"
	"golang.org/x/oauth2"
)

var (
	twoFactorKey       = os.Getenv("TWO_FACTOR_KEY")
	googleClientID     = os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	emailKey           = os.Getenv("SENDMAIL_KEY")
	googleRedirectURL  = "http://localhost:3000/auth/google/callback"
	oauthStateString   = "random-pseudo-state"
)

const (
	authHeader        = "X-Auth-Token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo?access_token="
)

var (
	errUnauthorized = errors.New("unauthorized")
	errForbidden    = errors.New("forbidden")
	errNotFound     = errors.New("not found")
	errOTPRequired  = errors.New("otp required")
	errInvalidOTP   = errors.New("invalid otp code")
)

var (
	CTX          atomic.Pointer[context.Context]
	Cancel       atomic.Pointer[context.CancelFunc]
	Config       atomic.Pointer[types.ServerConfig]
	APITLSConfig atomic.Pointer[tls.Config]
	QUICConfig   atomic.Pointer[quic.Config]
	PrivKey      atomic.Pointer[any]
	PubKey       atomic.Pointer[any]
	KeyPair      atomic.Pointer[tls.Certificate]
)

var (
	LANEnabled        bool
	VPNEnabled        bool
	AUTHEnabled       bool
	DNSEnabled        bool
	logger            *slog.Logger
	numShards         = 5
	twoFactorAppName  = "tunnels"
	pendingTwoFactor  = sync.Map{}
	googleOauthConfig *oauth2.Config
	serverConfigPath  = "./server.json"
	slots             int

	publicPath  string
	privatePath string

	dataSocketFD int
	rawUDPSockFD int
	rawTCPSockFD int
	InterfaceIP  net.IP
	TCPRWC       io.ReadWriteCloser
	UDPRWC       io.ReadWriteCloser

	toUserChannelMonitor   = make(chan int, 200000)
	fromUserChannelMonitor = make(chan int, 200000)

	portMappingResponseDurations = time.Duration(30 * time.Second)

	clientCoreMappings [math.MaxUint16 + 1]*UserCoreMapping
	portToCoreMapping  [math.MaxUint16 + 1]*PortRange
	coreMutex          = sync.Mutex{}

	VPLNetwork  *net.IPNet
	DHCPMapping [math.MaxUint16 + 1]*types.DHCPRecord

	VPLIPToCore         = make([][][][]*UserCoreMapping, 255)
	lanFirewallDisabled bool
	disableLogs         bool
)

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
		go signal.NewSignal("2FCLEANER", ctx, cancel, 5*time.Minute, goroutineLogger, cleanTwoFactorPendingMap)
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

		go signal.NewSignal("CONTROL", ctx, cancel, 1*time.Second, goroutineLogger, ControlSocketListener)
		go signal.NewSignal("DATA", ctx, cancel, 1*time.Second, goroutineLogger, DataSocketListener)
		go signal.NewSignal("TCP", ctx, cancel, 1*time.Second, goroutineLogger, ExternalTCPListener)
		go signal.NewSignal("UDP", ctx, cancel, 1*time.Second, goroutineLogger, ExternalUDPListener)
		go signal.NewSignal("PING", ctx, cancel, 10*time.Second, goroutineLogger, pingActiveUsers)
	}

	go signal.NewSignal("API", ctx, cancel, 1*time.Second, goroutineLogger, StartAPI)

	go signal.NewSignal("CONFIG", ctx, cancel, 10*time.Second, goroutineLogger, func() {
		_ = LoadServerConfig("./config.json")
	})

	logger.Info("Tunnels ready")
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

	if Config.TwoFactorKey == "" {
		return fmt.Errorf("Two factor authentication encryption key no set")
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
	if err != nil {
		panic(err)
	}
	pub, pubB, err := loadPublicKey(Config.CertPem)
	if err != nil {
		panic(err)
	}
	PrivKey.Store(&priv)
	PubKey.Store(&pub)
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

	// var existed bool
	// err, existed = iptables.SetIPTablesRSTDropFilter(Config.VPNIP)
	// if err != nil {
	// 	ERR("Error applying iptables rule: ", err)
	// 	os.Exit(1)
	// }
	// if !existed {
	// 	INFO("> added iptables rule")
	// }

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
		lanFirewallDisabled = Config.DisableLanFirewall
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

			if portToCoreMapping[i] != nil {
				if portToCoreMapping[i].StartPort < PR.StartPort {
					return errors.New("start port is too small")
				}
				if portToCoreMapping[i].StartPort < PR.EndPort {
					return errors.New("end port is too big")
				}
			}

			portToCoreMapping[i] = PR
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
	f, err := os.Create(ep + "config.json")
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

func recoverAndLog() {
	r := recover()
	if r != nil {
		logger.Error("PANIC", slog.Any("stacktrace", getStacktraceLines(3)))
	}
}

func getStacktraceLines(skipFrames int) []string {
	buf := make([]byte, 4086)

	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	lines := strings.Split(stack, "\n")

	startIndex := 1 + (skipFrames * 2)
	if startIndex >= len(lines) {
		startIndex = 1
	}

	cleanedLines := make([]string, 0, len(lines)-startIndex)
	for _, line := range lines[startIndex:] {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}

	return cleanedLines
}
