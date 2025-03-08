package core

import (
	"bufio"
	"context"
	"crypto/x509"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/certs"
)

func InitService() error {
	defer RecoverAndLogToFile()
	INFO("Starting Tunnels")
	cli := CLI.Load()

	InitBaseFoldersAndPaths()

	if cli.DNS != "" {
		LoadDNSConfig()
	} else {
		LoadConfig()
	}

	s := STATE.Load()

	if !s.ConsoleLogOnly {
		var err error
		LogFile, err = CreateFile(*s.LogFileName.Load())
		if err != nil {
			return err
		}

		// TraceFile, err = CreateFile(*s.TraceFileName.Load())
		// if err != nil {
		// 	panic(err)
		// }
	}

	INFO("Operating specific initializations")
	_ = OSSpecificInit()
	INFO("Checking permissins")
	AdminCheck()

	// TODO ..
	printInfo()
	printInfo2()

	InitDNSHandler()

	INFO("Loading certificates")

	var err error
	CertPool, err = certs.LoadTunnelsCACertPool()
	if err != nil {
		DEBUG("Could not load root CA:", err)
		return err
	}

	if !MINIMAL {
		doEvent(highPriorityChannel, func() {
			err := ReBuildBlockLists()
			if err == nil {
				SaveConfig()
			}
		})
	}

	INFO("Tunnels is ready")
	return nil
}

func (m *TunnelMETA) LoadPrivateCerts() (p *x509.CertPool, err error) {
	if len(m.PrivateCertBytes) > 0 {
		return LoadPrivateCertFromBytes(m.PrivateCertBytes)
	}
	return LoadPrivateCert(m.PrivateCert)
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
	s := STATE.Load()
	log.Println("")
	log.Println("=======================================================================")
	log.Println("======================= HELPFUL INFORMATION ===========================")
	log.Println("=======================================================================")
	log.Println("")
	log.Printf("APP: https://%s:%s\n", s.APIIP, s.APIPort)
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

type X *bool

func LaunchEverything() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

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

	if !MINIMAL {
		newConcurrentSignal("APIServer", CancelContext, func() {
			LaunchAPI()
		})
		newConcurrentSignal("UDPDNSHandler", CancelContext, func() {
			StartUDPDNSHandler()
		})
		newConcurrentSignal("OpenUI", CancelContext, func() {
			popUI()
		})
	}

	newConcurrentSignal("Pinger", CancelContext, func() {
		PingConnections()
	})

	newConcurrentSignal("GatewayChecker", CancelContext, func() {
		GetDefaultGateway()
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
			high.method()
			continue mainLoop
		case med := <-mediumPriorityChannel:
			med.method()
			continue mainLoop
		case low := <-lowPriorityChannel:
			low.method()
			continue mainLoop
		default:
		}

		select {
		case sig := <-quit:
			DEBUG("", "exit signal caught: ", sig)
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

func SaveConfig() (err error) {
	defer RecoverAndLogToFile()
	s := STATE.Load()

	cb, err := json.Marshal(s)
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

	_, err = f.Write(cb)
	if err != nil {
		ERROR("Unable to write config bytes to new config file: ", err)
		return err
	}

	return
}

func LoadDNSConfig() {
	defer func() {
		r := recover()
		if r != nil {
			ERROR(r, string(debug.Stack()))
		}
	}()
	DEBUG("Loading DNS Configurations")

	err := LoadExistingConfig()
	STATEOLD.C = C
	if err == nil {
		for i := range C.Connections {
			if C.Connections[i] == nil {
				continue
			}
			if C.Connections[i].Tag == DefaultTunnelNameMin {
				changed := false
				if C.Connections[i].DeviceKey != CLIDeviceKey && CLIDeviceKey != "" {
					C.Connections[i].Hostname = CLIDeviceKey
					DEBUG("Updated device key to: ", CLIDeviceKey)
					changed = true
				}

				if C.Connections[i].Hostname != CLIHostname && CLIHostname != "" {
					C.Connections[i].Hostname = CLIHostname
					DEBUG("Updated hostname to: ", CLIHostname)
					changed = true
				}

				if C.Connections[i].DNSDiscovery != CLIDNS && CLIDNS != "" {
					C.Connections[i].DNSDiscovery = CLIDNS
					DEBUG("Updated DNS Discovery to: ", CLIDNS)
					changed = true
				}

				if changed {
					STATEOLD.C = C
					SaveConfig(C)
					break
				}
			}
		}

		DEBUG("Loaded configurations from disk")
		return
	}

	C = new(Config)
	C.InfoLogging = true
	C.ErrorLogging = true
	C.ConsoleLogOnly = true
	C.ConsoleLogging = true
	C.DebugLogging = true

	DEBUG("Generating a new minimal config")

	newCon := createMinimalConnection()

	C.Connections = []*TunnelMETA{newCon}
	C.DNSstats = false
	C.AvailableBlockLists = make([]*BlockList, 0)

	STATEOLD.C = C
	SaveConfig(C)
	DEBUG("Configurations loaded")
}

func LoadExistingConfig() (err error) {
	STATEOLD.ConfigFileName = STATEOLD.BasePath + "config.json"
	config, err := os.ReadFile(STATEOLD.ConfigFileName)
	if err != nil {
		return
	}

	err = json.Unmarshal(config, C)
	if err != nil {
		ERROR("Unable to turn config file into config object: ", err)
		return
	}

	return
}

func LoadConfig() {
	defer func() {
		r := recover()
		if r != nil {
			ERROR(r, string(debug.Stack()))
		}
	}()
	DEBUG("Loading configurations from file")

	var config *os.File
	var err error
	defer func() {
		if config != nil {
			_ = config.Close()
		}
	}()

	if STATEOLD.C != nil {
		DEBUG("Config already loaded")
		return
	}

	// GLOBAL_STATE.ConfigPath = GLOBAL_STATE.BasePath + "config.json"
	DEBUG("Loading config from: ", STATEOLD.ConfigFileName)
	// config, err = os.Open(GLOBAL_STATE.ConfigPath)
	err = LoadExistingConfig()
	if err != nil {

		DEBUG("Generating a new default config")

		NC := new(Config)
		NC.DebugLogging = false
		NC.InfoLogging = true
		NC.ErrorLogging = true
		NC.IsolationMode = false
		NC.ConnectionTracer = false

		NC.DarkMode = false

		NC.DNSServerIP = "127.0.0.1"
		NC.DNSServerPort = "53"
		NC.DNS1Default = "1.1.1.1"
		NC.DNS2Default = "8.8.8.8"
		NC.LogBlockedDomains = true
		NC.LogAllDomains = true

		NC.APIIP = "127.0.0.1"
		NC.APIPort = "7777"
		applyCertificateDefaults(NC)
		NC.APICertType = certs.ECDSA

		newCon := createDefaultTunnelMeta()
		NC.Connections = make([]*TunnelMETA, 0)
		NC.Connections = append(NC.Connections, newCon)

		NC.DNSstats = false
		NC.AvailableBlockLists = GetDefaultBlockLists()

		var cb []byte
		cb, err = json.Marshal(NC)
		if err != nil {
			ERROR("Unable to turn new config into bytes: ", err)
			return
		}

		config, err = os.Create(STATEOLD.ConfigFileName)
		if err != nil {
			ERROR("Unable to create new config file: ", err)
			return
		}

		err = os.Chmod(STATEOLD.ConfigFileName, 0o777)
		if err != nil {
			ERROR("Unable to change ownership of log file: ", err)
			return
		}

		_, err = config.Write(cb)
		if err != nil {
			ERROR("Unable to write config bytes to new config file: ", err)
			return
		}

		C = NC
	}

	applyCertificateDefaults(C)

	STATEOLD.C = C
	if len(C.AvailableBlockLists) == 0 && !CLIDisableBlockLists {
		C.AvailableBlockLists = GetDefaultBlockLists()
		STATEOLD.C.AvailableBlockLists = C.AvailableBlockLists
		SaveConfig(C)
	}

	DEBUG("Configurations loaded")
}

func applyCertificateDefaults(cfg *Config) {
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

func LoadDNSWhitelist() (err error) {
	defer RecoverAndLogToFile()

	if C.DomainWhitelist == "" {
		return nil
	}

	WFile, err := os.OpenFile(C.DomainWhitelist, os.O_RDWR|os.O_CREATE, 0o777)
	if err != nil {
		return err
	}
	defer WFile.Close()

	scanner := bufio.NewScanner(WFile)

	WhitelistMap := make(map[string]bool)
	for scanner.Scan() {
		domain := scanner.Text()
		if domain == "" {
			continue
		}
		WhitelistMap[domain] = true
	}

	err = scanner.Err()
	if err != nil {
		ERROR("Unable to load domain whitelist: ", err)
		return err
	}

	DNSWhitelist = WhitelistMap

	return nil
}

func CleanupOnClose() {
	defer RecoverAndLogToFile()
	// CleanupWithStateLock()
	for _, v := range TunList {
		if v == nil {
			continue
		}
		if v.Interface != nil {
			_ = v.Interface.Disconnect(v)
		}
	}
	// Keeping this here for now
	// RestoreDNSOnClose()
	_ = LogFile.Close()
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
