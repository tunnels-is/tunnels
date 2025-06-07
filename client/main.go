package client

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func InitService() error {
	defer RecoverAndLogToFile()

	InitBaseFoldersAndPaths()
	_ = loadConfigFromDisk()
	loadTunnelsFromDisk()
	loadDefaultGateway()
	loadDefaultInterface()
	InitDNSHandler()
	INFO("Starting Tunnels")

	conf := CONFIG.Load()
	s := STATE.Load()

	if !conf.ConsoleLogOnly {
		var err error
		LogFile, err = CreateFile(s.LogFileName)
		if err != nil {
			return err
		}
	}

	INFO("Operating specific initializations")
	_ = OSSpecificInit()

	INFO("Checking permissins")
	AdminCheck()

	printInfo()
	printInfo2()

	doEvent(highPriorityChannel, func() {
		reloadBlockLists(false, true)
	})

	INFO("Tunnels is ready")
	return nil
}

func printInfo() {
	log.Println("")
	log.Println("")
	log.Println("==============================================================")
	log.Println("======================= TUNNELS.IS ===========================")
	log.Println("==============================================================")
	log.Println("NOTE: If the app closes without any logs/errors you might need to delete your config and try again")
	log.Println("")
}

func printInfo2() {
	conf := CONFIG.Load()
	s := STATE.Load()
	log.Println("")
	log.Println("=======================================================================")
	log.Println("======================= HELPFUL INFORMATION ===========================")
	log.Println("=======================================================================")
	log.Println("")
	log.Printf("APP: https://%s:%s\n", conf.APIIP, conf.APIPort)
	log.Println("")
	log.Println("BASE PATH:", s.BasePath)
	log.Println("")
	log.Println("- Tunnels request network admin permissions to run.")
	log.Println("- Remember to configure your DNS servers if you want to prevent DNS leaks.")
	log.Println("- Remember to turn all logging off if you are concerned about privacy.")
	log.Println("- There is a --basePath flag that can let you reconfigure the base directory for logs and configs, the default location is where you placed tunnels.")
	log.Println("")
	log.Println("=======================================================================")
	log.Println("=======================================================================")
}

func LaunchTunnels() {
	defer RecoverAndLogToFile()

	CancelContext, CancelFunc = context.WithCancel(GlobalContext)
	quit = make(chan os.Signal, 10)

	signal.Notify(
		quit,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGILL,
	)

	newConcurrentSignal("LogProcessor", CancelContext, func() {
		StartLogQueueProcessor()
	})

	newConcurrentSignal("APIServer", CancelContext, func() {
		LaunchAPI()
	})

	newConcurrentSignal("UDPDNSHandler", CancelContext, func() {
		StartUDPDNSHandler()
	})

	config := CONFIG.Load()
	if config.OpenUI {
		newConcurrentSignal("OpenUI", CancelContext, func() {
			popUI()
		})
	}

	newConcurrentSignal("Pinger", CancelContext, func() {
		PingConnections()
	})

	newConcurrentSignal("BlockListUpdater", CancelContext, func() {
		reloadBlockLists(true, true)
	})

	newConcurrentSignal("LogMapCleaner", CancelContext, func() {
		CleanUniqueLogMap()
	})

	newConcurrentSignal("CleanPortAllocs", CancelContext, func() {
		CleanPortsForAllConnections()
	})

	newConcurrentSignal("DefaultGateway", CancelContext, func() {
		GetDefaultGateway()
	})

	newConcurrentSignal("CleanDNSCache", CancelContext, func() {
		CleanDNSCache()
	})

	newConcurrentSignal("AutoConnect", CancelContext, func() {
		AutoConnect()
	})

mainLoop:
	for {

		select {
		case high := <-highPriorityChannel:
			go high.method()
			continue mainLoop
		case med := <-mediumPriorityChannel:
			go med.method()
			continue mainLoop
		case low := <-lowPriorityChannel:
			go low.method()
			continue mainLoop
		default:
		}

		select {
		case sig := <-quit:
			DEBUG("", "exit signal caught: ", sig.String())
			CancelFunc()
			CleanupOnClose()
			os.Exit(1)

		case Tun := <-interfaceMonitor:
			go Tun.ReadFromTunnelInterface()
		case Tun := <-tunnelMonitor:
			go Tun.ReadFromServeTunnel()

		case signal := <-concurrencyMonitor:
			DEBUG(signal.tag)
			go signal.execute()

		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func InitMinimalService() error {
	defer RecoverAndLogToFile()
	InitBaseFoldersAndPaths()
	_ = loadConfigFromDisk()
	loadTunnelsFromDisk()
	loadDefaultGateway()
	loadDefaultInterface()

	conf := CONFIG.Load()
	conf.OpenUI = false
	conf.ConsoleLogOnly = true
	CONFIG.Store(conf)

	INFO("Operating specific initializations")
	_ = OSSpecificInit()

	INFO("Checking permissins")
	AdminCheck()

	cli := CLIConfig.Load()
	if cli.DNS {
		InitDNSHandler()
		INFO("Starting Tunnels")
		doEvent(highPriorityChannel, func() {
			reloadBlockLists(false, true)
		})
	}

	// err := getDeviceAndServer()
	// if err != nil {
	// 	return err
	// }
	//
	// doEvent(highPriorityChannel, func() {
	// 	code, _ := PublicConnect(&ConnectionRequest{
	// 		URL:       cli.AuthServer,
	// 		Secure:    cli.Secure,
	// 		DeviceKey: cli.DeviceID,
	// 		Tag:       DefaultTunnelName,
	// 		ServerID:  cli.ServerID,
	// 	})
	// 	if code != 200 {
	// 		time.Sleep(5 * time.Second)
	// 	}
	// })

	return nil
}

func LaunchMinimalTunnels() {
	defer RecoverAndLogToFile()
	cli := CLIConfig.Load()

	CancelContext, CancelFunc = context.WithCancel(GlobalContext)
	quit = make(chan os.Signal, 10)

	signal.Notify(
		quit,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGILL,
	)

	newConcurrentSignal("LogProcessor", CancelContext, func() {
		StartLogQueueProcessor()
	})

	if cli.DNS {
		newConcurrentSignal("UDPDNSHandler", CancelContext, func() {
			StartUDPDNSHandler()
		})
		newConcurrentSignal("BlockListUpdater", CancelContext, func() {
			reloadBlockLists(true, true)
		})
		newConcurrentSignal("CleanDNSCache", CancelContext, func() {
			CleanDNSCache()
		})
	}

	newConcurrentSignal("Pinger", CancelContext, func() {
		PingConnections()
	})
	newConcurrentSignal("AutoConnect", CancelContext, func() {
		AutoConnect()
	})

	newConcurrentSignal("LogMapCleaner", CancelContext, func() {
		CleanUniqueLogMap()
	})

	newConcurrentSignal("CleanPortAllocs", CancelContext, func() {
		CleanPortsForAllConnections()
	})

	newConcurrentSignal("DefaultGateway", CancelContext, func() {
		GetDefaultGateway()
	})

mainLoop:
	for {

		select {
		case high := <-highPriorityChannel:
			go high.method()
			continue mainLoop
		case med := <-mediumPriorityChannel:
			go med.method()
			continue mainLoop
		case low := <-lowPriorityChannel:
			go low.method()
			continue mainLoop
		default:
		}

		select {
		case sig := <-quit:
			DEBUG("", "exit signal caught: ", sig.String())
			CancelFunc()
			CleanupOnClose()
			os.Exit(1)

		case IF := <-interfaceMonitor:
			go IF.ReadFromTunnelInterface()
		case Tun := <-tunnelMonitor:
			go Tun.ReadFromServeTunnel()

		case signal := <-concurrencyMonitor:
			DEBUG(signal.tag)
			go signal.execute()

		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func writeConfigToDisk() (err error) {
	defer RecoverAndLogToFile()
	conf := CONFIG.Load()
	s := STATE.Load()

	cb, err := json.Marshal(conf)
	if err != nil {
		ERROR("Unable to marshal config into bytes: ", err)
		return err
	}

	err = RenameFile(s.ConfigFileName, s.ConfigFileName+".bak")
	if err != nil {
		ERROR("Unable to rename config file: ", err)
	}

	f, err := CreateFile(s.ConfigFileName)
	if err != nil {
		ERROR("Unable to create new config", err)
		return err
	}
	defer f.Close()

	_, err = f.Write(cb)
	if err != nil {
		ERROR("Unable to write config bytes to new config file: ", err)
		return err
	}

	return
}

func LoadPrivateCertFromBytes(data []byte) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(data)
	return certPool, nil
}

func LoadPrivateCert(path string) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	certData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	certPool.AppendCertsFromPEM(certData)
	return certPool, nil
}

func (m *TUN) LoadCertPEMBytes(cert []byte) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(cert)
	if !ok {
		return certPool, fmt.Errorf("unable to append cert")
	}
	return certPool, nil
}

func ReadConfigFileFromDisk() (err error) {
	state := STATE.Load()
	config, err := os.ReadFile(state.ConfigFileName)
	if err != nil {
		return err
	}

	Conf := new(configV2)
	err = json.Unmarshal(config, Conf)
	if err != nil {
		ERROR("Unable to turn config file into config object: ", err)
		return
	}

	CONFIG.Store(Conf)

	return
}

func writeTunnelsToDisk(tag string) (outErr error) {
	s := STATE.Load()
	TunnelMetaMap.Range(func(key, value any) bool {
		t, ok := value.(*TunnelMETA)
		if !ok {
			ERROR("Unable to save tunnel to disk: unable to cast any to meta")
			outErr = errors.New("unable to save tunnel to disk")
			return false
		}
		if tag != "" {
			if t.Tag != tag {
				return true
			}
		}
		tb, err := json.Marshal(value)
		if err != nil {
			ERROR("Unable to transform tunnel to json:", err)
			outErr = err
			return false
		}

		err = RenameFile(s.TunnelsPath+t.Tag+tunnelFileSuffix, s.TunnelsPath+t.Tag+tunnelFileSuffix+".bak")
		if err != nil {
			ERROR("Unable to rename tunnel file:", err)
		}

		tf, err := CreateFile(s.TunnelsPath + t.Tag + tunnelFileSuffix)
		if err != nil {
			ERROR("Unable to save tunnel to disk:", err)
			outErr = err
			return false
		}

		_, err = tf.Write(tb)
		if err != nil {
			ERROR("Unable to write tunnel json to file:", err)
			outErr = err
			return false
		}

		return true
	})

	return
}

func loadTunnelsFromDisk() {
	s := STATE.Load()
	foundDefault := false
	err := filepath.WalkDir(s.TunnelsPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		if !strings.HasSuffix(path, tunnelFileSuffix) {
			return nil
		}

		tb, ferr := os.ReadFile(path)
		if ferr != nil {
			ERROR("Unable to read tunnel file:", err)
			return err
		}

		tunnel := new(TunnelMETA)
		merr := json.Unmarshal(tb, tunnel)
		if merr != nil {
			ERROR("Unable to marshal tunnel file:", err)
			return err
		}
		TunnelMetaMap.Store(tunnel.Tag, tunnel)
		DEBUG("Loaded tunnel:", tunnel.Tag)
		if tunnel.Tag == DefaultTunnelName {
			foundDefault = true
		}

		return nil
	})
	if err != nil {
		ERROR("Unable to walk tunnel path:", err)
	}

	if !foundDefault {
		cli := CLIConfig.Load()
		newTun := createDefaultTunnelMeta(cli.Enabled)
		TunnelMetaMap.Store(newTun.Tag, newTun)
		_ = writeTunnelsToDisk(newTun.Tag)
	}
}

func DefaultMinimalConfig(withDNS bool) *configV2 {
	conf := &configV2{
		DebugLogging:     true,
		InfoLogging:      true,
		ErrorLogging:     true,
		ConnectionTracer: false,
		ConsoleLogging:   true,
		ConsoleLogOnly:   true,
	}
	if withDNS {
		conf.DNSServerIP = "0.0.0.0"
		conf.DNSServerPort = "53"
		conf.DNS1Default = "1.1.1.1"
		conf.DNS2Default = "8.8.8.8"
		conf.LogBlockedDomains = true
		conf.LogAllDomains = true
		conf.DNSstats = true
		conf.DNSBlockLists = GetDefaultBlockLists()
	}
	return conf
}

// DefaultConfig returns a new configV2 with default values
func DefaultConfig() *configV2 {
	conf := &configV2{
		DebugLogging:      true,
		InfoLogging:       true,
		ErrorLogging:      true,
		ConnectionTracer:  false,
		DNSServerIP:       "127.0.0.1",
		DNSServerPort:     "53",
		DNS1Default:       "1.1.1.1",
		DNS2Default:       "8.8.8.8",
		LogBlockedDomains: true,
		LogAllDomains:     true,
		DNSstats:          true,
		DNSBlockLists:     GetDefaultBlockLists(),
		APIIP:             "127.0.0.1",
		APIPort:           "7777",
		AuthServers:       []string{"https://api.tunnels.is", "https://127.0.0.1"},
	}
	applyCertificateDefaultsToConfig(conf)
	return conf
}

func loadConfigFromDisk() error {
	defer RecoverAndLogToFile()
	DEBUG("Loading configurations from file")

	if err := ReadConfigFileFromDisk(); err == nil {
		return nil
	}

	cli := CLIConfig.Load()
	if cli.Enabled {
		CONFIG.Store(DefaultMinimalConfig(cli.DNS))
	} else {
		CONFIG.Store(DefaultConfig())
	}
	DEBUG("Generating a new default config")
	return writeConfigToDisk()
}

func applyCertificateDefaultsToConfig(cfg *configV2) {
	if cfg.APIKey == "" {
		cfg.APIKey = "./api.key"
	}
	if cfg.APICert == "" {
		cfg.APICert = "./api.crt"
	}

	cfg.APICertType = certs.RSA

	if len(cfg.APICertIPs) < 1 {
		cfg.APICertIPs = []string{"127.0.0.1", "0.0.0.0"}
	}

	if len(cfg.APICertDomains) < 1 {
		cfg.APICertDomains = []string{"tunnels.app", "app.tunnels.is"}
	}
}

//	func LoadDNSWhitelist() (err error) {
//		defer RecoverAndLogToFile()
//
//		if C.DomainWhitelist == "" {
//			return nil
//		}
//
//		WFile, err := os.OpenFile(C.DomainWhitelist, os.O_RDWR|os.O_CREATE, 0o777)
//		if err != nil {
//			return err
//		}
//		defer WFile.Close()
//
//		scanner := bufio.NewScanner(WFile)
//
//		WhitelistMap := make(map[string]bool)
//		for scanner.Scan() {
//			domain := scanner.Text()
//			if domain == "" {
//				continue
//			}
//			WhitelistMap[domain] = true
//		}
//
//		err = scanner.Err()
//		if err != nil {
//			ERROR("Unable to load domain whitelist: ", err)
//			return err
//		}
//
//		DNSWhitelist = WhitelistMap
//
//		return nil
//	}

func CleanupOnClose() {
	defer RecoverAndLogToFile()
	tunnelMapRange(func(tun *TUN) bool {
		tunnel := tun.tunnel.Load()
		err := tunnel.Disconnect(tun)
		if err != nil {
			ERROR("unable to disconnect tunnel", tun.ID, tunnel.IPv4Address, "error:", err)
		}
		return true
	})
	if TraceFile != nil {
		_ = TraceFile.Close()
	}
	if LogFile != nil {
		_ = LogFile.Close()
	}
}

func popUI() {
	defer RecoverAndLogToFile()
	<-uiChan
	time.Sleep(2 * time.Second)

	url := "https://" + API_SERVER.Addr
	INFO("opening UI @ ", url)

	switch runtime.GOOS {
	case "windows":
		_ = openURL(url)

	case "darwin":
		_ = openURL(url)

	default:
		if !isWSL() {
			_ = openURL(url)
		}

	}
}

func isWSL() bool {
	releaseData, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(releaseData)), "microsoft")
}

func getServerByID(secure bool, authServer string, deviceKey string, deviceToken string, UserID string, ServerID string) (s *types.Server, err error) {
	SID, _ := primitive.ObjectIDFromHex(ServerID)
	UID, _ := primitive.ObjectIDFromHex(UserID)

	FR := &FORWARD_REQUEST{
		URL:     authServer,
		Secure:  secure,
		Path:    "/v3/server",
		Method:  "POST",
		Timeout: 10000,
		JSONData: &types.FORM_GET_SERVER{
			DeviceToken: deviceToken,
			DeviceKey:   deviceKey,
			UID:         UID,
			ServerID:    SID,
		},
	}
	responseBytes, code, err := SendRequestToURL(
		nil,
		FR.Method,
		FR.URL+FR.Path,
		FR.JSONData,
		FR.Timeout,
		FR.Secure,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "error calling controller", err)
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d", "invalid code from controller", code)
	}

	s = new(types.Server)
	err = json.Unmarshal(responseBytes, s)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "invalid response from controller", err)
	}
	return
}

func GetDeviceByID(secure bool, authServer string, deviceID string) (d *types.Device, err error) {
	DID, _ := primitive.ObjectIDFromHex(deviceID)

	FR := &FORWARD_REQUEST{
		URL:     "https://" + authServer,
		Secure:  secure,
		Path:    "/v3/device",
		Method:  "POST",
		Timeout: 10000,
		JSONData: &types.FORM_GET_DEVICE{
			DeviceID: DID,
		},
	}
	responseBytes, code, err := SendRequestToURL(
		nil,
		FR.Method,
		FR.URL+FR.Path,
		FR.JSONData,
		FR.Timeout,
		FR.Secure,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "error calling controller", err)
	}
	if code != 200 {
		return nil, fmt.Errorf("%s: %d", "invalid code from controller", code)
	}

	d = new(types.Device)
	err = json.Unmarshal(responseBytes, d)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", "invalid response from controller", err)
	}
	return
}
