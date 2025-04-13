package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

func anyToPrivateKeys(key any) (RSA *rsa.PrivateKey, EC *ecdsa.PrivateKey) {
	RSA, _ = key.(*rsa.PrivateKey)
	EC, _ = key.(*ecdsa.PrivateKey)
	return
}
func anyToPublicKeys(key any) (RSA *rsa.PublicKey, EC *ecdsa.PublicKey) {
	RSA, _ = key.(*rsa.PublicKey)
	EC, _ = key.(*ecdsa.PublicKey)
	return
}

func loadPrivateKey(filePath string) (any, []byte, error) {
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
		// If both failed, return the PKCS#8 error
		return nil, nil, fmt.Errorf("failed to parse private key (tried PKCS#1 and PKCS#8): %w", err)
	}

	return keyInterface, keyBytes, nil
}

func loadPublicKey(filePath string) (any, []byte, error) {
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

func signData(data []byte) ([]byte, error) {
	privateKey := PrivKey.Load()
	hashed := sha256.Sum256(data)

	rsaKey, ecKey := anyToPrivateKeys(privateKey)
	if rsaKey != nil {
		opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto, Hash: crypto.SHA256}
		signature, err := rsa.SignPSS(rand.Reader, rsaKey, crypto.SHA256, hashed[:], opts)
		if err != nil {
			return nil, fmt.Errorf("failed to sign data using rsa key: %w", err)
		}
		return signature, nil
	} else if ecKey != nil {
		signature, err := ecdsa.SignASN1(rand.Reader, ecKey, hashed[:])
		if err != nil {
			return nil, fmt.Errorf("failed to sign data using ec key: %w", err)
		}
		return signature, nil
	}

	return nil, fmt.Errorf("no valid private key found")
}

func verifySignature(data []byte, signature []byte) error {
	publicKey := PubKey.Load()
	hashed := sha256.Sum256(data)

	rsaKey, ecKey := anyToPublicKeys(publicKey)
	if rsaKey != nil {
		opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto, Hash: crypto.SHA256}
		err := rsa.VerifyPSS(rsaKey, crypto.SHA256, hashed[:], signature, opts)
		if err != nil {
			return fmt.Errorf("rsa signature verification failed: %w", err)
		}
	} else if ecKey != nil {
		ok := ecdsa.VerifyASN1(ecKey, hashed[:], signature)
		if !ok {
			return fmt.Errorf("ec signature verification failed")
		}

	}
	return nil
}
