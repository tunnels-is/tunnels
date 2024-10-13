package certs

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"runtime/debug"
	"time"
)

type certType int

const (
	RSA certType = iota
	ECDSA
)

func MakeCert(ct certType, certPath string, keyPath string, ips []string, domains []string, org string, expirationDate time.Time, saveToDisk bool) (c tls.Certificate, err error) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	var privateKey any
	var publicKey any
	key := make([]byte, 0)
	kb := bytes.NewBuffer(key)
	var gg []byte
	var keyFile *os.File

	if saveToDisk {
		keyFile, err = os.Create("server.key")
		if err != nil {
			return c, err
		}
		defer keyFile.Close()
	}

	if ct == RSA {
		pk, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return c, err
		}
		privateKey = pk
		publicKey = &pk.PublicKey
		gg, err = x509.MarshalPKCS8PrivateKey(pk)
		pem.Encode(kb, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: gg})
		if saveToDisk && keyFile != nil {
			pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: gg})
		}

	} else if ct == ECDSA {
		pk, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		if err != nil {
			return c, err
		}
		privateKey = pk
		publicKey = &pk.PublicKey
		gg, err = x509.MarshalPKCS8PrivateKey(pk)
		pem.Encode(kb, &pem.Block{Type: "EC PRIVATE KEY", Bytes: gg})
		if saveToDisk && keyFile != nil {
			pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: gg})
		}
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
		return c, err
	}

	parsedIPs := make([]net.IP, 0)
	for _, v := range ips {
		parsedIPs = append(parsedIPs, net.ParseIP(v).To4())
	}

	// Create a self-signed certificate template
	if org == "" {
		org = "Tunnels Server"
	}
	if expirationDate.IsZero() {
		expirationDate = time.Now().Add(10 * 365 * 24 * time.Hour)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{org},
			OrganizationalUnit: []string{"networking"},
		},
		NotBefore:             time.Now(),
		NotAfter:              expirationDate,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           parsedIPs,
		DNSNames:              domains,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey, privateKey)
	if err != nil {
		return c, err
	}

	cert := make([]byte, 0)
	cb := bytes.NewBuffer(cert)
	pem.Encode(cb, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	if saveToDisk {
		certFile, err := os.Create("server.crt")
		if err != nil {
			return c, err
		}
		defer certFile.Close()
		pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	}

	return tls.X509KeyPair(cb.Bytes(), kb.Bytes())
}

func ExtractSerialNumberHex(cert tls.Certificate) string {
	if cert.Leaf == nil {
		return ""
	}
	serialNumber := cert.Leaf.SerialNumber
	serialBytes := serialNumber.Bytes()
	serialHex := hex.EncodeToString(serialBytes)
	return serialHex
}
