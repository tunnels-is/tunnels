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
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

type CertType int

const (
	RSA CertType = iota
	ECDSA
)

func LoadTunnelsCACertPool() (pool *x509.CertPool, err error) {
	pool = x509.NewCertPool()
	ok := pool.AppendCertsFromPEM([]byte(CAcert1))
	if !ok {
		return nil, fmt.Errorf("Unable to load first CA certificate")
	}
	ok = pool.AppendCertsFromPEM([]byte(CAcert2))
	if !ok {
		return nil, fmt.Errorf("Unable to load second CA certificate")
	}
	return
}

func MakeCert(ct CertType, certPath string, keyPath string, ips []string, domains []string, org string, expirationDate time.Time, saveToDisk bool) (c tls.Certificate, err error) {
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
		keyFile, err = os.Create(keyPath)
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
		IsCA:                  true,
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
		certFile, err := os.Create(certPath)
		if err != nil {
			return c, err
		}
		defer certFile.Close()
		pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	}

	return tls.X509KeyPair(cb.Bytes(), kb.Bytes())
}

type Certs struct {
	Priv        any
	Pub         any
	CertPem     []byte
	KeyPem      []byte
	KeyPKCS8    []byte
	X509KeyPair tls.Certificate
	CertBytes   []byte
}

func MakeCertV2(ct CertType, certPath string, keyPath string, ips []string, domains []string, org string, expirationDate time.Time, saveToDisk bool) (CR *Certs, err error) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	CR = new(Certs)

	if ct == RSA {
		var pk *rsa.PrivateKey
		pk, err = rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return
		}
		CR.Priv = pk
		CR.Pub = &pk.PublicKey
	} else if ct == ECDSA {
		var pk *ecdsa.PrivateKey
		pk, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		if err != nil {
			return
		}
		CR.Priv = pk
		CR.Pub = &pk.PublicKey
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
		return
	}

	parsedIPs := make([]net.IP, 0)
	for _, v := range ips {
		parsedIPs = append(parsedIPs, net.ParseIP(v).To4())
	}

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
		IsCA:                  true,
		IPAddresses:           parsedIPs,
		DNSNames:              domains,
	}

	CR.CertBytes, err = x509.CreateCertificate(rand.Reader, &template, &template, CR.Pub, CR.Priv)
	if err != nil {
		return
	}

	CR.CertPem = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: CR.CertBytes})
	if CR.CertPem == nil {
		return nil, fmt.Errorf("unable to encode certificate pem")
	}

	typeString := ""
	_, ok := CR.Priv.(*rsa.PrivateKey)
	if ok {
		typeString = "RSA PRIVATE KEY"
	}
	_, ok = CR.Priv.(*ecdsa.PrivateKey)
	if ok {
		typeString = "EC PRIVATE KEY"
	}

	CR.KeyPKCS8, err = x509.MarshalPKCS8PrivateKey(CR.Priv)
	if err != nil {
		return
	}

	CR.KeyPem = pem.EncodeToMemory(&pem.Block{Type: typeString, Bytes: CR.KeyPKCS8})
	if CR.KeyPem == nil {
		return nil, fmt.Errorf("unable to encode PKCS8")
	}

	CR.X509KeyPair, err = tls.X509KeyPair(CR.CertPem, CR.KeyPem)
	if saveToDisk {
		_, err := os.Stat(certPath)
		if err != nil {
			cpem, err := os.Create(certPath)
			if err != nil {
				return nil, err
			}
			defer cpem.Close()
			if err := pem.Encode(cpem, &pem.Block{Type: "CERTIFICATE", Bytes: CR.CertBytes}); err != nil {
				return nil, fmt.Errorf("failed to write certificate data to %s: %w", certPath, err)
			}
		}

		_, err = os.Stat(keyPath)
		if err != nil {
			kpem, err := os.Create(keyPath)
			if err != nil {
				return nil, err
			}
			defer kpem.Close()
			if err := pem.Encode(kpem, &pem.Block{Type: "PRIVATE KEY", Bytes: CR.KeyPKCS8}); err != nil {
				return nil, fmt.Errorf("failed to write certificate data to %s: %w", certPath, err)
			}
		}
	}
	return
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

func ExtractSerialNumberFromCRT(path string) (serial string, err error) {
	// Read the contents of the .crt file
	var data []byte
	data, err = os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// PEM decode the certificate
	pemBlock, _ := pem.Decode(data)
	if pemBlock == nil {
		return "", fmt.Errorf("unable to decode pem block")
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%X", cert.SerialNumber), nil
}

type DNSInfo struct {
	Cert     []byte
	IP       string
	Port     string
	ServerID string
}

func ResolveMetaTXT(domain string) (info *DNSInfo, err error) {
	txt, err := net.LookupTXT(domain)
	if err != nil {
		return nil, fmt.Errorf("error in base lookup: %s", err)
	}
	// certParts := make([][]byte, 100)
	info = new(DNSInfo)
	info.Cert = make([]byte, 0)

	for _, v := range txt {
		if strings.Contains(v, "----") {
			info.Cert = []byte(v)
			// info.Cert = bytes.Replace(info.Cert, []byte("\n"), []byte{}, -1)
		} else {
			split := strings.Split(v, ":")
			if len(split) < 3 {
				return nil, errors.New("bad dns format, 0: field is less then 4 in length")
			}
			info.IP = split[0]
			info.Port = split[1]
			info.ServerID = split[2]
			continue
		}
	}
	if info.IP == "" {
		return nil, errors.New("bad dns format, IP is empty")
	}
	if info.Port == "" {
		return nil, errors.New("bad dns format, Port is empty")
	}
	if info.ServerID == "" {
		return nil, errors.New("bad dns format, ServerID is empty")
	}

	return
}
