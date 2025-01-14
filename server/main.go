package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/jackpal/gateway"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/iptables"
	"github.com/tunnels-is/tunnels/setcap"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/net/quic"
)

var ControlCert = `-----BEGIN CERTIFICATE-----
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
bQKaDDbUGfsdOTGh0H1A
-----END CERTIFICATE-----`

func LOG(x ...any) {
	log.Println(x...)
}

func INFO(x ...any) {
	log.Println(x...)
}

func WARN(x ...any) {
	log.Println(x...)
}

func ERR(x ...any) {
	log.Println(x...)
}

var (
	id              string
	interfaceIP     string
	config          bool
	features        string
	defaultHostname string
	enabledFeatures []string

	VPLEnabled bool = false
	VPNEnabled bool = false
	DNSEnabled bool = false
	APIEnabled bool = false
)

const (
	VPNFeature string = "VPN"
	VPLFeature string = "VPL"
	DNSFeature string = "DNS"
	APIFeature string = "API"
)

func isFeatureEnabled(feature string) bool {
	return slices.Contains(enabledFeatures, feature)
}

func main() {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.StringVar(&id, "id", "", "Tunnels ID used when generating the config. NOTE: not including and id will skip config generation but the certificate will still be generated.")
	flag.StringVar(&interfaceIP, "interfaceIP", "", "InterfaceIP used when generating config and certificates")
	flag.BoolVar(&config, "config", false, "Generate a config and make certificates ( Remember to copy the serial number ! )")
	flag.StringVar(&features, "features", "", "Select enabled features. Available: VPN,VPL,API")
	flag.StringVar(&defaultHostname, "hostname", "", "Main domain/hostname for DHCP devices")
	flag.Parse()

	if config {
		makeConfigAndCertificates()
		os.Exit(1)
	}

	enabledFeatures = strings.Split(features, ",")
	if len(features) == 0 {
		fmt.Println("you need to enabled at least one feature use --help for more information")
		os.Exit(0)
	}

	var err error
	Config, err = GetServerConfig(serverConfigPath)
	if err != nil {
		fmt.Println("Error loading config: ", err)
		os.Exit(0)
	}

	for i := range enabledFeatures {
		switch enabledFeatures[i] {
		case APIFeature:
			APIEnabled = true
			fmt.Println("Enabling API Feature..")
		case VPNFeature:
			VPNEnabled = true
			fmt.Println("Enabling VPN Feature..")
		case VPLFeature:
			VPLEnabled = true
			fmt.Println("Enabling VPL Feature..")
			if Config.VPL == nil {
				fmt.Println("VPL Configuration missing")
				os.Exit(0)
			}
		case DNSFeature:
			fmt.Println("Enabling DNS Feature..")
			fmt.Println("DNS Feature is in development")
			DNSEnabled = true
		default:
			fmt.Println("Unknown feature: ", enabledFeatures[i])
			os.Exit(0)
		}
	}

	if VPNEnabled {
		initializeVPN()
	}

	if VPLEnabled {
		initializeVPL()
	}

	if Config.UserMaxConnections < 1 {
		Config.UserMaxConnections = 2
	}

	initializeCertsAndTLSConfig()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())

	// GENERIC ROUTINES
	SignalMonitor <- NewSignal(ctx, 1)
	SignalMonitor <- NewSignal(ctx, 2)
	SignalMonitor <- NewSignal(ctx, 3)
	SignalMonitor <- NewSignal(ctx, 4)
	SignalMonitor <- NewSignal(ctx, 5)

	if VPNEnabled {
		SignalMonitor <- NewSignal(ctx, 10)
		SignalMonitor <- NewSignal(ctx, 11)
		SignalMonitor <- NewSignal(ctx, 12)
	}

	if VPLEnabled {
		SignalMonitor <- NewSignal(ctx, 20)
		SignalMonitor <- NewSignal(ctx, 21)
		SignalMonitor <- NewSignal(ctx, 22)
	}

	for {
		select {
		case signal := <-quit:

			cancel()
			WARN("EXIT", signal)
			return

		case index := <-toUserChannelMonitor:
			go toUserChannel(index)
		case index := <-fromUserChannelMonitor:
			go fromUserChannel(index)

		case SIGNAL := <-SignalMonitor:
			LOG(SIGNAL)

			switch SIGNAL.ID {
			case 1:
				go pingActiveUsers(SIGNAL)
			case 2:
				go ControlSocketListener(SIGNAL)
			case 3:
				go DataSocketListener(SIGNAL)
			case 4:
				go startAPI(SIGNAL)
			case 5:

			// VPN
			case 10:
				go ExternalUDPListener(SIGNAL)
			case 11:
				go ExternalTCPListener(SIGNAL)
			case 12:

			// VPL
			case 20:
			case 21:
			case 22:

			}
		}
	}
}

func initializeVPN() {
	err := setcap.CheckCapabilities()
	if err != nil {
		fmt.Println("Tunnels server is missing capabilities, err:", err)
		os.Exit(1)
	}

	var existed bool
	err, existed = iptables.SetIPTablesRSTDropFilter(Config.InterfaceIP)
	if err != nil {
		fmt.Println("Error applying iptables rule: ", err)
		os.Exit(1)
	}
	if !existed {
		fmt.Println("> added iptables rule")
	}

	InterfaceIP = net.ParseIP(Config.InterfaceIP)
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

	GeneratePortAllocation()
}

func initializeVPL() (err error) {
	err = generateDHCPMap()
	if err != nil {
		return
	}

	if Config.VPL != nil {
		AllowAll = Config.VPL.AllowAll
	}
	return
}

func initializeCertsAndTLSConfig() {
	var err error
	publicSigningCert, publicSigningKey, err = certs.LoadServerSignCertAndKey()
	if err != nil {
		panic(err)
	}

	controlCertificate, err = tls.LoadX509KeyPair(Config.ControlCert, Config.ControlKey)
	if err != nil {
		panic(err)
	}

	controlConfig = &tls.Config{
		MinVersion:       tls.VersionTLS13,
		MaxVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.CurveP521},
		Certificates:     []tls.Certificate{controlCertificate},
	}

	quicConfig = &quic.Config{
		TLSConfig:                controlConfig,
		RequireAddressValidation: false,
		HandshakeTimeout:         time.Duration(10 * time.Second),
		KeepAlivePeriod:          0,
		MaxUniRemoteStreams:      500,
		MaxBidiRemoteStreams:     500,
		MaxStreamReadBufferSize:  70000,
		MaxStreamWriteBufferSize: 70000,
		MaxConnReadBufferSize:    70000,
		MaxIdleTimeout:           60 * time.Second,
	}
}

func makeConfigAndCertificates() {
	ep, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	eps := strings.Split(ep, "/")
	ep = strings.Join(eps[:len(eps)-1], "/")
	ep += "/"

	if interfaceIP == "" {
		IFIP, err := gateway.DiscoverInterface()
		if err != nil {
			panic(err)
		}
		interfaceIP = IFIP.String()
	}

	if id != "" {
		oid, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			fmt.Printf("--id invalid, err: %s", err)
			os.Exit(1)
		}
		sc := new(Server)
		sc.ID = oid
		sc.ControlIP = interfaceIP
		sc.InterfaceIP = interfaceIP
		sc.APIPort = "444"
		sc.ControlPort = "444"
		sc.DataPort = "443"
		sc.StartPort = 2000
		sc.EndPort = 65500
		sc.UserMaxConnections = 4
		sc.AvailableMbps = 1000
		sc.AvailableUserMbps = 10
		sc.InternetAccess = true
		sc.LocalNetworkAccess = true
		sc.DNSAllowCustomOnly = false
		sc.DNS = make([]*ServerDNS, 0)
		sc.Networks = make([]*ServerNetwork, 0)
		sc.DNSServers = []string{"1.1.1.1", "8.8.8.8"}
		sc.ControlCert = ep + "server.crt"
		sc.ControlKey = ep + "server.key"

		N := new(ServerNetwork)
		N.Tag = "default"
		N.Network = interfaceIP + "/24"
		N.Nat = "10.10.10.1/24"
		sc.Networks = append(sc.Networks, N)

		sc.VPL = new(VPLSettings)
		sc.VPL.Network = new(ServerNetwork)
		sc.VPL.Network.Tag = "VPL"
		sc.VPL.Network.Network = "10.0.0.0/16"
		sc.VPL.Network.Nat = ""
		sc.VPL.Network.Routes = []*Route{
			{
				Address: "10.0.0.0/16",
				Metric:  "0",
			},
		}

		sc.VPL.MaxDevices = math.MaxUint16
		sc.VPL.AllowAll = true

		f, err := os.Create(ep + "server.json")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		encoder := json.NewEncoder(f)
		encoder.SetIndent("", "    ")

		// Encode the config to the file
		if err := encoder.Encode(sc); err != nil {
			fmt.Println("Error encoding JSON:", err)
		} else {
			fmt.Println("Config file has been written successfully.")
		}

	}

	_, err = certs.MakeCert(
		certs.ECDSA,
		ep+"server.crt",
		ep+"server.key",
		[]string{interfaceIP},
		[]string{""},
		"",
		time.Time{},
		true,
	)
	if err != nil {
		panic(err)
	}

	serialN, err := certs.ExtractSerialNumberFromCRT(ep + "server.crt")
	fmt.Println("CERT SERIAL NUMBER: ", serialN)
	f, err := os.Create(ep + "serial")
	if err != nil {
		panic("unable to create folder for serial number")
	}
	if f != nil {
		defer f.Close()
	}
	_, err = f.WriteString(serialN)
	if err != nil {
		panic("unable to write serial number to file")
	}
}

func GeneratePortAllocation() (err error) {
	slots = Config.AvailableMbps / Config.AvailableUserMbps
	portPerUser := (Config.EndPort - Config.StartPort) / slots

	defer func() {
		BasicRecover()
		if err != nil {
			panic(err)
		}
	}()

	currentPort := uint16(Config.StartPort)

	for uc := 0; uc < slots; uc++ {
		PR := new(PortRange)
		PR.StartPort = uint16(currentPort)
		PR.EndPort = PR.StartPort + uint16(portPerUser)

		// log.Println("ASSIGNING PORTS: ", PR.StartPort, " >> ", PR.EndPort)
		for i := PR.StartPort; i <= PR.EndPort; i++ {

			if i < PR.StartPort {
				return errors.New("start port is too small")
			} else if i > PR.EndPort {
				return errors.New("end port is too big")
			}

			if PortToCoreMapping[i] != nil {
				if PortToCoreMapping[i].StartPort < PR.StartPort {
					return errors.New("start port is too small")
				}
				if PortToCoreMapping[i].StartPort < PR.EndPort {
					return errors.New("end port is too big")
				}
			}

			PortToCoreMapping[i] = PR
		}

		currentPort = PR.EndPort + 1
	}

	return nil
}
