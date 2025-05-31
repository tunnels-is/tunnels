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

func main() {
	secretText := "This is my super secret message!"
	// IMPORTANT: Use a secure, high-entropy 32-byte password/key in a real application
	// This is just an example. Consider how you will securely manage this password.
	password := []byte("3!-@#0a,.b5445E58C3!(}6cc9e8e6d6")
	// password := []byte("my-secure-32-byte-password-1234") // Example password (exactly 32 bytes)
	if len(password) != 32 {
		fmt.Println("Warning: Example password is not exactly 32 bytes.")
		// In a real scenario, you might pad, hash, or truncate, but it's better
		// to use a KDF like PBKDF2 which handles this properly and securely
		// The code above uses PBKDF2, so the input password length isn't strictly fixed *here*,
		// but the derived key will be 32 bytes.
	}

	fmt.Printf("Original Text: %s\n", secretText)
	fmt.Printf("Password (bytes): %v\n", password) // Don't log passwords in real apps!

	// Encrypt
	encrypted, err := Encrypt(secretText, password)
	if err != nil {
		fmt.Printf("Encryption failed: %v\n", err)
		return
	}
	fmt.Printf("Encrypted (Base64): %s\n", encrypted)

	// Decrypt
	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		fmt.Printf("Decryption failed: %v\n", err)
		return
	}
	fmt.Printf("Decrypted Text: %s\n", decrypted)

	// Test with wrong password
	fmt.Println("\n--- Attempting decryption with wrong password ---")
	wrongPassword := []byte("incorrect-password-xxxxxxxxxxxxx")
	_, err = Decrypt(encrypted, wrongPassword)
	if err != nil {
		fmt.Printf("Decryption failed as expected: %v\n", err) // Should fail
	} else {
		fmt.Println("Error: Decryption succeeded with wrong password!")
	}
}
