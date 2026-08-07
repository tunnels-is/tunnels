package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	sig "os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/NdoleStudio/lemonsqueezy-go"
	"github.com/google/uuid"
	"github.com/jackpal/gateway"
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/signal"
	"github.com/tunnels-is/tunnels/types"
	"github.com/tunnels-is/tunnels/version"
	wgserver "github.com/tunnels-is/tunnels/wg-server"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
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
	flag.Parse()

	explicitFlags := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicitFlags[f.Name] = true })

	serverConfigPath = *configPath
	wgConfigPath = *wgConfigPathFlag
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
	if configRequested || *allTheThings {
		switch configMode {
		case "all", "auth", "wg":
			logger.Info("generating config", "mode", configMode)
			if err := makeConfig(*ipOverride, configMode); err != nil {
				logger.Error("unable to create config", "error", err)
				os.Exit(1)
			}
		case "":
		default:
			logger.Error("invalid -createConfig value (allowed: '', 'all', 'auth', 'wg')", "value", *createConfig)
			os.Exit(1)
		}
	}

	if *createAdmin || *allTheThings {
		err := ConnectToBBoltDB("tunnels.db")
		if err != nil {
			logger.Error("unable to connect to bbolt", slog.Any("err", err))
			os.Exit(1)
		}
		if err := initializeAdminUser(); err != nil {
			logger.Error("unable to create admin user", slog.Any("err", err))
			os.Exit(1)
		}
		BBoltDB.Close()
	}

	if *createServer || *allTheThings {
		err := ConnectToBBoltDB("tunnels.db")
		if err != nil {
			logger.Error("unable to connect to bbolt", slog.Any("err", err))
			os.Exit(1)
		}
		if err := initializeDefaultServer(); err != nil {
			logger.Error("unable to create default server", slog.Any("err", err))
			os.Exit(1)
		}
		BBoltDB.Close()
	}

	if *createCert != "" || *allTheThings {
		certValue := strings.TrimSpace(*createCert)
		if certValue == "selfsign" {
			logger.Info("generating self-signed certificates")
			if err := generateSelfSignedCerts(*ipOverride); err != nil {
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

	ctx, cancel := context.WithCancel(context.Background())
	CTX.Store(&ctx)
	Cancel.Store(&cancel)

	if *authServerEnabled || *allTheThings {
		err := ConnectToBBoltDB("tunnels.db")
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

	var wgDone chan struct{}
	if *wgServerEnabled || *allTheThings {
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

		wgDone = make(chan struct{})
		go wgserver.Init(ctx, ctrlURL, wgCfg.APIKey, wgConfigPath, wgCfg.InsecureSkipVerify, *logLevel, *showNewRules, wgDone)

		go signal.NewSignal("WG-CONFIG", ctx, 30*time.Second, goroutineLogger, func() {
			if err := LoadWGConfig(wgConfigPath); err != nil {
				logger.Error("WG feature enabled but wg config could not be loaded", "path", wgConfigPath, slog.Any("err", err))
			}
		})
	}

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

const minSecretLen = 32

func validateServerConfig(c *types.ServerConfig) error {
	if c == nil {
		return errors.New("server config is nil")
	}
	if len(c.CookieSigningKey) < minSecretLen {
		return fmt.Errorf("CookieSigningKey must be set and at least %d characters: it seeds the AES-256 key that seals admin session cookies; generate a config with -createConfig or set a strong random value", minSecretLen)
	}
	if len(c.TwoFactorKey) < minSecretLen {
		return fmt.Errorf("TwoFactorKey must be set and at least %d characters: it seeds the AES-256 key that encrypts 2FA and recovery codes at rest; generate a config with -createConfig or set a strong random value", minSecretLen)
	}
	return nil
}

func parseServerConfig(path string) (*types.ServerConfig, error) {
	nb, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	C := new(types.ServerConfig)

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(nb, &C)
	case ".json", "":
		err = json.Unmarshal(nb, &C)
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml)", ext)
	}
	if err != nil {
		return nil, err
	}
	return C, nil
}

func LoadServerConfig(path string) (err error) {
	C, err := parseServerConfig(path)
	if err != nil {
		return err
	}
	Config.Store(C)
	return nil
}

func SaveServerConfig(path string) (err error) {
	C := Config.Load()

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
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

func LoadWGConfig(path string) error {
	nb, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	W := new(types.WGBootstrap)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(nb, W)
	case ".json", "":
		err = json.Unmarshal(nb, W)
	default:
		return fmt.Errorf("unsupported wg config file format: %s (supported: .json, .yaml, .yml)", ext)
	}
	if err != nil {
		return err
	}
	WGConfig.Store(W)
	return nil
}

func SaveWGConfig(path string) error {
	W := WGConfig.Load()
	if W == nil {
		return fmt.Errorf("no wg config loaded")
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		encoder := yaml.NewEncoder(f)
		encoder.SetIndent(2)
		if err := encoder.Encode(W); err != nil {
			return err
		}
		_ = encoder.Close()
	case ".json", "":
		encoder := json.NewEncoder(f)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(W); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported wg config file format: %s (supported: .json, .yaml, .yml)", ext)
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
	if len(keyPems) != len(CertPems) {
		return fmt.Errorf("config KeyPems (%d) and CertPems (%d) must have the same length", len(keyPems), len(CertPems))
	}
	for i := range keyPems {
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

func makeConfig(ipOverride string, mode string) error {
	writeServer := mode == "all" || mode == "auth"
	writeWG := mode == "all" || mode == "wg"

	if writeServer {
		if err := writeServerConfig(ipOverride, mode); err != nil {
			return err
		}
	}
	if writeWG {
		if err := writeWGConfig(); err != nil {
			return err
		}
	}
	return nil
}

func writeServerConfig(ipOverride, mode string) error {
	if err := LoadServerConfig(serverConfigPath); err == nil {
		return nil
	}

	interfaceIP, err := resolveInterfaceIP(ipOverride)
	if err != nil {
		return err
	}

	newConfig := &types.ServerConfig{
		APIIP:            interfaceIP,
		APIPort:          "443",
		DBurl:            "",
		AdminAPIKey:      uuid.NewString(),
		TwoFactorKey:     strings.ReplaceAll(uuid.NewString(), "-", ""),
		CookieSigningKey: strings.ReplaceAll(uuid.NewString(), "-", ""),
		CertPem:          "./cert.pem",
		KeyPem:           "./key.pem",
	}
	Config.Store(newConfig)
	return SaveServerConfig(serverConfigPath)
}

func writeWGConfig() error {
	if err := LoadWGConfig(wgConfigPath); err == nil {
		return nil
	}

	WGConfig.Store(&types.WGBootstrap{InsecureSkipVerify: false})
	return SaveWGConfig(wgConfigPath)
}

func initializeAdminUser() error {
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
	if err := DB_CreateUser(newUser); err != nil {
		return err
	}

	logger.Info("ADMIN PASSWORD (change this!!)", "pass", pw)
	return nil
}

const defaultWGSubnet = "10.0.0.0/22"

func initializeDefaultServer() error {
	cfg := Config.Load()

	servers, err := DB_FindAllServers(math.MaxInt64, 0)
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
	if err := DB_CreateServer(server); err != nil {
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
