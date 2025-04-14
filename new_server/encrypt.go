package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

// Constants for PBKDF2 and AES-GCM
const (
	// Use a reasonable iteration count for PBKDF2
	pbkdf2Iterations = 600000
	// AES-256 needs a 32-byte key
	keyLength = 32
	// Salt length - 16 bytes is a common choice
	saltLength = 16
	// Nonce length for GCM - 12 bytes is standard
	nonceLength = 12
)

func Encrypt(plaintext string, password []byte) (string, error) {
	if len(password) == 0 {
		return "", errors.New("password cannot be empty")
	}
	if len(plaintext) == 0 {
		return "", errors.New("plaintext cannot be empty")
	}

	salt := make([]byte, saltLength)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	key := pbkdf2.Key(password, salt, pbkdf2Iterations, keyLength, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, nonceLength) // GCM standard nonce size is 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nil, nonce, []byte(plaintext), nil)
	encryptedData := append(salt, nonce...)
	encryptedData = append(encryptedData, ciphertext...)

	return base64.StdEncoding.EncodeToString(encryptedData), nil
}

func Decrypt(encryptedBase64 string, password []byte) (string, error) {
	if len(password) == 0 {
		return "", errors.New("password cannot be empty")
	}
	if len(encryptedBase64) == 0 {
		return "", errors.New("encrypted data cannot be empty")
	}

	encryptedData, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(encryptedData) < saltLength+nonceLength {
		return "", errors.New("invalid encrypted data: too short")
	}

	salt := encryptedData[:saltLength]
	nonce := encryptedData[saltLength : saltLength+nonceLength]
	ciphertext := encryptedData[saltLength+nonceLength:]
	key := pbkdf2.Key(password, salt, pbkdf2Iterations, keyLength, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	plaintextBytes, err := aesGCM.Open(nil, nonce, ciphertext, nil) // Pass nil AAD
	if err != nil {
		return "", fmt.Errorf("failed to decrypt (check password/data integrity): %w", err)
	}

	return string(plaintextBytes), nil
}
