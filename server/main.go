package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"net"
	"os"
	sig "os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NdoleStudio/lemonsqueezy-go"
	"github.com/google/uuid"
	"github.com/jackpal/gateway"
	"github.com/joho/godotenv"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/iptables"
	"github.com/tunnels-is/tunnels/setcap"
	"github.com/tunnels-is/tunnels/signal"
	"github.com/tunnels-is/tunnels/types"
	"github.com/tunnels-is/tunnels/version"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

var (
// twoFactorKey       = os.Getenv("TWO_FACTOR_KEY")
// googleClientID     = os.Getenv("GOOGLE_CLIENT_ID")
// googleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
// googleRedirectURL  = "http://localhost:3000/auth/google/callback"
// oauthStateString   = "random-pseudo-state"
)

const (
// authHeader        = "X-Auth-Token"
// googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo?access_token="
)

var (
	CTX          atomic.Pointer[context.Context]
	Cancel       atomic.Pointer[context.CancelFunc]
	Config       atomic.Pointer[types.ServerConfig]
	APITLSConfig atomic.Pointer[tls.Config]
	PrivKey      any
	SignKey      any
	PubKey       any
	KeyPair      atomic.Pointer[tls.Certificate]
	lc           atomic.Pointer[lemonsqueezy.Client]
)

var (
	LANEnabled   bool
	VPNEnabled   bool
	AUTHEnabled  bool
	DNSEnabled   bool
	BBOLTEnabled bool
	logger       *slog.Logger

	slots int

	dataSocketFD int
	rawUDPSockFD int
	rawTCPSockFD int
	InterfaceIP  net.IP
	TCPRWC       io.ReadWriteCloser
	UDPRWC       io.ReadWriteCloser

	clientCoreMappings [math.MaxUint16 + 1]*UserCoreMapping
	portToCoreMapping  [math.MaxUint16 + 1]*PortRange
	coreMutex          = sync.Mutex{}

	VPLNetwork  *net.IPNet
	DHCPMapping [math.MaxUint16 + 1]*types.DHCPRecord

	VPLIPToCore         = make([][][][]*UserCoreMapping, 255)
	lanFirewallDisabled bool
	disableLogs         bool
)

func GetVPLCM(ip [4]byte) {
}

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

func ADMIN(x ...any) {
	if !disableLogs {
		log.Println(x...)
	}
}

func main() {
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "show version and exit")

	configFlag := flag.Bool("config", false, "This command runs the server and creates a config + certificates")
	certsOnly := flag.Bool("certs", false, "This command generates certificates and exits")
	silent := flag.Bool("silent", false, "This command disables logging")
	disableLogs = *silent
	logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger = slog.New(logHandler)
	slog.SetDefault(logger)

	flag.Parse()
	if showVersion {
		fmt.Println(version.Version)
		os.Exit(1)
	}

	if *configFlag {
		logger.Info("generating config")
		makeConfigAndCerts()
	}
	if *certsOnly {
		logger.Info("generating certs")
		makeCertsOnly()
		os.Exit(1)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	err := LoadServerConfig("./config.json")
	if err != nil {
		panic(err)
	}

	config := Config.Load()
	err = godotenv.Load(".env")
	if err != nil {
		if config.SecretStore == types.EnvStore {
			logger.Error("no .env file found")
			os.Exit(1)
		}
	}

	AUTHEnabled = slices.Contains(config.Features, types.AUTH)
	LANEnabled = slices.Contains(config.Features, types.LAN)
	DNSEnabled = slices.Contains(config.Features, types.DNS)
	VPNEnabled = slices.Contains(config.Features, types.VPN)
	BBOLTEnabled = slices.Contains(config.Features, types.BBOLT)

	err = loadCertificatesAndTLSSettings()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	CTX.Store(&ctx)
	Cancel.Store(&cancel)

	if AUTHEnabled {
		if BBOLTEnabled {
			err = ConnectToBBoltDB("tunnels.db")
			if err != nil {
				logger.Error("unable to connect to bbolt", slog.Any("err", err))
				os.Exit(1)
			}

		} else {
			err = ConnectToDB(loadSecret("DBurl"))
			if err != nil {
				logger.Error("unable to connect to mongodb", slog.Any("err", err))
				os.Exit(1)
			}
		}

		// Tunnels public network specific
		if loadSecret("PayKey") != "" {
			lemonClient := lemonsqueezy.New(lemonsqueezy.WithAPIKey(loadSecret("PayKey")))
			if lemonClient == nil {
				logger.Error("Unable to initialize lemon queezy client", slog.Any("err", err))
				os.Exit(1)
			}
			lc.Store(lemonClient)
			go signal.NewSignal("SUBSCANNER", ctx, cancel, 12*time.Hour, goroutineLogger, scanSubs)
		}

		if *configFlag {
			err = initializeNewServer()
			if err != nil {
				logger.Error("unable to create admin user", slog.Any("err", err))
				os.Exit(1)
			}
		}
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

		go signal.NewSignal("DATA", ctx, cancel, 1*time.Second, goroutineLogger, DataSocketListener)
		go signal.NewSignal("TCP", ctx, cancel, 1*time.Second, goroutineLogger, ExternalTCPListener)
		go signal.NewSignal("UDP", ctx, cancel, 1*time.Second, goroutineLogger, ExternalUDPListener)
		go signal.NewSignal("PING", ctx, cancel, 10*time.Second, goroutineLogger, pingActiveUsers)
	}

	go signal.NewSignal("API", ctx, cancel, 1*time.Second, goroutineLogger, launchAPIServer)

	go signal.NewSignal("CONFIG", ctx, cancel, 30*time.Second, goroutineLogger, func() {
		_ = LoadServerConfig("./config.json")
	})

	logger.Info("Tunnels ready")
	quit := make(chan os.Signal, 1)
	sig.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Tunnels server exiting")
}

func goroutineLogger(msg string) {
	logger.Debug(msg)
}

func validateConfig(Config *types.ServerConfig) (err error) {
	if Config.UserMaxConnections < 1 {
		Config.UserMaxConnections = 2
	}
	if Config.PingTimeoutMinutes < 2 {
		Config.PingTimeoutMinutes = 2
	}
	if Config.DHCPTimeoutHours < 1 {
		Config.DHCPTimeoutHours = 1
	}

	if len(Config.Features) == 0 {
		return fmt.Errorf("no features enbaled")
	}

	if Config.SecretStore == "" {
		Config.SecretStore = types.EnvStore
	}

	return nil
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
	priv, privB, err := crypt.LoadPrivateKey(loadSecret("KeyPem"))
	if err != nil {
		return err
	}
	pub, pubB, err := crypt.LoadPublicKey(loadSecret("CertPem"))
	if err != nil {
		return err
	}
	PrivKey = priv
	PubKey = pub
	if AUTHEnabled && VPNEnabled {
		SignKey = pub
	} else {
		sign, _, err := crypt.LoadPublicKey(loadSecret("SignPem"))
		if err != nil {
			return err
		}
		SignKey = sign
	}
	tlscert, err := tls.X509KeyPair(pubB, privB)
	if err != nil {
		return err
	}
	KeyPair.Store(&tlscert)

	APITLSConfig.Store(&tls.Config{
		MinVersion:       tls.VersionTLS13,
		MaxVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.CurveP521},
		Certificates:     []tls.Certificate{*KeyPair.Load()},
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
	slots = Config.ServerBandwidthMbps / Config.UserBandwidthMbps
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
		PR.StartPort = currentPort
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

	err = LoadServerConfig("./config.json")
	if err != nil {
		Config := &types.ServerConfig{
			Features: []types.Feature{
				types.LAN,
				types.VPN,
				types.AUTH,
				types.DNS,
				types.BBOLT,
			},
			VPNIP:     interfaceIP,
			VPNPort:   "444",
			APIIP:     interfaceIP,
			APIPort:   "443",
			NetAdmins: []string{},
			Hostname:  "tunnels.local",
			Lan: &types.Network{
				Tag:     "lan",
				Network: "10.0.0.0/16",
			},
			Routes: []*types.Route{
				{Address: "10.0.0.0/16", Metric: "0"},
			},
			SubNets:             []*types.Network{},
			DisableLanFirewall:  false,
			StartPort:           2000,
			EndPort:             65530,
			UserMaxConnections:  10,
			InternetAccess:      true,
			LocalNetworkAccess:  false,
			ServerBandwidthMbps: 1000,
			UserBandwidthMbps:   10,
			DNSRecords:          []*types.DNSRecord{},
			DNSServers:          []string{},
			SecretStore:         "config",
			// secrets
			DBurl:        "",
			AdminAPIKey:  uuid.NewString(),
			TwoFactorKey: strings.ReplaceAll(uuid.NewString(), "-", ""),
			CertPem:      "./cert.pem",
			KeyPem:       "./key.pem",
			SignPem:      "./sign.pem",
		}
		f, err := os.Create(ep + "config.json")
		if err != nil {
			panic(err)
		}
		defer func() {
			_ = f.Close()
		}()
		encoder := json.NewEncoder(f)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(Config); err != nil {
			panic(err)
		}
	}

	makeCerts(ep, interfaceIP)
}

func makeCerts(execPath string, IP string) {
	_, err := certs.MakeCertV2(
		certs.ECDSA,
		filepath.Join(execPath, "cert.pem"),
		filepath.Join(execPath, "key.pem"),
		[]string{IP},
		[]string{""},
		"",
		time.Time{},
		true,
	)
	if err != nil {
		panic(err)
	}
}

func makeCertsOnly() {
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
	makeCerts(ep, interfaceIP)
}

func initializeNewServer() error {
	user, err := DB_findUserByEmail("admin")
	if err != nil {
		return err
	}
	if user != nil {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), 13)
	if err != nil {
		return err
	}

	newUser := new(User)
	newUser.ID = primitive.NewObjectID()
	newUser.Password = string(hash)
	newUser.IsAdmin = true
	newUser.IsManager = true
	newUser.AdditionalInformation = ""
	newUser.Email = "admin"
	newUser.ResetCode = uuid.NewString()
	newUser.Updated = time.Now()
	newUser.Trial = false
	newUser.APIKey = uuid.NewString()
	newUser.Updated = time.Now()
	newUser.SubExpiration = time.Now().AddDate(100, 0, 0)
	newUser.Groups = make([]primitive.ObjectID, 0)
	newUser.Tokens = make([]*DeviceToken, 0)
	err = DB_CreateUser(newUser)
	if err != nil {
		return err
	}

	logger.Info("ADMIN RESET CODE", "code", newUser.ResetCode)

	c := Config.Load()
	keyBytes, err := os.ReadFile(c.CertPem)
	if err != nil {
		return err
	}
	return DB_CreateServer(&types.Server{
		ID:       primitive.NewObjectID(),
		Tag:      "tunnels",
		Country:  "tunnels",
		IP:       c.VPNIP,
		Port:     c.APIPort,
		DataPort: c.VPNPort,
		PubKey:   string(keyBytes),
		Groups:   []primitive.ObjectID{},
	})
}
