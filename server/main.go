package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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
	"gopkg.in/yaml.v3"
)

var (
	CTX          atomic.Pointer[context.Context]
	Cancel       atomic.Pointer[context.CancelFunc]
	Config       atomic.Pointer[types.ServerConfig]
	APITLSConfig atomic.Pointer[tls.Config]
	KeyPair      atomic.Pointer[tls.Certificate]
	PrivKey      any
	SignKey      any
	PubKey       any

	coreMutex          = sync.Mutex{}
	slots              int
	VPLNetwork         *net.IPNet
	clientCoreMappings [math.MaxUint16 + 1]*UserCoreMapping
	portToCoreMapping  [math.MaxUint16 + 1]*PortRange
	DHCPMapping        [math.MaxUint16 + 1]*types.DHCPRecord
	VPLIPToCore        = make([][][][]*UserCoreMapping, 255)

	LANEnabled   bool
	VPNEnabled   bool
	AUTHEnabled  bool
	DNSEnabled   bool
	BBOLTEnabled bool
	SOCKSEnabled bool

	lanFirewallDisabled bool
	disableLogs         bool
	serverConfigPath    string

	dataSocketFD int
	rawUDPSockFD int
	rawTCPSockFD int
	InterfaceIP  net.IP
	TCPRWC       io.ReadWriteCloser
	UDPRWC       io.ReadWriteCloser
	logger       *slog.Logger

	// Tunnels public network only
	lc atomic.Pointer[lemonsqueezy.Client]
)

func main() {
	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "show version and exit")

	configFlag := flag.Bool("config", false, "This command runs the server and creates a config + certificates")
	configPath := flag.String("configPath", "./config.json", "path to config file (supports .json, .yaml, .yml)")
	jsonLogs := flag.Bool("json", true, "enable/disable json logging")
	sourceInfo := flag.Bool("source", false, "disable source line information in logs")
	certsOnly := flag.Bool("certs", false, "This command generates certificates and exits")
	silent := flag.Bool("silent", false, "This command disables logging")
	logLevel := flag.String("logLevel", "debug", "set the log level. Available levels: debug, info, warn, error")
	adminFlag := flag.String("admin", "", "Add an admin identifier (DeviceToken/DeviceKey/UserID) to NetAdmins")
	flag.Parse()

	serverConfigPath = *configPath
	initLogging(*silent, *jsonLogs, *sourceInfo, *logLevel)

	if showVersion {
		fmt.Println(version.Version)
		os.Exit(1)
	}

	if *configFlag {
		logger.Info("generating config")
		err := makeConfigAndCerts()
		if err != nil {
			logger.Error("unable to create certificates or config", "error", err)
			os.Exit(1)
		}
	}

	if *certsOnly {
		logger.Info("generating certs")
		err := makeCertsOnly()
		if err != nil {
			logger.Error("unable to create certificates", "error", err)
			os.Exit(1)
		}
	}

	if *adminFlag != "" {
		err := addAdminToConfig(*adminFlag)
		if err != nil {
			logger.Error("failed to add admin to config", slog.Any("err", err))
			os.Exit(1)
		}
		logger.Info("successfully added admin to NetAdmins")
		os.Exit(0)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	err := LoadServerConfig(serverConfigPath)
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
	VPNEnabled = slices.Contains(config.Features, types.VPN)
	BBOLTEnabled = slices.Contains(config.Features, types.BBOLT)
	SOCKSEnabled = slices.Contains(config.Features, types.SOCKS)

	// In development
	// DNSEnabled = slices.Contains(config.Features, types.DNS)

	if SOCKSEnabled {
		if config.SOCKSIP == "" {
			config.SOCKSIP = config.VPNIP
			Config.Store(config)
		}

		if VPNEnabled && config.SOCKSIP == config.VPNIP {
			logger.Error("SOCKS and VPN features cannot use the same IP address",
				slog.String("SOCKSIP", config.SOCKSIP),
				slog.String("VPNIP", config.VPNIP))
			os.Exit(1)
		}

		logger.Info("SOCKS5 proxy enabled",
			slog.String("ip", config.SOCKSIP),
			slog.String("port", config.SOCKSPort))
	}

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

	if SOCKSEnabled {
		go signal.NewSignal("SOCKS5", ctx, cancel, 1*time.Second, goroutineLogger, LaunchSOCKS5Server)
		StartProxyCleanupRoutine()
	}

	go signal.NewSignal("API", ctx, cancel, 1*time.Second, goroutineLogger, launchAPIServer)

	go signal.NewSignal("CONFIG", ctx, cancel, 30*time.Second, goroutineLogger, func() {
		_ = LoadServerConfig(serverConfigPath)
	})

	logger.Info("Tunnels ready")
	quit := make(chan os.Signal, 1)
	sig.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Tunnels server exiting")
}

func goroutineLogger(msg string) {
	if !disableLogs {
		logger.Debug(msg)
	}
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

	if Config.SOCKSPort == "" {
		Config.SOCKSPort = "80"
	}

	return nil
}

func LoadServerConfig(path string) (err error) {
	var nb []byte
	nb, err = os.ReadFile(path)
	if err != nil {
		return err
	}
	C := new(types.ServerConfig)

	// Determine format based on file extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(nb, &C)
	case ".json", "":
		err = json.Unmarshal(nb, &C)
	default:
		return fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml)", ext)
	}

	if err != nil {
		return err
	}
	err = validateConfig(C)
	if err != nil {
		return err
	}
	Config.Store(C)
	return err
}

func SaveServerConfig(path string) (err error) {
	C := Config.Load()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	// Determine format based on file extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		encoder := yaml.NewEncoder(f)
		encoder.SetIndent(2)
		if err := encoder.Encode(C); err != nil {
			return err
		}
		_ = encoder.Close()
	case ".json", "":
		encoder := json.NewEncoder(f)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(C); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml)", ext)
	}

	return nil
}

func addAdminToConfig(identifier string) error {
	err := LoadServerConfig(serverConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	hashedIdentifier := hashIdentifier(identifier)

	C := Config.Load()
	C.NetAdmins = append(C.NetAdmins, hashedIdentifier)
	Config.Store(C)

	err = SaveServerConfig(serverConfigPath)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func loadKeyPair(key, cert string) (c tls.Certificate, err error) {
	_, priv, err := crypt.LoadPrivateKey(key)
	if err != nil {
		return c, err
	}
	_, pub, err := crypt.LoadPublicKey(cert)
	if err != nil {
		return c, err
	}
	c, err = tls.X509KeyPair(pub, priv)
	if err != nil {
		return c, err
	}

	return c, nil
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

	apiCerts := []tls.Certificate{}
	keyPems := loadStringSliceKey("KeyPems")
	CertPems := loadStringSliceKey("CertPems")
	for i := range keyPems {
		fmt.Println(keyPems[i], CertPems[i])
		tlsc, err := loadKeyPair(keyPems[i], CertPems[i])
		if err != nil {
			return err
		}
		apiCerts = append(apiCerts, tlsc)
	}

	apiCerts = append(apiCerts, *KeyPair.Load())

	APITLSConfig.Store(&tls.Config{
		MinVersion:       tls.VersionTLS13,
		MaxVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768},
		Certificates:     apiCerts,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
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
		return err
	}
	Config := Config.Load()

	if Config.Lan != nil {
		lanFirewallDisabled = Config.DisableLanFirewall
	}
	return err
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

func makeConfigAndCerts() (err error) {
	ep, err := os.Executable()
	if err != nil {
		return err
	}
	eps := strings.Split(ep, "/")
	ep = strings.Join(eps[:len(eps)-1], "/")
	ep += "/"

	IFIP, err := gateway.DiscoverInterface()
	if err != nil {
		return err
	}
	interfaceIP := IFIP.String()

	err = LoadServerConfig(serverConfigPath)
	if err != nil {
		newConfig := &types.ServerConfig{
			Features: []types.Feature{
				types.LAN,
				types.VPN,
				types.AUTH,
				types.DNS,
				types.BBOLT,
			},
			VPNIP:     interfaceIP,
			VPNPort:   "444",
			SOCKSIP:   interfaceIP,
			SOCKSPort: "80",
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
			DBurl:               "",
			AdminAPIKey:         uuid.NewString(),
			TwoFactorKey:        strings.ReplaceAll(uuid.NewString(), "-", ""),
			CertPem:             "./cert.pem",
			KeyPem:              "./key.pem",
			SignPem:             "./sign.pem",
		}
		Config.Store(newConfig)
		if err := SaveServerConfig(serverConfigPath); err != nil {
			return err
		}
	}

	return makeCerts(ep, interfaceIP)
}

func makeCerts(execPath string, IP string) (err error) {
	_, err = certs.MakeCertV2(
		certs.ECDSA,
		filepath.Join(execPath, "cert.pem"),
		filepath.Join(execPath, "key.pem"),
		[]string{IP},
		[]string{""},
		"",
		time.Time{},
		true,
	)
	return err
}

func makeCertsOnly() (err error) {
	ep, err := os.Executable()
	if err != nil {
		return err
	}
	eps := strings.Split(ep, "/")
	ep = strings.Join(eps[:len(eps)-1], "/")
	ep += "/"

	IFIP, err := gateway.DiscoverInterface()
	if err != nil {
		return err
	}
	interfaceIP := IFIP.String()
	return makeCerts(ep, interfaceIP)
}

func initializeNewServer() error {
	user, err := DB_findUserByEmail("admin")
	if err != nil {
		return err
	}
	if user != nil {
		return nil
	}
	pw := GENERATE_CODE()

	hash, err := bcrypt.GenerateFromPassword([]byte(pw), 13)
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

	logger.Info("ADMIN PASSWORD (change this!!)", "pass", pw)

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
