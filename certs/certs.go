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
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"runtime/debug"
	"strconv"
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
	Cert  []byte
	IP    string
	Port  string
	OrgID string
}

func ResolveMetaTXT(domain string) (info *DNSInfo, err error) {
	txt, err := net.LookupTXT(domain)
	if err != nil {
		return nil, err
	}
	certParts := make([][]byte, 100)
	info = new(DNSInfo)

	for _, v := range txt {
		split := strings.Split(v, ":")
		if split[0] == "0" {
			if len(split) < 4 {
				return nil, errors.New("bad dns format, 0: field is less then 4 in length")
			}
			info.IP = split[1]
			info.Port = split[2]
			info.OrgID = split[3]
			continue
		}
		index, err := strconv.Atoi(split[0])
		if err != nil {
			return nil, err
		}
		certParts[index] = []byte(strings.Join(split[1:], ""))
	}

	preCert := make([]byte, 0)
	clen := 0
	for _, dnsTxt := range certParts {
		for _, certBytes := range dnsTxt {
			preCert = append(preCert, certBytes)
			clen = len(preCert)
		}
	}

	// We could use a formula to find the aprox. length of encoded
	// content, or we can just make sure the slice is big enough
	// and strip the end.
	info.Cert = make([]byte, clen+500)
	base64.RawStdEncoding.Decode(info.Cert, preCert)
	info.Cert = bytes.ReplaceAll(info.Cert, []byte{0}, []byte{})
	fmt.Println(info.IP, info.Port, info.OrgID)
	fmt.Println(string(info.Cert))
	return
}
