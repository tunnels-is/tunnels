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
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
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
	id     string
	config bool
)

func main() {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()
	runtime.GOMAXPROCS(runtime.NumCPU())

	err := setcap.CheckCapabilities()
	if err != nil {
		panic(err)
	}

	flag.StringVar(&id, "id", "", "Include your servers ID. This ID can be found in the Tunnels UI")
	flag.BoolVar(&config, "config", false, "Generate a config")
	flag.Parse()

	if config {
		if id == "" {
			fmt.Println("--id missing")
			os.Exit(1)
		}
		makeConfigAndCertificates()
		os.Exit(1)
	}

	Config, err = GetServerConfig(serverConfigPath)
	if err != nil {
		panic(err)
	}

	var existed bool
	err, existed = iptables.SetIPTablesRSTDropFilter(Config.InterfaceIP)
	if err != nil {
		panic(err)
	}
	if !existed {
		fmt.Println("> added iptables rule")
	}

	if Config.UserMaxConnections < 1 {
		Config.UserMaxConnections = 2
	}

	err = loadPublicSigningCert()
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

	ctx, cancel := context.WithCancel(context.Background())

	SignalMonitor <- NewSignal(ctx, 1)
	SignalMonitor <- NewSignal(ctx, 2)
	SignalMonitor <- NewSignal(ctx, 3)
	SignalMonitor <- NewSignal(ctx, 100)
	SignalMonitor <- NewSignal(ctx, 101)
	SignalMonitor <- NewSignal(ctx, 102)
	SignalMonitor <- NewSignal(ctx, 443)
	SignalMonitor <- NewSignal(ctx, 444)

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	GeneratePortAllocation()

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

			if SIGNAL.ID == 1 {
				//
			} else if SIGNAL.ID == 2 {
				//
			} else if SIGNAL.ID == 3 {
				go pingActiveUsers(SIGNAL)
			} else if SIGNAL.ID == 100 {
				go ExternalUDPListener(SIGNAL)
			} else if SIGNAL.ID == 101 {
				go ExternalTCPListener(SIGNAL)
			} else if SIGNAL.ID == 102 {
				//
			} else if SIGNAL.ID == 443 {
				go ControlSocketListener(SIGNAL)
			} else if SIGNAL.ID == 444 {
				go DataSocketListener(SIGNAL)
			}
		}
	}
}

func makeConfigAndCertificates() {
	InterfaceIP, err := gateway.DiscoverInterface()
	if err != nil {
		panic(err)
	}

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		fmt.Printf("--id invalid, err: %s", err)
		os.Exit(1)
	}
	sc := new(Server)
	sc.ID = oid
	sc.ControlIP = InterfaceIP.String()
	sc.InterfaceIP = InterfaceIP.String()
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
	sc.ControlCert = "./server.crt"
	sc.ControlKey = "./server.key"

	N := new(ServerNetwork)
	N.Tag = "default"
	N.Network = InterfaceIP.String() + "/24"
	N.Nat = "10.10.10.1/24"
	sc.Networks = append(sc.Networks, N)

	outConfig, err := json.Marshal(sc)
	if err != nil {
		panic(err)
	}
	f, err := os.Create("./server.json")
	if err != nil {
		panic(err)
	}
	_, err = f.Write(outConfig)
	if err != nil {
		panic(err)
	}

	_, err = certs.MakeCert(
		certs.ECDSA,
		sc.ControlCert,
		sc.ControlKey,
		[]string{InterfaceIP.String()},
		[]string{""},
		"",
		time.Time{},
		true,
	)
	if err != nil {
		panic(err)
	}

	serialN, err := certs.ExtractSerialNumberFromCRT(sc.ControlCert)
	fmt.Println("CERT SERIAL NUMBER: ", serialN)
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

			if PortToUserMapping[i] != nil {
				if PortToUserMapping[i].StartPort < PR.StartPort {
					return errors.New("start port is too small")
				}
				if PortToUserMapping[i].StartPort < PR.EndPort {
					return errors.New("end port is too big")
				}
			}

			PortToUserMapping[i] = PR
		}

		currentPort = PR.EndPort + 1
	}

	return nil
}
