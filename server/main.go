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
MIIGHTCCA9GgAwIBAgIUaYKyHX+VENcRlOYAYzC9K8wR7kEwQQYJKoZIhvcNAQEK
MDSgDzANBglghkgBZQMEAgEFAKEcMBoGCSqGSIb3DQEBCDANBglghkgBZQMEAgEF
AKIDAgEgMFkxCzAJBgNVBAYTAklTMRAwDgYDVQQIDAdJY2VsYW5kMRAwDgYDVQQH
DAdJY2VsYW5kMRQwEgYDVQQKDAtUdW5uZWxzIEVIRjEQMA4GA1UEAwwHdHVubmVs
czAeFw0yNDA3MDUwMzI0MDRaFw0yNTA3MDUwMzI0MDRaMFkxCzAJBgNVBAYTAklT
MRAwDgYDVQQIDAdJY2VsYW5kMRAwDgYDVQQHDAdJY2VsYW5kMRQwEgYDVQQKDAtU
dW5uZWxzIEVIRjEQMA4GA1UEAwwHdHVubmVsczCCAiIwDQYJKoZIhvcNAQEBBQAD
ggIPADCCAgoCggIBAKXjj42VHvfRdp8NSRYA+5k3B45mwBxRM45iLrrdCLXwyNKY
cLhq8tkTz9CmXS8ie9OSJk93UV1BJdBnC/c/WVHfGxVwAwivYdiAqkziikD2GvR6
qVY5kwdy9wa2uXrTsJmdZW99sNwwhzg2ckKdn4Gy8fZPvu2/eCUZhrO5zRY0VvZP
/ZufpSj3+8tBjtXcUZMh3MEebfigSGcEsVfqp+RsHrvD9K3A832uKCPhm7jXoOY5
ZC82Pi9nNwUg4s82FQjWhX8rx70LhHoWZTJpzjAB+LUYi8cutifEXHVcI7/urQH1
hJkrHEP/fLHilsPONleZXtPzfZmfQRHjwl+7iTVvQCfc3YW1vvrjukVBgPGoapxa
IDqbvPlevfIZMNRJm0ojXErE8C2L6Y2gwzyJnQX8eXrSWsLkOlsc+uwMtQiQEoxQ
A9V/gYrERvF+I58oJjlJsS+R8cChjYN9B70DrxcxyPtHvY35uBKv3RHFQXHBWEgN
q40x/8PQkluFFIkvcQV8LqFI8e3xN+YOuwdn5SC+Ta15v13vuNtJYBEcx5xCqsyY
3jGMm7I3p3HRz5xlOBVEgym1cwi3rS3EtqmQRg0YoFDsFwifRzjUixkdJAYRuCOs
Zwfmhr8jQvmsJy5I87i//TxtkX1Lck5kX2pchFZLYkiYWkDvVWGmQMYiFbg/AgMB
AAGjdTBzMDEGA1UdEQQqMCiCEmFwaS5uaWNlbGFuZHZwbi5pc4ISYXBpLm5pY2Vs
YW5kdnBuLmlzMB0GA1UdDgQWBBS/0qLiotvYAhjF9OMCDkw+nzd++DAfBgNVHSME
GDAWgBSiBmVqh25Iv9yL13i6bjTT7qRHBjBBBgkqhkiG9w0BAQowNKAPMA0GCWCG
SAFlAwQCAQUAoRwwGgYJKoZIhvcNAQEIMA0GCWCGSAFlAwQCAQUAogMCASADggIB
AJEt122JR3uCgU1Plc+J/uhTT/nOD1sYuOtU9jbEg+xrHnnl2tYY5sqzGV/CUGOm
Ppytn2OVVcnc8i9UphnV6iyqeUXTpKpSqpLEhEexcFnUI7T1aEsC8oFQtL7/Wdcx
NBH4h3e9qf8Q+qLvNgqn0n+WSnVlL1dUbICJnaYURpcWx4I1l5B+i1akoiL17jcl
aL5nesSwEFR3V23/VZo4cNKACholpli9xtD62W1d+JevZ+mD63Hb0RSpO7MDxva8
TW5T0Njw986CsDDVF6BXbBNGjrv7qB3iT5Zp+gohRRA0mSGt/38/tIM0oQ4W4rlM
RAAqpugBWOUbVGiypP2PeUKD+/6hB2RHGK+5BESFRDo95ljrNPSEZQy2E/pE7204
CvDQNLN2HnLXQt30X56EsD6qG1GgJhJIn0wWDFkis06jINNX2QLvxhdkrSHtssQs
z64P4TlHNsTeCQypVi4bvTMnjFJwOWBfqTYCrQn6Bg9UnNeH8chtO59XH8WN1NXm
vTi4XyNu6gHrJPqtndMULABA6Py1Y/PW/HpEgAR1r/FdSErbrTV1lGIXrc9bHAfu
y0z4Mr0BVwDPksFyvq9YTc00n04CcWYgvSPsSleNxWs+Xs3FJ+JCBA32JjRAmGhq
uERvglh4s+Yi3Ugkn+BpytXJMJCNmSqxRVV55Sd/5Tpu
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

func loadPublicSigningCert() (err error) {
	pubKeyBlock, _ := pem.Decode([]byte(ControlCert))
	publicSigningCert, err = x509.ParseCertificate(pubKeyBlock.Bytes)
	if err != nil {
		return err
	}
	publicSigningKey = publicSigningCert.PublicKey.(*rsa.PublicKey)

	return
}

var (
	id              string
	interfaceIP     string
	config          bool
	features        string
	enabledFeatures []string

	VPLEnabled bool = false
	VPNEnabled bool = false
	DNSEnabled bool = false
)

const (
	VPNFeature string = "VPN"
	VPLFeature string = "VPL"
	DNSFeature string = "DNS"
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
	flag.StringVar(&features, "features", "VPN,VPL", "Select enabled features. Available: VPN,VPL")
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
