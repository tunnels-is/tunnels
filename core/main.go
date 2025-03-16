package core

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
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
)

func InitService() error {
	defer RecoverAndLogToFile()

	InitBaseFoldersAndPaths()
	loadConfigFromDisk()
	loadTunnelsFromDisk()
	loadDefaultGateway()
	loadDefaultInterface()
	InitDNSHandler()
	INFO("Starting Tunnels")

	set := CONFIG.Load()
	s := STATE.Load()

	// updateDNSHandlerInterface(s.DefaultInterface)

	if !set.ConsoleLogOnly {
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

	// TODO ..
	printInfo()
	printInfo2()

	INFO("Loading certificates")

	var err error
	CertPool, err = certs.LoadTunnelsCACertPool()
	if err != nil {
		DEBUG("Could not load root CA:", err)
		return err
	}

	if !set.Minimal {
		doEvent(highPriorityChannel, func() {
			err := ReBuildBlockLists()
			if err == nil {
				writeConfigToDisk()
			}
		})
	}

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

func LaunchEverything() {
	defer RecoverAndLogToFile()
	conf := CONFIG.Load()

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

	if !conf.Minimal {
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
	}

	newConcurrentSignal("Pinger", CancelContext, func() {
		PingConnections()
	})

	newConcurrentSignal("BlockListUpdater", CancelContext, func() {
		updateBlockLists()
	})

	newConcurrentSignal("LogMapCleaner", CancelContext, func() {
		CleanUniqueLogMap()
	})

	newConcurrentSignal("CleanPortAllocs", CancelContext, func() {
		CleanPortsForAllConnections()
	})

	newConcurrentSignal("StartTraceWorker", CancelContext, func() {
		StartTraceProcessor()
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

	f, err := CreateFile(s.ConfigFileName)
	if err != nil {
		ERROR("Unable to create new config", err)
		return err
	}
	defer f.Close()

	err = f.Truncate(0)
	if err != nil {
		ERROR("Unable to write config bytes to new config file: ", err)
		return err
	}

	_, err = f.Write(cb)
	if err != nil {
		ERROR("Unable to write config bytes to new config file: ", err)
		return err
	}

	return
}

func (m *TUN) LoadPrivateCerts(certpath string) (p *x509.CertPool, err error) {
	if len(m.ServerCertBytes) > 0 {
		return LoadPrivateCertFromBytes(m.ServerCertBytes)
	}
	return LoadPrivateCert(certpath)
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
		tf, err := CreateFile(s.TunnelsPath + string(os.PathSeparator) + t.Tag + tunnelFileSuffix)
		if err != nil {
			ERROR("Unable to save tunnel to disk:", err)
			outErr = err
			return false
		}

		err = tf.Truncate(0)
		if err != nil {
			ERROR("Unable to truncate tunnel json to file:", err)
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

		tb, err := os.ReadFile(path)
		if err != nil {
			ERROR("Unable to read tunnel file:", err)
			return err
		}

		tunnel := new(TunnelMETA)
		err = json.Unmarshal(tb, tunnel)
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
		newTun := createDefaultTunnelMeta()
		TunnelMetaMap.Store(newTun.Tag, newTun)
		writeTunnelsToDisk(newTun.Tag)
	}
}

func loadConfigFromDisk() {
	defer RecoverAndLogToFile()
	DEBUG("Loading configurations from file")

	err := ReadConfigFileFromDisk()
	if err == nil {
		return
	}

	DEBUG("Generating a new default config")

	Conf := new(configV2)
	Conf.DarkMode = true

	Conf.DebugLogging = true
	Conf.InfoLogging = true
	Conf.ErrorLogging = true
	Conf.ConnectionTracer = false

	Conf.DNSServerIP = "127.0.0.1"
	Conf.DNSServerPort = "53"
	Conf.DNS1Default = "1.1.1.1"
	Conf.DNS2Default = "8.8.8.8"
	Conf.LogBlockedDomains = true
	Conf.LogAllDomains = true
	Conf.DNSstats = true
	Conf.AvailableBlockLists = GetDefaultBlockLists()

	Conf.APIIP = "127.0.0.1"
	Conf.APIPort = "7777"
	applyCertificateDefaultsToConfig(Conf)
	Conf.APICertType = certs.ECDSA

	CONFIG.Store(Conf)
	writeConfigToDisk()
}

func applyCertificateDefaultsToConfig(cfg *configV2) {
	if cfg.APIKey == "" {
		cfg.APIKey = "./api.key"
	}
	if cfg.APICert == "" {
		cfg.APICert = "./api.crt"
	}

	cfg.APICertType = certs.RSA

	if cfg.APICertIPs == nil || len(cfg.APICertIPs) < 1 {
		cfg.APICertIPs = []string{"127.0.0.1"}
	}

	if cfg.APICertDomains == nil || len(cfg.APICertDomains) < 1 {
		cfg.APICertDomains = []string{"tunnels.app", "app.tunnels.is"}
	}
	return
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
		tunnel.Disconnect(tun)
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
		openURL(url)
	case "darwin":
		openURL(url)
	default:
		if !isWSL() {
			openURL(url)
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
