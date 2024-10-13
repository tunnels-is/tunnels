package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/certs"
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
		if Config.AutoCert {
			if Config.Cert != nil {
				controlCertificate, err = certs.MakeCert(
					certs.RSA,
					Config.ControlCert,
					Config.ControlKey,
					Config.Cert.IPs,
					Config.Cert.Domains,
					Config.Cert.Org,
					Config.Cert.Expires,
					true,
				)
				if err != nil {
					panic(err)
				}
			} else {
				controlCertificate, err = certs.MakeCert(
					certs.RSA,
					Config.ControlCert,
					Config.ControlKey,
					[]string{Config.ControlIP, Config.InterfaceIP},
					[]string{""},
					"",
					time.Time{},
					true,
				)
				if err != nil {
					panic(err)
				}
			}
		} else {
			panic(err)
		}
	}

	masterSerial := certs.ExtractSerialNumberHex(controlCertificate)

	controlConfig = &tls.Config{
		MinVersion:       tls.VersionTLS13,
		MaxVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.CurveP521},
		Certificates:     []tls.Certificate{controlCertificate},
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) > 0 {
				Serial := fmt.Sprintf("%x", cs.PeerCertificates[0].SerialNumber)
				fmt.Println(Serial, " <> ", masterSerial)
			}
			return nil
		},
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
