package core

import (
	"bufio"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	CreateBaseFolder()
	InitLogfile()
	LoadConfig()
	InitDNSHandler()
	if C.InfoLogging {
		printInfo()
	}

	go func() {
		InitBlockListPath()
		err := ReBuildBlockLists(C)
		if err == nil {
			SaveConfig(C)
			SwapConfig(C)
		}
	}()

	if GLOBAL_STATE.C == nil {
		ERROR("", "Global state could not be set.. possible config issue")
		return errors.New("unable to create global state.. possible config error")
	}

	err := LoadCA()
	if err != nil {
		INFO("", "Could not load root CA")
		return errors.New("could not load root CA")
	}

	INFO("Tunnels is ready")
	return nil
}

var CAcert = `-----BEGIN CERTIFICATE-----
MIIGCjCCA76gAwIBAgIUbOdy8n2a9Ao6Qdy3ar4DncTmax0wQQYJKoZIhvcNAQEK
MDSgDzANBglghkgBZQMEAgEFAKEcMBoGCSqGSIb3DQEBCDANBglghkgBZQMEAgEF
AKIDAgEgMFkxCzAJBgNVBAYTAklTMRAwDgYDVQQIDAdJY2VsYW5kMRAwDgYDVQQH
DAdJY2VsYW5kMRQwEgYDVQQKDAtUdW5uZWxzIEVIRjEQMA4GA1UEAwwHdHVubmVs
czAeFw0yNDA3MDUwMzEzNDlaFw0zNDA3MDMwMzEzNDlaMFkxCzAJBgNVBAYTAklT
MRAwDgYDVQQIDAdJY2VsYW5kMRAwDgYDVQQHDAdJY2VsYW5kMRQwEgYDVQQKDAtU
dW5uZWxzIEVIRjEQMA4GA1UEAwwHdHVubmVsczCCAiIwDQYJKoZIhvcNAQEBBQAD
ggIPADCCAgoCggIBALWEnLmsnBeGs80H9cowNK5naFsxpmOm0D3FZINupNPqeGnp
Z7WUSfPp8p8HEhEoQQkuZLW+pyP7dBIt5S1gcM8hccQKVZsD16B5d/YC9znjDAZP
Vq7FX6aOJzqVNPMdtzSqjj+nN2+T8rQFv3JRPjrzTyUJSQo6WviI8usu6st2CplV
5bsYQYV/HADU5i8DfjQ5jK8hnR+66EYu9epW20pKjJ0iNsBU9UwJk+IazrjE8gf3
ZDGc/cv2KN9hGslIIXRSb3KXmCalPncDNB1VExLc7nJg+8jBn3hTinReREE043IP
4YITNR3twj1+JVkkAjoH4sT7BL7tPf9U1w6vFbXQhuEo2mjVVqW+TUSqFzLqwGrD
yeGTRvQTSL41vpNO6tpYYKjKjfFyhojrP6iCCRe+kh6qiEmjNGCaxJmxgrWg3knC
j6eAJOZ/w7YDfdWAfI8zwIQ0VwfiR3eGbEku3Jrl/492gM/6efLbmLGglKwWzfhK
/Njm3xWglDi5UTXNWzJJ544RZeVPcXFdAO14Szz+vgDBvYvCQQTvOvexeS9qhykI
z24TCiXwrGQ3frP9G1bowxdX+lTInMLOkvb4sxazG4uZUuXvvmDFATZc2C6WR24H
GCIS6bALJdp1UWdZHNtFd/O7HrOX2W5H171Ip2NGL0uSqHZCtOS9U8BTStBPAgMB
AAGjYjBgMAwGA1UdEwQFMAMBAf8wMQYDVR0RBCowKIISYXBpLm5pY2VsYW5kdnBu
LmlzghJhcGkubmljZWxhbmR2cG4uaXMwHQYDVR0OBBYEFKIGZWqHbki/3IvXeLpu
NNPupEcGMEEGCSqGSIb3DQEBCjA0oA8wDQYJYIZIAWUDBAIBBQChHDAaBgkqhkiG
9w0BAQgwDQYJYIZIAWUDBAIBBQCiAwIBIAOCAgEAp7kRHI2IrLswy4NSLOsMs+xR
zr4k1N1dyF2vVFAQbv5wlvkLKBDtb9DvahEZjbuGW/uT4SI1UxTm1Z/BaeTNqIuz
QIbcPfC6hJ2kOkO6Uzo7rGFiZbYJ1/vZLLx89yc03bnf3Vp7FFRG0y5d9VscSSVq
jVFGebYb/MoF7l70Rx7a5Rkv/rCJ/xawl/y1mctRA1o4FSVwcwHVpzxyytcblQwn
Ybiy/cLBNU8s3Epoj7hB2ruOY02FBLDkozG34NYJUrq95eL+a0LYb+AKdHzWts0M
/U8kE5sbQpSVYxidiBS33q6uTKxrE1yYizRKGfyykeovRraRVu6wIOwDaVKZi4AX
pgdqDFJFBbrAd5KZtJMlwI2eoDmY3pOsi63ecSxwOSSr+0ZZ009vFFNsY92XzVMZ
baJKJuGkKFcpWoF2CMcoe52Hj1SuRA+3yYtMYQfZE8p77mVWnEORoVkgXX96fWjq
tQBsxq4Nb6GrkR4M4Ql7aYlcj47y0ILBTjeTMhAPb/qxSrUYhIAxeMOjCcvzQtaF
BXYclK7t+CCarFAwmQ439SQ2a/x0c9w+oDDV9PI2YqYWaqsqNtuLaq3rrUwh9unI
LAbdR4NFNWbV4vO+lXTrvwEtlJE8WsSwvpCMZABs6CzRpAnZgOq350ZKEf1V0yfI
PRsYUYN/Y3GAN8csBfs=
-----END CERTIFICATE-----`

func LoadPrivateCert(path string) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	certData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	certPool.AppendCertsFromPEM(certData)
	return certPool, nil
}

func LoadCA() (err error) {
	CertPool = x509.NewCertPool()
	CertPool.AppendCertsFromPEM([]byte(CAcert))
	return
}

func printInfo() {
	fmt.Println("=======================================================================")
	fmt.Println("======================= HELPFUL INFORMATION ===========================")
	fmt.Println("=======================================================================")
	fmt.Println("")
	fmt.Println("- Tunnels request network admin permissions to run.")
	fmt.Println("- Remember to configure your DNS servers if you want to use Tunnels DNS functionality.")
	fmt.Println("- The UI can be found here: https://"+C.APIIP+":"+C.APIPort, " -- This might change depending on settings.")
	fmt.Println("- Remember to turn all logging off if you are concerned about privacy.")
	fmt.Println("- There is a --basePath flag that can let you reconfigure the base directory for logs and configs.")
	fmt.Println("")
	fmt.Println("=======================================================================")
	fmt.Println("=======================================================================")
	fmt.Println("")
	fmt.Println("NOTE: If the app closes without any logs/errors you will need to delete your config")
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

	routineMonitor <- 1
	routineMonitor <- 2
	routineMonitor <- 3
	routineMonitor <- 4
	routineMonitor <- 5
	routineMonitor <- 6
	routineMonitor <- 7
	routineMonitor <- 8
	routineMonitor <- 9

	routineMonitor <- 101
	routineMonitor <- 102
	routineMonitor <- 103
	routineMonitor <- 104

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
				go StartAPI(routineMonitor)
			} else if ID == 3 {
				go PingConnections(routineMonitor)
			} else if ID == 4 {
				go GetDefaultGateway(routineMonitor)
			} else if ID == 5 {
			} else if ID == 6 {
				go CleanPortsForAllConnections(routineMonitor)
			} else if ID == 7 {
				go StartTraceProcessor(routineMonitor)
			} else if ID == 101 {
				go CleanDNSCache(routineMonitor)
			} else if ID == 102 {
				// go StartTCPDNSHandler(routineMonitor)
			} else if ID == 103 {
				go StartUDPDNSHandler(routineMonitor)
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

// var GLOBAL_STATE.ConfigPath string

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

	GLOBAL_STATE.ConfigPath = GLOBAL_STATE.BasePath + "config.json"
	DEBUG("Loading config from: ", GLOBAL_STATE.ConfigPath)
	config, err = os.Open(GLOBAL_STATE.ConfigPath)
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
	} else {

		var cb []byte
		cb, err = io.ReadAll(config)
		if err != nil {
			ERROR("Unable to read bytes from config file: ", err)
			return
		}

		err = json.Unmarshal(cb, C)
		if err != nil {
			ERROR("Unable to turn config file into config object: ", err)
			return
		}

		if len(C.AvailableBlockLists) == 0 {
			C.AvailableBlockLists = GetDefaultBlockLists()
			GLOBAL_STATE.C.AvailableBlockLists = C.AvailableBlockLists
			SaveConfig(C)
		}

	}

	applyCertificateDefaults(C)

	GLOBAL_STATE.C = C
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
	for _, v := range ConList {
		if v == nil {
			continue
		}
		if v.Interface != nil {
			_ = v.Interface.Disconnect(v)
		}
	}
	RestoreDNSOnClose()
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
