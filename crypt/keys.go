package crypt

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// CheckKeyFilePermissions reports whether a private key file is group/other-readable.
// Callers should log a warning and continue (or fail closed) based on policy.
func CheckKeyFilePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		return fmt.Errorf("%s has insecure permissions %o (want 0600)", path, mode)
	}
	return nil
}

func LoadPrivateKey(filePath string) (any, []byte, error) {
	keyBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, nil, errors.New("failed to decode PEM block containing private key")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return privateKey, keyBytes, nil
	}

	keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {

		return nil, nil, fmt.Errorf("failed to parse private key (tried PKCS#1 and PKCS#8): %w", err)
	}

	return keyInterface, keyBytes, nil
}

func LoadPublicKey(filePath string) (any, []byte, error) {
	keyBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, nil, errors.New("failed to decode PEM block containing public key")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract public key from cert")
	}

	return cert.PublicKey, keyBytes, nil
}
