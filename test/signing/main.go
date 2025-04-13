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

// loadPrivateKey loads an RSA private key from a PEM file.
// Supports PKCS#1 (RSA PRIVATE KEY) and PKCS#8 (PRIVATE KEY) formats.
func loadPrivateKey(filePath string) (*rsa.PrivateKey, error) {
	keyBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing private key")
	}

	// Try parsing as PKCS#1
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err == nil {
		return privateKey, nil
	}

	// If PKCS#1 parsing failed, try parsing as PKCS#8
	keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// If both failed, return the PKCS#8 error
		return nil, fmt.Errorf("failed to parse private key (tried PKCS#1 and PKCS#8): %w", err)
	}

	// Check if the parsed key is actually an RSA private key
	rsaPrivateKey, ok := keyInterface.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("parsed key is not an RSA private key (PKCS#8)")
	}

	return rsaPrivateKey, nil
}

// loadPublicKey loads an RSA public key from a PEM file (PKIX format).
func loadPublicKey(filePath string) (*rsa.PublicKey, error) {
	keyBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, errors.New("failed to decode PEM block containing public key")
	}

	// Parse the key (must be PKIX format for public keys)
	keyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	// Type assert to *rsa.PublicKey
	rsaPublicKey, ok := keyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("parsed key is not an RSA public key")
	}

	return rsaPublicKey, nil
}

// signData signs the given data using the private key specified by the path.
// It uses RSA-PSS with SHA-256 hashing.
func signData(privateKeyPath string, data []byte) ([]byte, error) {
	// Load the private key
	privateKey, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("could not load private key: %w", err)
	}

	// Hash the data using SHA-256
	hashed := sha256.Sum256(data)

	// Sign the hash using RSA-PSS
	// PSS is generally recommended over PKCS#1 v1.5 for new applications.
	opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto, Hash: crypto.SHA256}
	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, hashed[:], opts)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}

	return signature, nil
}

// verifySignature verifies the signature of the given data using the public key specified by the path.
// It expects the signature to have been created using RSA-PSS with SHA-256 hashing.
// Returns nil error if the signature is valid, otherwise returns an error.
func verifySignature(publicKeyPath string, data []byte, signature []byte) error {
	// Load the public key
	publicKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		return fmt.Errorf("could not load public key: %w", err)
	}

	// Hash the data using SHA-256 (must be the same hash used for signing)
	hashed := sha256.Sum256(data)

	// Verify the signature using RSA-PSS
	opts := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto, Hash: crypto.SHA256}
	err = rsa.VerifyPSS(publicKey, crypto.SHA256, hashed[:], signature, opts)
	if err != nil {
		// The error specifically indicates invalid signature or other issues.
		// rsa.ErrVerification is the specific error for an invalid signature.
		return fmt.Errorf("signature verification failed: %w", err)
	}

	// If err is nil, verification was successful
	return nil
}

// --- Example Usage ---
func main() {
	privateKeyFile := "private.pem"
	publicKeyFile := "public2.pem"
	originalData := []byte("This is the secret message to be signed.")

	fmt.Printf("Original Data: %s\n", string(originalData))

	// 1. Sign the data
	fmt.Printf("Signing data using %s...\n", privateKeyFile)
	signature, err := signData(privateKeyFile, originalData)
	if err != nil {
		fmt.Printf("Error signing data: %v\n", err)
		return
	}
	fmt.Printf("Signature (hex): %x\n", signature)

	// 2. Verify the signature with the correct data and public key
	fmt.Printf("\nVerifying signature using %s...\n", publicKeyFile)
	err = verifySignature(publicKeyFile, originalData, signature)
	if err == nil {
		fmt.Println("Signature is VALID.")
	} else {
		fmt.Printf("Signature verification FAILED: %v\n", err)
	}

	// 3. Try verifying with tampered data (should fail)
	fmt.Println("\nVerifying signature with TAMPERED data...")
	tamperedData := []byte("This is NOT the secret message.")
	err = verifySignature(publicKeyFile, tamperedData, signature)
	if err == nil {
		fmt.Println("Signature is VALID (ERROR: This should have failed!).")
	} else {
		fmt.Printf("Signature verification FAILED as expected: %v\n", err)
		// Check if the specific error is due to invalid signature
		if errors.Is(err, rsa.ErrVerification) {
			fmt.Println("(Failure was due to invalid signature match)")
		}
	}

	// 4. (Optional) Try verifying with a different public key (should fail)
	// Create another key pair (e.g., wrong_private.pem, wrong_public.pem)
	// err = verifySignature("wrong_public.pem", originalData, signature)
	// ... handle error ...
}
