package client

import (
	"crypto/x509"
	"fmt"
	"os"
)

// LoadPrivateCertFromBytes loads a certificate pool from byte data
func LoadPrivateCertFromBytes(data []byte) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(data)
	return certPool, nil
}

// LoadPrivateCert loads a certificate pool from a file path
func LoadPrivateCert(path string) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	certData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	certPool.AppendCertsFromPEM(certData)
	return certPool, nil
}

// LoadCertPEMBytes loads certificate PEM bytes into a cert pool
func (m *TUN) LoadCertPEMBytes(cert []byte) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(cert)
	if !ok {
		return certPool, fmt.Errorf("unable to append cert")
	}
	return certPool, nil
}
