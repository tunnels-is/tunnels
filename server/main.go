package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	sig "os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NdoleStudio/lemonsqueezy-go"
	"github.com/google/uuid"
	"github.com/jackpal/gateway"
	"github.com/joho/godotenv"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/signal"
	"github.com/tunnels-is/tunnels/types"
	"github.com/tunnels-is/tunnels/version"
	wgserver "github.com/tunnels-is/tunnels/wg-server"
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

	AUTHEnabled  bool
	BBOLTEnabled bool

	disableLogs      bool
	serverConfigPath string

	logger *slog.Logger

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
	BBOLTEnabled = slices.Contains(config.Features, types.BBOLT)
	WGEnabled := slices.Contains(config.Features, types.WG)

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
			seedNetworks()
			err = initializeNewServer()
			if err != nil {
				logger.Error("unable to create admin user", slog.Any("err", err))
				os.Exit(1)
			}
			err = initializeWGServer()
			if err != nil {
				logger.Error("unable to initialize WG server", slog.Any("err", err))
				os.Exit(1)
			}
		}
	}

	if WGEnabled {
		latestCfg := Config.Load()
		wgCfg := latestCfg.WG
		if wgCfg == nil || wgCfg.APIKey == "" {
			logger.Error("WG feature enabled but no WG config found in config file")
			os.Exit(1)
		}
		ctrlURL := wgCfg.ControllerURL
		if ctrlURL == "" {
			ctrlURL = "https://" + latestCfg.APIIP + ":" + latestCfg.APIPort
		}
		go wgserver.Init(ctx, ctrlURL, wgCfg.APIKey, wgCfg.InsecureSkipVerify)
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
		return err
	}
	C := new(types.ServerConfig)

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
	_, privB, err := crypt.LoadPrivateKey(loadSecret("KeyPem"))
	if err != nil {
		return err
	}
	_, pubB, err := crypt.LoadPublicKey(loadSecret("CertPem"))
	if err != nil {
		return err
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
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768},
		Certificates:     apiCerts,
	})

	return nil
}

func hashIdentifier(identifier string) string {
	h, err := bcrypt.GenerateFromPassword([]byte(identifier), bcrypt.MinCost)
	if err != nil {
		return identifier
	}
	return string(h)
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
				types.AUTH,
				types.DNS,
				types.BBOLT,
				types.WG,
			},
			VPNIP:              interfaceIP,
			APIIP:              interfaceIP,
			APIPort:            "443",
			NetAdmins:          []string{},
			Hostname:           "tunnels.local",
			Routes:             []*types.Route{},
			SubNets:            []*types.Network{},
			UserMaxConnections: 10,
			DNSRecords:         []*types.DNSRecord{},
			DNSServers:         []string{},
			SecretStore:        "config",
			DBurl:              "",
			AdminAPIKey:        uuid.NewString(),
			TwoFactorKey:       strings.ReplaceAll(uuid.NewString(), "-", ""),
			CertPem:            "./cert.pem",
			KeyPem:             "./key.pem",
			SignPem:            "./sign.pem",
			WG:                 &types.WGBootstrap{InsecureSkipVerify: true},
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
	return DB_CreateServer(&types.Server{
		ID:      primitive.NewObjectID(),
		Tag:     "tunnels",
		Country: "tunnels",
		IP:      c.VPNIP,
		Port:    c.APIPort,
		Groups:  []primitive.ObjectID{},
	})
}

func initializeWGServer() error {
	cfg := Config.Load()
	if cfg.WG != nil && cfg.WG.APIKey != "" {
		return nil
	}

	internetIface := discoverInternetIface()

	network := &Network{
		ID:        primitive.NewObjectID(),
		CIDR:      "10.0.0.0/22",
		Tag:       "wg-default",
		CreatedAt: time.Now(),
	}
	if err := DB_CreateNetworksBatch([]*Network{network}); err != nil {
		return fmt.Errorf("create wg network: %w", err)
	}

	privKey := generateWGPrivKey()
	pubKey, err := deriveWGPubKey(privKey)
	if err != nil {
		return fmt.Errorf("derive wg public key: %w", err)
	}

	wgCfg := &types.WGServerConfig{
		ID:                 primitive.NewObjectID(),
		Tag:                "tunnels",
		APIKey:             uuid.NewString(),
		WireGuardPort:      51820,
		WireGuardPrivKey:   privKey,
		NetworkID:          network.ID,
		WireGuardIface:     "wg0",
		InternetIface:      internetIface,
		InsecureSkipVerify: true,
	}
	if err := DB_CreateWGServerConfig(wgCfg); err != nil {
		return fmt.Errorf("create wg server config: %w", err)
	}

	network.WGConfigID = wgCfg.ID
	if err := DB_UpdateNetwork(network); err != nil {
		return fmt.Errorf("update network: %w", err)
	}

	servers, err := DB_FindAllServers()
	if err != nil {
		return fmt.Errorf("find servers: %w", err)
	}
	var defaultServer *types.Server
	for _, s := range servers {
		if s.Tag == "tunnels" {
			defaultServer = s
			break
		}
	}
	if defaultServer == nil {
		return fmt.Errorf("default server not found")
	}
	if err := DB_SetServerWGConfigID(defaultServer.ID, wgCfg, pubKey, network.CIDR); err != nil {
		return fmt.Errorf("assign wg config to server: %w", err)
	}

	cfg.WG = &types.WGBootstrap{
		APIKey:             wgCfg.APIKey,
		InsecureSkipVerify: true,
	}
	Config.Store(cfg)
	if err := SaveServerConfig(serverConfigPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	logger.Info("WG server initialized",
		"pubKey", pubKey,
		"subnet", network.CIDR,
		"port", wgCfg.WireGuardPort,
		"iface", wgCfg.WireGuardIface,
		"internetIface", internetIface,
	)
	return nil
}

func discoverInternetIface() string {
	ip, err := gateway.DiscoverInterface()
	if err != nil {
		return ""
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ifaceIP net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ifaceIP = v.IP
			case *net.IPAddr:
				ifaceIP = v.IP
			}
			if ifaceIP != nil && ifaceIP.Equal(ip) {
				return iface.Name
			}
		}
	}
	return ""
}
