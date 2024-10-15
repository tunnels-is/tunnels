package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/net/quic"
)

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

func loadPublicSigningCert(path string) (err error) {
	pubKeyPem, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	pubKeyBlock, _ := pem.Decode(pubKeyPem)
	publicSigningCert, err = x509.ParseCertificate(pubKeyBlock.Bytes)
	if err != nil {
		return err
	}
	publicSigningKey = publicSigningCert.PublicKey.(*rsa.PublicKey)

	return
}

func main() {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()

	if len(os.Args) > 1 {
		if os.Args[1] == "config" {
			makeConfigAndCertificates()
		}
		os.Exit(1)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	var err error

	Config, err = GetServerConfig(serverConfigPath)
	if err != nil {
		panic(err)
	}

	if Config.UserMaxConnections < 1 {
		Config.UserMaxConnections = 2
	}

	err = loadPublicSigningCert(Config.SignKey)
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

	// _, err = GetServersFromFile()
	// if err != nil {
	// 	panic(err)
	// }

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

	sc := new(Server)
	sc.ID = primitive.ObjectID{}
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
	sc.SignKey = "./controller.crt"

	N := new(ServerNetwork)
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
