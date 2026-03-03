package client

import (
	"crypto/x509"
	"fmt"
	"os"
)

func LoadPrivateCertFromBytes(data []byte) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(data)
	return certPool, nil
}

func LoadPrivateCert(path string) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	certData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	certPool.AppendCertsFromPEM(certData)
	return certPool, nil
}

func (m *TUN) LoadCertPEMBytes(cert []byte) (pool *x509.CertPool, err error) {
	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(cert)
	if !ok {
		return certPool, fmt.Errorf("unable to append cert")
	}
	return certPool, nil
}
