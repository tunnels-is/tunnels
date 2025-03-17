package crypto

import (
	"testing"
)

func TestNewKey(t *testing.T) {
	key, err := NewKey("test-key")
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}
	if key == nil {
		t.Error("Expected key to be non-nil")
	}
	if len(key.key) != 32 { // SHA-256 produces 32 bytes
		t.Errorf("Expected key length to be 32, got %d", len(key.key))
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key, err := NewKey("test-key")
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}

	// Test data
	plaintext := []byte("Hello, World!")

	// Encrypt
	ciphertext, err := key.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}
	if len(ciphertext) <= len(plaintext) {
		t.Error("Expected ciphertext to be longer than plaintext")
	}

	// Decrypt
	decrypted, err := key.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("Expected decrypted text to be '%s', got '%s'", plaintext, decrypted)
	}
}

func TestEncryptDecryptString(t *testing.T) {
	key, err := NewKey("test-key")
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}

	// Test data
	plaintext := "Hello, World!"

	// Encrypt
	ciphertext, err := key.EncryptString(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt string: %v", err)
	}
	if len(ciphertext) <= len(plaintext) {
		t.Error("Expected ciphertext to be longer than plaintext")
	}

	// Decrypt
	decrypted, err := key.DecryptString(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt string: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("Expected decrypted text to be '%s', got '%s'", plaintext, decrypted)
	}
}

func TestEncryptDecryptEmpty(t *testing.T) {
	key, err := NewKey("test-key")
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}

	// Test empty data
	plaintext := []byte{}

	// Encrypt
	ciphertext, err := key.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt empty data: %v", err)
	}

	// Decrypt
	decrypted, err := key.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Failed to decrypt empty data: %v", err)
	}
	if len(decrypted) != 0 {
		t.Error("Expected decrypted data to be empty")
	}
}

func TestDecryptInvalidData(t *testing.T) {
	key, err := NewKey("test-key")
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}

	// Test invalid data
	invalidData := []byte("invalid data")

	// Attempt to decrypt
	_, err = key.Decrypt(invalidData)
	if err == nil {
		t.Error("Expected error when decrypting invalid data")
	}
}
