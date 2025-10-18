package client

import (
	"bytes"
	"crypto/aes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	// Test data
	testCases := []struct {
		name      string
		plaintext []byte
		keySize   int
	}{
		{
			name:      "empty string",
			plaintext: []byte(""),
			keySize:   32, // AES-256
		},
		{
			name:      "short message",
			plaintext: []byte("Hello, World!"),
			keySize:   32,
		},
		{
			name:      "longer message",
			plaintext: []byte("This is a longer test message with more characters to encrypt and decrypt properly."),
			keySize:   32,
		},
		{
			name:      "binary data",
			plaintext: []byte{0x00, 0x01, 0x02, 0x03, 0xFF, 0xFE, 0xFD, 0xFC},
			keySize:   32,
		},
		{
			name:      "unicode characters",
			plaintext: []byte("Hello 世界! 🌍🔒"),
			keySize:   32,
		},
		{
			name:      "AES-128 key",
			plaintext: []byte("Test with 128-bit key"),
			keySize:   16,
		},
		{
			name:      "AES-192 key",
			plaintext: []byte("Test with 192-bit key"),
			keySize:   24,
		},
		{
			name:      "large block of text",
			plaintext: bytes.Repeat([]byte("Lorem ipsum dolor sit amet. "), 100),
			keySize:   32,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate a key of appropriate size
			key := make([]byte, tc.keySize)
			for i := range key {
				key[i] = byte(i % 256)
			}

			// Encrypt
			ciphertext, err := Encrypt(tc.plaintext, key)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Verify ciphertext is longer (includes IV)
			if len(ciphertext) < aes.BlockSize {
				t.Errorf("Ciphertext too short: got %d bytes, expected at least %d", len(ciphertext), aes.BlockSize)
			}

			// Extract IV and encrypted data
			iv := ciphertext[:aes.BlockSize]
			encryptedData := ciphertext[aes.BlockSize:]

			// Decrypt
			decrypted, err := Decrypt(encryptedData, iv, key)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			// Verify round-trip
			if !bytes.Equal(decrypted, tc.plaintext) {
				t.Errorf("Decrypted data does not match original\nOriginal:  %q\nDecrypted: %q", tc.plaintext, decrypted)
			}

			t.Logf("Successfully encrypted and decrypted %d bytes ✓", len(tc.plaintext))
		})
	}
}

func TestEncryptProducesUniqueOutputs(t *testing.T) {
	plaintext := []byte("The same plaintext encrypted twice")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	// Encrypt same data twice
	ciphertext1, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	ciphertext2, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	// The ciphertexts should be different due to random IV
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Two encryptions of the same plaintext produced identical ciphertext (IV should be random)")
	}

	// But both should decrypt to the same plaintext
	iv1 := ciphertext1[:aes.BlockSize]
	encrypted1 := ciphertext1[aes.BlockSize:]
	decrypted1, err := Decrypt(encrypted1, iv1, key)
	if err != nil {
		t.Fatalf("First decryption failed: %v", err)
	}

	iv2 := ciphertext2[:aes.BlockSize]
	encrypted2 := ciphertext2[aes.BlockSize:]
	decrypted2, err := Decrypt(encrypted2, iv2, key)
	if err != nil {
		t.Fatalf("Second decryption failed: %v", err)
	}

	if !bytes.Equal(decrypted1, plaintext) || !bytes.Equal(decrypted2, plaintext) {
		t.Error("Decrypted data does not match original plaintext")
	}

	t.Log("Encrypt produces unique outputs with random IVs ✓")
}

func TestDecryptWithWrongKey(t *testing.T) {
	plaintext := []byte("Secret message")
	correctKey := make([]byte, 32)
	wrongKey := make([]byte, 32)

	for i := range correctKey {
		correctKey[i] = byte(i)
		wrongKey[i] = byte(255 - i) // Different key
	}

	// Encrypt with correct key
	ciphertext, err := Encrypt(plaintext, correctKey)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with wrong key
	iv := ciphertext[:aes.BlockSize]
	encrypted := ciphertext[aes.BlockSize:]
	decrypted, err := Decrypt(encrypted, iv, wrongKey)
	if err != nil {
		t.Fatalf("Decryption with wrong key failed: %v", err)
	}

	// Decrypted data should be garbage, not the original plaintext
	if bytes.Equal(decrypted, plaintext) {
		t.Error("Decryption with wrong key produced correct plaintext (should be garbage)")
	}

	t.Log("Decryption with wrong key produces garbage as expected ✓")
}

func TestDecryptWithWrongIV(t *testing.T) {
	plaintext := []byte("Another secret message")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	// Encrypt
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Use wrong IV
	wrongIV := make([]byte, aes.BlockSize)
	for i := range wrongIV {
		wrongIV[i] = 0xFF
	}
	encrypted := ciphertext[aes.BlockSize:]

	// Decrypt with wrong IV
	decrypted, err := Decrypt(encrypted, wrongIV, key)
	if err != nil {
		t.Fatalf("Decryption with wrong IV failed: %v", err)
	}

	// Should produce garbage
	if bytes.Equal(decrypted, plaintext) {
		t.Error("Decryption with wrong IV produced correct plaintext (should be garbage)")
	}

	t.Log("Decryption with wrong IV produces garbage as expected ✓")
}

func TestEncryptWithInvalidKeySize(t *testing.T) {
	plaintext := []byte("Test message")
	invalidKey := []byte{0x01, 0x02, 0x03} // Too short

	_, err := Encrypt(plaintext, invalidKey)
	if err == nil {
		t.Error("Encrypt should fail with invalid key size, but succeeded")
	}

	t.Logf("Encrypt correctly rejected invalid key size: %v ✓", err)
}

func TestDecryptWithInvalidKeySize(t *testing.T) {
	encrypted := []byte("fake encrypted data")
	iv := make([]byte, aes.BlockSize)
	invalidKey := []byte{0x01, 0x02, 0x03} // Too short

	_, err := Decrypt(encrypted, iv, invalidKey)
	if err == nil {
		t.Error("Decrypt should fail with invalid key size, but succeeded")
	}

	t.Logf("Decrypt correctly rejected invalid key size: %v ✓", err)
}
