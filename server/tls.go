package main

import (
	"crypto/tls"
	"fmt"
	"os"

	"github.com/tunnels-is/tunnels/crypt"
)

func secretFileWorldAccessible(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().Perm()&0o007 != 0
}

func loadKeyPair(key, cert string) (c tls.Certificate, err error) {
	if err := crypt.CheckKeyFilePermissions(key); err != nil {
		if secretFileWorldAccessible(key) {
			return c, err
		}
		logger.Warn("TLS private key is group-accessible (continuing)", "path", key, "err", err)
	}
	_, priv, err := crypt.LoadPrivateKey(key)
	if err != nil {
		return c, err
	}
	_, pub, err := crypt.LoadPublicKey(cert)
	if err != nil {
		return c, err
	}
	c, err = tls.X509KeyPair(pub, priv)
	if err != nil {
		return c, err
	}

	return c, nil
}

func loadCertificatesAndTLSSettings() (err error) {
	keyPem := loadSecret("KeyPem")
	if err := crypt.CheckKeyFilePermissions(keyPem); err != nil {
		if secretFileWorldAccessible(keyPem) {
			return err
		}
		logger.Warn("TLS private key is group-accessible (continuing)", "path", keyPem, "err", err)
	}
	_, privB, err := crypt.LoadPrivateKey(keyPem)
	if err != nil {
		return err
	}
	_, pubB, err := crypt.LoadPublicKey(loadSecret("CertPem"))
	if err != nil {
		return err
	}
	tlscert, err := tls.X509KeyPair(pubB, privB)
	if err != nil {
		return err
	}
	KeyPair.Store(&tlscert)

	apiCerts := []tls.Certificate{}
	keyPems := loadStringSliceKey("KeyPems")
	CertPems := loadStringSliceKey("CertPems")
	if len(keyPems) != len(CertPems) {
		return fmt.Errorf("config KeyPems (%d) and CertPems (%d) must have the same length", len(keyPems), len(CertPems))
	}
	for i := range keyPems {
		tlsc, err := loadKeyPair(keyPems[i], CertPems[i])
		if err != nil {
			return err
		}
		apiCerts = append(apiCerts, tlsc)
	}

	apiCerts = append(apiCerts, *KeyPair.Load())

	APITLSConfig.Store(&tls.Config{
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768},
		Certificates:     apiCerts,
	})

	return nil
}
