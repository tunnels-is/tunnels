package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/jackpal/gateway"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/iptables"
	"github.com/tunnels-is/tunnels/setcap"
	"golang.org/x/net/quic"
)

func LOG(x ...any) {
	if !disableLogs {
		log.Println(x...)
	}
}

func INFO(x ...any) {
	if !disableLogs {
		log.Println(x...)
	}
}

func WARN(x ...any) {
	if !disableLogs {
		log.Println(x...)
	}
}

func ERR(x ...any) {
	if !disableLogs {
		log.Println(x...)
	}
}

func loadPublicSigningCert() (err error) {
	pubKeyBlock, _ := pem.Decode([]byte(certs.ControllerSigningCert))
	publicSigningCert, err = x509.ParseCertificate(pubKeyBlock.Bytes)
	if err != nil {
		return err
	}
	publicSigningKey = publicSigningCert.PublicKey.(*rsa.PublicKey)

	return
}

var (
	interfaceIP     string
	config          bool
	features        string
	defaultHostname string
	enabledFeatures []string
	disableLogs     bool

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

func main() {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()
	runtime.GOMAXPROCS(runtime.NumCPU())

	flag.StringVar(&interfaceIP, "interfaceIP", "", "InterfaceIP used when generating config and certificates")
	flag.BoolVar(&config, "config", false, "Generate a config and make certificates ( Remember to copy the serial number ! )")
	flag.StringVar(&features, "features", "", "Select enabled features. Available: VPN,VPL,API")
	flag.StringVar(&defaultHostname, "hostname", "", "Main domain/hostname for DHCP devices")
	flag.BoolVar(&disableLogs, "disableLogs", false, "Disable all logging")
	flag.Parse()

	if config {
		makeConfigAndCertificates()
		os.Exit(1)
	}

	enabledFeatures = strings.Split(features, ",")
	if len(features) == 0 {
		ERR("you need to enabled at least one feature use --help for more information")
		os.Exit(0)
	}

	var err error
	Config, err = GetServerConfig(serverConfigPath)
	if err != nil {
		ERR("Error loading config: ", err)
		os.Exit(0)
	}

	for i := range enabledFeatures {
		switch enabledFeatures[i] {
		case APIFeature:
			APIEnabled = true
			INFO("Enabling API Feature..")
		case VPNFeature:
			VPNEnabled = true
			INFO("Enabling VPN Feature..")
		case VPLFeature:
			VPLEnabled = true
			INFO("Enabling VPL Feature..")
			if Config.VPL == nil {
				INFO("VPL Configuration missing")
				os.Exit(0)
			}
		case DNSFeature:
			INFO("Enabling DNS Feature..")
			WARN("DNS Feature is in development")
			DNSEnabled = true
		default:
			ERR("Unknown feature: ", enabledFeatures[i])
			os.Exit(0)
		}
	}

	if VPNEnabled {
		initializeVPN()
	}

	if VPLEnabled {
		err = initializeVPL()
		if err != nil {
			ERR("unable to initialize VPL")
			os.Exit(1)
		}
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
			// LOG(SIGNAL.ID)

			switch SIGNAL.ID {
			case 1:
				go pingActiveUsers(SIGNAL)
			case 2:
				go ControlSocketListener(SIGNAL)
			case 3:
				go ExternalSocketListener(SIGNAL)
			case 4:
				go startAPI(SIGNAL)
			case 5:
				go ReloadConfig(SIGNAL)

			// VPN
			case 12:
				go DataSocketListener(SIGNAL)
			// VPL
			case 20:
			case 21:
			case 22:

			}
		}
	}
}

func ReloadConfig(SIGNAL *SIGNAL) {
	defer RecoverAndReturnID(SIGNAL, 30)
	newConf, err := GetServerConfig(serverConfigPath)
	if err != nil {
		ERR("Error loading config: ", err)
		os.Exit(0)
	}
	if newConf != nil {
		Config = newConf
	}
}

func initializeVPN() {
	err := setcap.CheckCapabilities()
	if err != nil {
		ERR("Tunnels server is missing capabilities, err:", err)
		os.Exit(1)
	}

	var existed bool
	err, existed = iptables.SetIPTablesRSTDropFilter(Config.InterfaceIP)
	if err != nil {
		ERR("Error applying iptables rule: ", err)
		os.Exit(1)
	}
	if !existed {
		INFO("> added iptables rule")
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

	err = GeneratePortAllocation()
	if err != nil {
		panic(err)
	}
	GenerateVPLCoreMappings()
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
	err := loadPublicSigningCert()
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
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768, tls.CurveP521},
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

	sc := new(Server)
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
		ERR("Error encoding JSON:", err)
	} else {
		INFO("Config file has been written successfully.")
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
	if err != nil {
		panic(err)
	}
	INFO("CERT SERIAL NUMBER: ", serialN)
	f, err = os.Create(ep + "serial")
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

func GenerateVPLCoreMappings() {
	VPLIPToCore[10] = make([][][]*UserCoreMapping, 11)
	VPLIPToCore[10][0] = make([][]*UserCoreMapping, 256)

	for ii := range 256 {
		VPLIPToCore[10][0][ii] = make([]*UserCoreMapping, 256)

		for iii := range 256 {
			VPLIPToCore[10][0][ii][iii] = nil
		}
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

	for range slots {
		PR := new(PortRange)
		PR.StartPort = uint16(currentPort)
		PR.EndPort = PR.StartPort + uint16(portPerUser)

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
