package core

import (
	"bufio"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/certs"
	// _ "net/http/pprof"
)

func InitService() error {
	defer RecoverAndLogToFile()

	INFO("loader", "Starting Tunnels")

	initializeGlobalVariables()

	_ = OSSpecificInit()
	AdminCheck()
	InitPaths()

	printInfo()

	CreateBaseFolder()
	if !MINIMAL {
		InitLogfile()
	}

	go StartLogQueueProcessor(routineMonitor)

	if MINIMAL && CLIDNS != "" {
		LoadDNSConfig()
	} else {
		LoadConfig()
	}

	printInfo2()

	if !MINIMAL {
		InitDNSHandler()
		InitBlockListPath()

		go func() {
			err := ReBuildBlockLists(C)
			if err == nil {
				SaveConfig(C)
				SwapConfig(C)
			}
		}()
	}

	if GLOBAL_STATE.C == nil {
		ERROR("", "Global state could not be set.. possible config issue")
		time.Sleep(3 * time.Second)
		return errors.New("unable to create global state.. possible config error")
	}

	var err error
	INFO("Loading certificates")
	CertPool, err = certs.LoadTunnelsCACertPool()
	if err != nil {
		DEBUG("Could not load root CA:", err)
		time.Sleep(3 * time.Second)
		return err
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
	fmt.Println("")
	fmt.Println("")
	fmt.Println("==============================================================")
	fmt.Println("======================= TUNNELS.IS ===========================")
	fmt.Println("==============================================================")
	fmt.Println("NOTE: If the app closes without any logs/errors you might need to delete your config and try again")
	fmt.Println("")
}

func printInfo2() {
	fmt.Println("")
	fmt.Println("=======================================================================")
	fmt.Println("======================= HELPFUL INFORMATION ===========================")
	fmt.Println("=======================================================================")
	fmt.Println("")
	fmt.Printf("APP: https://%s:%s\n", C.APIIP, C.APIPort)
	fmt.Println("")
	fmt.Println("BASE PATH:", GLOBAL_STATE.BasePath)
	fmt.Println("")
	fmt.Println("- Tunnels request network admin permissions to run.")
	fmt.Println("- Remember to configure your DNS servers if you want to use Tunnels DNS functionality.")
	fmt.Println("- Remember to turn all logging off if you are concerned about privacy.")
	fmt.Println("- There is a --basePath flag that can let you reconfigure the base directory for logs and configs.")
	fmt.Println("")
	fmt.Println("=======================================================================")
	fmt.Println("=======================================================================")
}

type X *bool

var (
	routineMonitor   = make(chan int, 200)
	interfaceMonitor = make(chan *TunnelInterface, 200)
	tunnelMonitor    = make(chan *Tunnel, 200)
)

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

	// already stared
	// routineMonitor <- 1

	routineMonitor <- 3
	routineMonitor <- 4
	routineMonitor <- 5
	routineMonitor <- 6

	if !MINIMAL {
		routineMonitor <- 2
		routineMonitor <- 7

		// DNS
		routineMonitor <- 101
		routineMonitor <- 102
		routineMonitor <- 103
	}

	if MINIMAL {
		routineMonitor <- 200
	}

	for {
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

		case ID := <-routineMonitor:
			if ID == 1 {
				go StartLogQueueProcessor(routineMonitor)
			} else if ID == 2 {
				go LaunchAPI(routineMonitor)
			} else if ID == 3 {
				go PingConnections(routineMonitor)
			} else if ID == 4 {
				go GetDefaultGateway(routineMonitor)
			} else if ID == 5 {
				//
			} else if ID == 6 {
				go CleanPortsForAllConnections(routineMonitor)
			} else if ID == 7 {
				go StartTraceProcessor(routineMonitor)
			} else if ID == 101 {
				go CleanDNSCache(routineMonitor)
			} else if ID == 102 {
				//
			} else if ID == 103 {
				go StartUDPDNSHandler(routineMonitor)
			} else if ID == 200 {
				go AutoConnect(routineMonitor)
			}
		default:
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func SaveConfig(c *Config) (err error) {
	var config *os.File
	defer func() {
		if config != nil {
			_ = config.Close()
		}
	}()
	defer RecoverAndLogToFile()

	// c.Version = GLOBAL_STATE.Version

	cb, err := json.Marshal(c)
	if err != nil {
		ERROR("Unable to marshal config into bytes: ", err)
		return err
	}

	config, err = os.Create(GLOBAL_STATE.ConfigPath)
	if err != nil {
		ERROR("Unable to create new config", err)
		return err
	}

	_, err = config.Write(cb)
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

	err := LoadExistingConfig()
	GLOBAL_STATE.C = C
	GLOBAL_STATE.ConfigInitialized = true
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
					GLOBAL_STATE.C = C
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

	fmt.Println("APPEND CONNECTION")
	C.Connections = []*TunnelMETA{newCon}
	C.DNSstats = false
	C.AvailableBlockLists = make([]*BlockList, 0)

	GLOBAL_STATE.C = C
	GLOBAL_STATE.ConfigInitialized = true
	SaveConfig(C)
	DEBUG("Configurations loaded")
}

func LoadExistingConfig() (err error) {
	GLOBAL_STATE.ConfigPath = GLOBAL_STATE.BasePath + "config.json"
	config, err := os.ReadFile(GLOBAL_STATE.ConfigPath)
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

	var config *os.File
	var err error
	defer func() {
		if config != nil {
			_ = config.Close()
		}
	}()

	if GLOBAL_STATE.C != nil {
		DEBUG("Config already loaded")
		return
	}

	// GLOBAL_STATE.ConfigPath = GLOBAL_STATE.BasePath + "config.json"
	DEBUG("Loading config from: ", GLOBAL_STATE.ConfigPath)
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

		config, err = os.Create(GLOBAL_STATE.ConfigPath)
		if err != nil {
			ERROR("Unable to create new config file: ", err)
			return
		}

		err = os.Chmod(GLOBAL_STATE.ConfigPath, 0o777)
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

	GLOBAL_STATE.C = C
	if len(C.AvailableBlockLists) == 0 && !CLIDisableBlockLists {
		C.AvailableBlockLists = GetDefaultBlockLists()
		GLOBAL_STATE.C.AvailableBlockLists = C.AvailableBlockLists
		SaveConfig(C)
	}

	GLOBAL_STATE.ConfigInitialized = true
	DEBUG("Configurations loaded")
}

func SwapConfig(newConfig *Config) {
	C = newConfig
	GLOBAL_STATE.C = C
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

// func REF_PingRouter(routerIP, gateway string) (*ping.Statistics, error) {
// 	defer RecoverAndLogToFile()
//
// 	DEBUG("PING: ", routerIP, " || gateway: ", gateway)
//
// 	pinger, err := ping.NewPinger(routerIP)
// 	if err != nil {
// 		ERROR("unable to create a new pinger: ", routerIP, err)
// 		return nil, err
// 	}
// 	defer pinger.Stop()
//
// 	// routeAdded := false
// 	_ = IP_AddRoute(
// 		routerIP+"/32",
// 		strconv.Itoa(DEFAULT_INTERFACE_ID),
// 		DEFAULT_GATEWAY.To4().String(),
// 		"0",
// 	)
// 	// if err == nil {
// 	// routeAdded = true
// 	// }
//
// 	pinger.SetPrivileged(true)
// 	pinger.Count = 1
// 	pinger.Timeout = time.Second * 3
// 	err = pinger.Run()
// 	if err != nil {
// 		ERROR("PING ERROR: ", routerIP, err)
// 		return nil, err
// 	}
//
// 	// if routeAdded {
// 	// err = tunnels.IP_DelRoute(routerIP, gateway, "10")
// 	// if err != nil {
// 	// 	ERROR("unable to delete route to: ", routerIP, err)
// 	// }
// 	// }
//
// 	return pinger.Statistics(), nil
// }

// func PingAllServers() {
// 	defer RecoverAndLogToFile()
//
// 	for i := range GLOBAL_STATE.Servers {
// 		if GLOBAL_STATE.Servers[i] == nil {
// 			continue
// 		}
//
// 		stats, err := REF_PingRouter(GLOBAL_STATE.Servers[i].IP, DEFAULT_GATEWAY.To4().String())
// 		if err != nil {
// 			continue
// 		}
//
// 		if stats.AvgRtt.Microseconds() == 0 {
// 			INFO(GLOBAL_STATE.Servers[i].IP, " // OFFLINE")
// 			GLOBAL_STATE.Servers[i].PingStats = *stats
// 			GLOBAL_STATE.Servers[i].MS = 9999
// 		} else {
// 			GLOBAL_STATE.Servers[i].PingStats = *stats
// 			GLOBAL_STATE.Servers[i].MS = uint64(stats.AvgRtt.Milliseconds())
// 			INFO(GLOBAL_STATE.Servers[i].IP, " // MS: ", GLOBAL_STATE.Servers[i].PingStats.AvgRtt)
// 		}
//
// 	}
// }
