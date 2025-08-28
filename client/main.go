package client

import (
	"context"
	"crypto/x509"
	"encoding/json"
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

func InitService() error {
	defer RecoverAndLog()

	InitBaseFoldersAndPaths()
	state := STATE.Load()

	cfgError := loadConfigFromDisk(false)
	if cfgError != nil {
		if state.RequireConfig {
			return cfgError
		}
		_ = loadConfigFromDisk(true)
	}
	conf := CONFIG.Load()

	if conf.AutoDownloadUpdate {
		doStartupUpdate()
	}

	loadTunnelsFromDisk()
	loadDefaultGateway()
	loadDefaultInterface()

	if conf.CLIConfig != nil {
		DEBUG("cli config loaded")
		wasChanged := false
		if conf.OpenUI {
			conf.OpenUI = false
			wasChanged = true
		}
		if conf.ConsoleLogOnly {
			conf.ConsoleLogOnly = true
			wasChanged = true
		}
		if wasChanged {
			CONFIG.Store(conf)
		}
	}

	INFO("Starting Tunnels")

	if !conf.ConsoleLogOnly {
		var err error
		LogFile, err = CreateFile(state.LogFileName)
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

	if !conf.DisableDNS {
		InitDNSHandler()
		INFO("Starting DNS Proxy")
		doEvent(highPriorityChannel, func() {
			reloadBlockLists(false)
		})
	}

	INFO("Tunnels is ready")
	return nil
}

func LaunchTunnels() {
	defer RecoverAndLog()

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
	conf := CONFIG.Load()

	if conf.CLIConfig == nil {
		newConcurrentSignal("APIServer", CancelContext, func() {
			LaunchAPI()
		})
	}

	if !conf.DisableDNS {
		newConcurrentSignal("UDPDNSHandler", CancelContext, func() {
			StartUDPDNSHandler()
		})
		newConcurrentSignal("BlockListUpdater", CancelContext, func() {
			reloadBlockLists(true)
		})
		newConcurrentSignal("CleanDNSCache", CancelContext, func() {
			CleanDNSCache()
		})
	}

	if conf.OpenUI {
		newConcurrentSignal("OpenUI", CancelContext, func() {
			popUI()
		})
	}

	newConcurrentSignal("LogMapCleaner", CancelContext, func() {
		CleanUniqueLogMap()
	})

	newConcurrentSignal("CleanPortAllocs", CancelContext, func() {
		CleanPortsForAllConnections()
	})

	newConcurrentSignal("DefaultGateway", CancelContext, func() {
		GetDefaultGateway()
	})

	newConcurrentSignal("AutoConnect", CancelContext, func() {
		AutoConnect()
	})

	newConcurrentSignal("Pinger", CancelContext, func() {
		PingConnections()
	})

	newConcurrentSignal("Updater", CancelContext, func() {
		doUpdate()
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

func writeConfigToDisk() (err error) {
	defer RecoverAndLog()
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

	if len(Conf.ControlServers) < 1 {
		Conf.ControlServers = append(Conf.ControlServers, &ControlServer{
			ID:                  "tunnels",
			Host:                "api.tunnels.is",
			Port:                "443",
			CertificatePath:     "",
			ValidateCertificate: true,
		})
		err = writeConfigToDisk()
		if err != nil {
			ERROR("unable to ad api.tunnels.is to default config")
		}
	}

	CONFIG.Store(Conf)

	return
}

func writeTunnelsToDisk(tag string) (outErr error) {
	s := STATE.Load()
	TunnelMetaMap.Range(func(key string, value *TunnelMETA) bool {
		t := value
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

		err = RenameFile(s.TunnelsPath+t.Tag+tunnelFileSuffix, s.TunnelsPath+t.Tag+tunnelFileSuffix+backupFileSuffix)
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
		tf.Sync()
		tf.Close()

		return true
	})

	return
}

func loadTunnelsFromDisk() (err error) {
	s := STATE.Load()
	foundDefault := false
	err = filepath.WalkDir(s.TunnelsPath, func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}
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
		return err
	}

	if !foundDefault {
		newTun := createDefaultTunnelMeta()
		TunnelMetaMap.Store(newTun.Tag, newTun)
		_ = writeTunnelsToDisk(newTun.Tag)
	}
	return nil
}

// DefaultConfig returns a new configV2 with default values
func DefaultConfig() *configV2 {
	conf := &configV2{
		DebugLogging:         true,
		InfoLogging:          true,
		ErrorLogging:         true,
		ConnectionTracer:     false,
		DNSServerIP:          "127.0.0.1",
		DNSServerPort:        "53",
		DNS1Default:          "1.1.1.1",
		DNS2Default:          "8.8.8.8",
		LogBlockedDomains:    true,
		LogAllDomains:        true,
		DNSstats:             true,
		DNSBlockLists:        GetDefaultBlockLists(),
		APIIP:                "127.0.0.1",
		APIPort:              "7777",
		RestartPostUpdate:    true,
		ExitPostUpdate:       false,
		AutoDownloadUpdate:   true,
		UpdateWhileConnected: false,
	}
	conf.ControlServers = append(conf.ControlServers, &ControlServer{
		ID:                  "tunnels",
		Host:                "api.tunnels.is",
		Port:                "443",
		CertificatePath:     "",
		ValidateCertificate: true,
	})
	applyCertificateDefaultsToConfig(conf)
	return conf
}

func loadConfigFromDisk(newConfig bool) error {
	defer RecoverAndLog()
	DEBUG("Loading configurations from file")
	if !newConfig {
		return ReadConfigFileFromDisk()
	} else {
		err := ReadConfigFileFromDisk()
		if err == nil {
			return nil
		}
	}

	DEBUG("Generating a new default config")

	CONFIG.Store(DefaultConfig())
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

func CleanupOnClose() {
	defer RecoverAndLog()
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
	defer RecoverAndLog()
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

func getServerByID(server *ControlServer, deviceKey string, deviceToken string, UserID string, ServerID string) (s *types.Server, err error) {
	SID, _ := primitive.ObjectIDFromHex(ServerID)
	UID, _ := primitive.ObjectIDFromHex(UserID)

	FR := &FORWARD_REQUEST{
		Server:  server,
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
	url := FR.Server.GetURL(FR.Path)
	responseBytes, code, err := SendRequestToURL(
		nil,
		FR.Method,
		url,
		FR.JSONData,
		FR.Timeout,
		FR.Server.ValidateCertificate,
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

func GetDeviceByID(server *ControlServer, deviceID string) (d *types.Device, err error) {
	DID, _ := primitive.ObjectIDFromHex(deviceID)

	FR := &FORWARD_REQUEST{
		Server:  server,
		Path:    "/v3/device",
		Method:  "POST",
		Timeout: 10000,
		JSONData: &types.FORM_GET_DEVICE{
			DeviceID: DID,
		},
	}
	url := FR.Server.GetURL(FR.Path)
	responseBytes, code, err := SendRequestToURL(
		nil,
		FR.Method,
		url,
		FR.JSONData,
		FR.Timeout,
		FR.Server.ValidateCertificate,
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
