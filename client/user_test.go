package client

import (
	"bytes"
	"crypto/aes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {

	testCases := []struct {
		name      string
		plaintext []byte
		keySize   int
	}{
		{
			name:      "empty string",
			plaintext: []byte(""),
			keySize:   32,
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

			key := make([]byte, tc.keySize)
			for i := range key {
				key[i] = byte(i % 256)
			}

			ciphertext, err := Encrypt(tc.plaintext, key)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			if len(ciphertext) < aes.BlockSize {
				t.Errorf("Ciphertext too short: got %d bytes, expected at least %d", len(ciphertext), aes.BlockSize)
			}

			iv := ciphertext[:aes.BlockSize]
			encryptedData := ciphertext[aes.BlockSize:]

			decrypted, err := Decrypt(encryptedData, iv, key)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

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

	ciphertext1, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	ciphertext2, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Two encryptions of the same plaintext produced identical ciphertext (IV should be random)")
	}

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
		wrongKey[i] = byte(255 - i)
	}

	ciphertext, err := Encrypt(plaintext, correctKey)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	iv := ciphertext[:aes.BlockSize]
	encrypted := ciphertext[aes.BlockSize:]
	decrypted, err := Decrypt(encrypted, iv, wrongKey)
	if err != nil {
		t.Fatalf("Decryption with wrong key failed: %v", err)
	}

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

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	wrongIV := make([]byte, aes.BlockSize)
	for i := range wrongIV {
		wrongIV[i] = 0xFF
	}
	encrypted := ciphertext[aes.BlockSize:]

	decrypted, err := Decrypt(encrypted, wrongIV, key)
	if err != nil {
		t.Fatalf("Decryption with wrong IV failed: %v", err)
	}

	if bytes.Equal(decrypted, plaintext) {
		t.Error("Decryption with wrong IV produced correct plaintext (should be garbage)")
	}

	t.Log("Decryption with wrong IV produces garbage as expected ✓")
}

func TestEncryptWithInvalidKeySize(t *testing.T) {
	plaintext := []byte("Test message")
	invalidKey := []byte{0x01, 0x02, 0x03}

	_, err := Encrypt(plaintext, invalidKey)
	if err == nil {
		t.Error("Encrypt should fail with invalid key size, but succeeded")
	}

	t.Logf("Encrypt correctly rejected invalid key size: %v ✓", err)
}

func TestDecryptWithInvalidKeySize(t *testing.T) {
	encrypted := []byte("fake encrypted data")
	iv := make([]byte, aes.BlockSize)
	invalidKey := []byte{0x01, 0x02, 0x03}

	_, err := Decrypt(encrypted, iv, invalidKey)
	if err == nil {
		t.Error("Decrypt should fail with invalid key size, but succeeded")
	}

	t.Logf("Decrypt correctly rejected invalid key size: %v ✓", err)
}
