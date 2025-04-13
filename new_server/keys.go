package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

func loadPrivateKey(filePath string) (*rsa.PrivateKey, []byte, error) {
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
		return privateKey, block.Bytes, nil
	}

	keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// If both failed, return the PKCS#8 error
		return nil, nil, fmt.Errorf("failed to parse private key (tried PKCS#1 and PKCS#8): %w", err)
	}

	rsaPrivateKey, ok := keyInterface.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, errors.New("parsed key is not an RSA private key (PKCS#8)")
	}

	return rsaPrivateKey, block.Bytes, nil
}

func loadPublicKey(filePath string) (*rsa.PublicKey, []byte, error) {
	keyBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, nil, errors.New("failed to decode PEM block containing public key")
	}

	keyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPublicKey, ok := keyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, nil, errors.New("parsed key is not an RSA public key")
	}

	return rsaPublicKey, block.Bytes, nil
}

func signData(data []byte) ([]byte, error) {
	privateKey := PrivKey.Load()
	hashed := sha256.Sum256(data)
	opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto, Hash: crypto.SHA256}
	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, hashed[:], opts)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	return signature, nil
}

func verifySignature(data []byte, signature []byte) error {
	publicKey := PubKey.Load()
	hashed := sha256.Sum256(data)
	opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto, Hash: crypto.SHA256}
	err := rsa.VerifyPSS(publicKey, crypto.SHA256, hashed[:], signature, opts)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}
