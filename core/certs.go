package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"time"
)

type certType int

const (
	RSA = iota
	ECDSA
)

func MakeCert(ct certType, ips []string, domains []string) (c tls.Certificate, err error) {
	// Generate a private key

	var privateKey any
	var publicKey any
	key := make([]byte, 0)
	kb := bytes.NewBuffer(key)

	if ct == RSA {
		pk, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return c, err
		}
		privateKey = pk
		publicKey = &pk.PublicKey
		// gg := x509.MarshalPKCS1PrivateKey(pk)
		gg, err := x509.MarshalPKCS8PrivateKey(pk)
		pem.Encode(kb, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: gg})

	} else if ct == ECDSA {
		pk, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		if err != nil {
			return c, err
		}
		privateKey = pk
		publicKey = &pk.PublicKey
		gg, err := x509.MarshalPKCS8PrivateKey(pk)
		pem.Encode(kb, &pem.Block{Type: "EC PRIVATE KEY", Bytes: gg})
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
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization:       []string{"Tunnels EHF"},
			OrganizationalUnit: []string{"administration"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // Valid for 10 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           parsedIPs,
		DNSNames:              domains,
	}

	// Generate the certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey, privateKey)
	if err != nil {
		return c, err
	}

	cert := make([]byte, 0)
	cb := bytes.NewBuffer(cert)
	pem.Encode(cb, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	return tls.X509KeyPair(cb.Bytes(), kb.Bytes())

	// Save the certificate and private key to files
	// certFile, err := os.Create("server.crt")
	// if err != nil {
	// 	panic(err)
	// }
	// defer certFile.Close()

	//
	// keyFile, err := os.Create("server.key")
	// if err != nil {
	// 	panic(err)
	// }
	// defer keyFile.Close()
	// gg, err := x509.MarshalECPrivateKey(privateKey)
	// pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: gg})

	// gg := x509.MarshalPKCS1PrivateKey(privateKey)
	// pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: gg})
}
