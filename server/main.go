package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	sig "os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NdoleStudio/lemonsqueezy-go"
	"github.com/google/uuid"
	"github.com/jackpal/gateway"
	"github.com/tunnels-is/tunnels/signal"
	"github.com/tunnels-is/tunnels/types"
	"github.com/tunnels-is/tunnels/version"
	wgserver "github.com/tunnels-is/tunnels/wg-server"
	"golang.org/x/crypto/bcrypt"
)

var (
	CTX          atomic.Pointer[context.Context]
	Cancel       atomic.Pointer[context.CancelFunc]
	Config       atomic.Pointer[types.ServerConfig]
	WGConfig     atomic.Pointer[types.WGBootstrap]
	APITLSConfig atomic.Pointer[tls.Config]
	KeyPair      atomic.Pointer[tls.Certificate]

	disableLogs      bool
	serverConfigPath string
	wgConfigPath     string

	logger *slog.Logger

	lc atomic.Pointer[lemonsqueezy.Client]
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	showVersion := false
	flag.BoolVar(&showVersion, "version", false, "show version and exit")

	allTheThings := flag.Bool("allinone", false, "full setup of an all-in-one vpn server + auth controller. This will create configs, generate certs and create a wrieguard server + admin user in the database. Essentially a (configure everything and run) flag")
	wgServerEnabled := flag.Bool("wg", false, "enable/disable the wireguard vpn server module")
	authServerEnabled := flag.Bool("auth", false, "enable/disable the auth server module")
	createConfig := flag.String("createConfig", "", "Generate a config. '' or 'all' creates both config.json and wg-config.json; 'auth' creates config.json only; 'wg' creates wg-config.json only")
	configPath := flag.String("configPath", "./config.json", "path to controller config file (supports .json, .yaml, .yml)")
	wgConfigPathFlag := flag.String("wgConfigPath", "./wg-config.json", "path to wg-server config file")
	jsonLogs := flag.Bool("json", false, "enable/disable json logging")
	sourceInfo := flag.Bool("source", false, "disable source line information in logs")
	createCert := flag.String("createCert", "", "Generate API certificates. Use 'selfsign' for a self-signed cert or a domain name (e.g. 'example.com') to obtain a Let's Encrypt certificate via ACME HTTP-01")
	silent := flag.Bool("silent", true, "This command disables logging")
	logLevel := flag.String("logLevel", "debug", "set the log level. Available levels: debug, info, warn, error")
	createAdmin := flag.Bool("createAdmin", false, "Create the default admin user in the auth DB on startup")
	createServer := flag.Bool("createServer", false, "Create the default 'tunnels' server (with WG bootstrap) in the auth DB on startup")
	ipOverride := flag.String("ip", "", "Override the IP used for -createConfig and -createCert (defaults to auto-discovered default-route interface IP)")
	showNewRules := flag.Bool("showNewRules", false, "After wg-server fetches config from the controller, print the iptables rules it would install and hard-exit. No rules are applied.")
	showActiveRules := flag.Bool("showActiveRules", false, "Print currently-installed iptables rules matching a config-agnostic wg-server shape, then exit. Does not fetch config or touch the network.")
	disablePublicReg := flag.Bool("disablePublicRegistration", false, "reject anonymous POST /client/user/create; admin UI user create still works")
	flag.Parse()

	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicitFlags[f.Name] = true })

	serverConfigPath = *configPath
	wgConfigPath = *wgConfigPathFlag
	disablePublicRegistrationCLI = *disablePublicReg
	initLogging(*silent, *jsonLogs, *sourceInfo, *logLevel)

	if showVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}

	if *showActiveRules {
		if err := wgserver.ShowActiveRules(); err != nil {
			fmt.Fprintln(os.Stderr, "showActiveRules failed:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	configRequested := explicitFlags["createConfig"]
	configMode := strings.ToLower(strings.TrimSpace(*createConfig))
	if *allTheThings && configMode == "" {
		configMode = "all"
	}
	skipWGVerify := wgBootstrapSkipVerify(*createCert)
	if configRequested || *allTheThings {
		runCreateConfig(*ipOverride, configMode, skipWGVerify, *createConfig)
	}

	if *createAdmin || *allTheThings {
		runCreateAdmin()
	}

	if *createServer || *allTheThings {
		runCreateServer()
	}

	if *createCert != "" || *allTheThings {
		runCreateCert(*createCert, *ipOverride)
	}

	ctx, cancel := context.WithCancel(context.Background())
	CTX.Store(&ctx)
	Cancel.Store(&cancel)

	if *authServerEnabled || *allTheThings {
		startAuthServer(ctx)
	}

	var wgDone chan struct{}
	if *wgServerEnabled || *allTheThings {
		wgDone = startWGServer(ctx, *logLevel, *showNewRules)
	}

	waitForShutdown(cancel, wgDone)
}

func runCreateConfig(ipOverride, configMode string, skipWGVerify bool, rawCreateConfig string) {
	switch configMode {
	case "all", "auth", "wg":
		logger.Info("generating config", "mode", configMode)
		if err := makeConfig(ipOverride, configMode, skipWGVerify); err != nil {
			logger.Error("unable to create config", "error", err)
			os.Exit(1)
		}
	case "":
	default:
		logger.Error("invalid -createConfig value (allowed: '', 'all', 'auth', 'wg')", "value", rawCreateConfig)
		os.Exit(1)
	}
}

func runCreateAdmin() {
	err := openDB("tunnels.db")
	if err != nil {
		logger.Error("unable to connect to bbolt", slog.Any("err", err))
		os.Exit(1)
	}
	if err := initializeAdminUser(); err != nil {
		logger.Error("unable to create admin user", slog.Any("err", err))
		os.Exit(1)
	}
	db.Close()
}

func runCreateServer() {
	err := openDB("tunnels.db")
	if err != nil {
		logger.Error("unable to connect to bbolt", slog.Any("err", err))
		os.Exit(1)
	}
	if err := initializeDefaultServer(); err != nil {
		logger.Error("unable to create default server", slog.Any("err", err))
		os.Exit(1)
	}
	db.Close()
}

func runCreateCert(createCert, ipOverride string) {
	certValue := strings.TrimSpace(createCert)
	if certValue == "selfsign" {
		logger.Info("generating self-signed certificates")
		if err := generateSelfSignedCerts(ipOverride); err != nil {
			logger.Error("unable to create self-signed certificates", "error", err)
			os.Exit(1)
		}
	} else {
		logger.Info("requesting Let's Encrypt certificate", "domain", certValue)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		err := generateLetsEncryptCerts(ctx, certValue)
		cancel()
		if err != nil {
			logger.Error("unable to obtain Let's Encrypt certificate", "error", err)
			os.Exit(1)
		}
	}
}

func startAuthServer(ctx context.Context) {
	err := openDB("tunnels.db")
	if err != nil {
		logger.Error("unable to connect to bbolt", slog.Any("err", err))
		os.Exit(1)
	}

	err = LoadServerConfig(serverConfigPath)
	if err != nil {
		panic(err)
	}

	if err := validateServerConfig(Config.Load()); err != nil {
		logger.Error("invalid server config, refusing to start", slog.Any("err", err))
		os.Exit(1)
	}

	err = loadCertificatesAndTLSSettings()
	if err != nil {
		panic(err)
	}

	if loadSecret("PayKey") != "" {
		lemonClient := lemonsqueezy.New(lemonsqueezy.WithAPIKey(loadSecret("PayKey")))
		if lemonClient == nil {
			logger.Error("Unable to initialize lemon queezy client", slog.Any("err", err))
			os.Exit(1)
		}
		lc.Store(lemonClient)
		go signal.NewSignal("SUBSCANNER", ctx, 12*time.Hour, goroutineLogger, scanSubs)
	}

	go signal.NewSignal("API", ctx, 1*time.Second, goroutineLogger, launchAPIServer)

	go signal.NewSignal("PWRESET-CLEAN", ctx, passwordResetCleanEvery, goroutineLogger, cleanPasswordResetAttempts)

	go signal.NewSignal("CONFIG", ctx, 30*time.Second, goroutineLogger, func() {
		C, err := parseServerConfig(serverConfigPath)
		if err != nil {
			logger.Error("config could not be loaded", "path", serverConfigPath, slog.Any("err", err))
			return
		}
		if err := validateServerConfig(C); err != nil {
			logger.Error("reloaded config failed validation; keeping previous config", slog.Any("err", err))
			return
		}
		Config.Store(C)
	})
}

func startWGServer(ctx context.Context, logLevel string, showNewRules bool) chan struct{} {
	if err := LoadWGConfig(wgConfigPath); err != nil {
		logger.Error("WG feature enabled but wg config could not be loaded", "path", wgConfigPath, slog.Any("err", err))
		os.Exit(1)
	}
	wgCfg := WGConfig.Load()
	if wgCfg.APIKey == "" {
		logger.Error("WG feature enabled but wg config has no APIKey", "path", wgConfigPath)
		os.Exit(1)
	}
	ctrlURL := wgCfg.ControllerURL
	if ctrlURL == "" {
		latestCfg := Config.Load()
		ctrlURL = "https://" + latestCfg.APIIP + ":" + latestCfg.APIPort
	}

	wgDone := make(chan struct{})
	go wgserver.Init(ctx, ctrlURL, wgCfg.APIKey, wgConfigPath, wgCfg.InsecureSkipVerify, logLevel, showNewRules, wgDone)

	go signal.NewSignal("WG-CONFIG", ctx, 30*time.Second, goroutineLogger, func() {
		if err := LoadWGConfig(wgConfigPath); err != nil {
			logger.Error("WG feature enabled but wg config could not be loaded", "path", wgConfigPath, slog.Any("err", err))
		}
	})
	return wgDone
}

func waitForShutdown(cancel context.CancelFunc, wgDone chan struct{}) {
	logger.Info("Tunnels ready")
	quit := make(chan os.Signal, 1)
	sig.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Tunnels server exiting")

	cancel()
	if wgDone != nil {
		select {
		case <-wgDone:
			logger.Info("wg-server clean shutdown")
		case <-time.After(30 * time.Second):
			logger.Warn("wg-server shutdown timed out; iptables rules may remain")
		}
	}
}

func goroutineLogger(msg string) {
	if !disableLogs {
		logger.Debug(msg)
	}
}

func initializeAdminUser() error {
	user, err := findUserByEmail("admin")
	if err != nil {
		return err
	}
	if user != nil {
		if !user.IsAdmin {
			return fmt.Errorf("user %q exists but is not an admin; delete or promote it before -createAdmin", user.Email)
		}
		return nil
	}
	pw := generateCode()

	hash, err := bcrypt.GenerateFromPassword([]byte(pw), 13)
	if err != nil {
		return err
	}

	newUser := new(User)
	newUser.ID = uuid.New()
	newUser.Password = string(hash)
	newUser.IsAdmin = true
	newUser.Email = "admin"
	newUser.Updated = time.Now()
	newUser.Trial = false
	newUser.APIKey = uuid.NewString()
	newUser.SubExpiration = time.Now().AddDate(100, 0, 0)
	newUser.Groups = make([]uuid.UUID, 0)
	newUser.Tokens = make([]*DeviceToken, 0)
	if err := createUser(newUser); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "ADMIN PASSWORD (change this!!): pass=%s\n", pw)
	return nil
}

const defaultWGSubnet = "10.0.0.0/22"

func initializeDefaultServer() error {
	cfg := Config.Load()

	servers, err := findAllServers(math.MaxInt64, 0)
	if err != nil {
		return fmt.Errorf("find servers: %w", err)
	}
	for _, s := range servers {
		if s.Tag == "tunnels" {
			return nil
		}
	}

	if err := LoadWGConfig(wgConfigPath); err != nil {
		return fmt.Errorf("load wg config %q: %w", wgConfigPath, err)
	}
	wgCfg := WGConfig.Load()

	apiKey := uuid.NewString()
	internetIface := discoverInternetIface()
	if !types.ValidIfaceName(internetIface) {
		return fmt.Errorf("could not discover a valid InternetIface (got %q)", internetIface)
	}

	server := &types.Server{
		ID:                 uuid.New(),
		Tag:                "tunnels",
		Country:            "tunnels",
		IP:                 cfg.APIIP,
		Port:               cfg.APIPort,
		Groups:             []uuid.UUID{},
		APIKey:             apiKey,
		WireGuardPort:      51820,
		WireGuardIface:     "wg0",
		WireGuardSubnet:    defaultWGSubnet,
		InternetIface:      internetIface,
		InsecureSkipVerify: wgCfg.InsecureSkipVerify,
	}
	if err := createServer(server); err != nil {
		return fmt.Errorf("create default server: %w", err)
	}

	wgCfg.APIKey = apiKey
	WGConfig.Store(wgCfg)
	if err := SaveWGConfig(wgConfigPath); err != nil {
		return fmt.Errorf("save wg config: %w", err)
	}

	logger.Info("default server initialized",
		"subnet", server.WireGuardSubnet,
		"port", server.WireGuardPort,
		"iface", server.WireGuardIface,
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
