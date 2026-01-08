package main

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
		password  []byte
	}{
		{
			name:      "short message with simple password",
			plaintext: "Hello, World!",
			password:  []byte("password123"),
		},
		{
			name:      "longer message",
			plaintext: "This is a longer test message with more characters to encrypt and decrypt properly.",
			password:  []byte("strong-password-with-special-chars!@#"),
		},
		{
			name:      "unicode characters",
			plaintext: "Hello 世界! 🌍🔒 Testing encryption",
			password:  []byte("unicode-password-测试"),
		},
		{
			name:      "special characters",
			plaintext: `!@#$%^&*()_+-=[]{}|;:'",.<>?/~` + "`",
			password:  []byte("special!@#$%"),
		},
		{
			name:      "newlines and whitespace",
			plaintext: "Line 1\nLine 2\n\tTabbed\n  Spaces",
			password:  []byte("whitespace-pass"),
		},
		{
			name:      "json-like data",
			plaintext: `{"key":"value","number":123,"nested":{"inner":"data"}}`,
			password:  []byte("json-password"),
		},
		{
			name:      "very long password",
			plaintext: "Test message",
			password:  []byte(strings.Repeat("long-password-", 10)),
		},
		{
			name:      "large message",
			plaintext: strings.Repeat("Large message content. ", 100),
			password:  []byte("large-msg-pass"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			encrypted, err := Encrypt(tc.plaintext, tc.password)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Verify encrypted data is not empty
			if len(encrypted) == 0 {
				t.Error("Encrypted data is empty")
			}

			// Verify encrypted data is longer than plaintext (includes salt + nonce + auth tag)
			if len(encrypted) < len(tc.plaintext)+28 { // salt(16) + nonce(12) minimum
				t.Errorf("Encrypted data seems too short: got %d bytes for %d byte plaintext",
					len(encrypted), len(tc.plaintext))
			}

			// Decrypt
			decrypted, err := Decrypt(encrypted, tc.password)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			// Verify round-trip
			if decrypted != tc.plaintext {
				t.Errorf("Decrypted data does not match original\nOriginal:  %q\nDecrypted: %q",
					tc.plaintext, decrypted)
			}

			t.Logf("Successfully encrypted and decrypted %d bytes ✓", len(tc.plaintext))
		})
	}
}

func TestEncryptProducesUniqueOutputs(t *testing.T) {
	plaintext := "The same plaintext encrypted twice"
	password := []byte("test-password")

	// Encrypt same data twice
	encrypted1, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	encrypted2, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	// The ciphertexts should be different due to random salt and nonce
	if string(encrypted1) == string(encrypted2) {
		t.Error("Two encryptions of the same plaintext produced identical ciphertext (salt/nonce should be random)")
	}

	// But both should decrypt to the same plaintext
	decrypted1, err := Decrypt(encrypted1, password)
	if err != nil {
		t.Fatalf("First decryption failed: %v", err)
	}

	decrypted2, err := Decrypt(encrypted2, password)
	if err != nil {
		t.Fatalf("Second decryption failed: %v", err)
	}

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Decrypted data does not match original plaintext")
	}

	t.Log("Encrypt produces unique outputs with random salt/nonce ✓")
}

func TestDecryptWithWrongPassword(t *testing.T) {
	plaintext := "Secret message"
	correctPassword := []byte("correct-password")
	wrongPassword := []byte("wrong-password")

	// Encrypt with correct password
	encrypted, err := Encrypt(plaintext, correctPassword)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Try to decrypt with wrong password
	_, err = Decrypt(encrypted, wrongPassword)
	if err == nil {
		t.Error("Decryption with wrong password should fail, but succeeded")
	}

	// Error message should mention authentication or password
	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "decrypt") && !strings.Contains(errMsg, "password") &&
		!strings.Contains(errMsg, "integrity") {
		t.Logf("Error message: %v", err)
	}

	t.Log("Decryption with wrong password correctly failed ✓")
}

func TestEncryptWithEmptyPassword(t *testing.T) {
	plaintext := "Test message"
	emptyPassword := []byte{}

	_, err := Encrypt(plaintext, emptyPassword)
	if err == nil {
		t.Error("Encrypt should fail with empty password, but succeeded")
	}

	if !strings.Contains(err.Error(), "password") {
		t.Errorf("Error message should mention password: %v", err)
	}

	t.Logf("Encrypt correctly rejected empty password: %v ✓", err)
}

func TestEncryptWithEmptyPlaintext(t *testing.T) {
	plaintext := ""
	password := []byte("test-password")

	_, err := Encrypt(plaintext, password)
	if err == nil {
		t.Error("Encrypt should fail with empty plaintext, but succeeded")
	}

	if !strings.Contains(err.Error(), "plaintext") {
		t.Errorf("Error message should mention plaintext: %v", err)
	}

	t.Logf("Encrypt correctly rejected empty plaintext: %v ✓", err)
}

func TestDecryptWithEmptyPassword(t *testing.T) {
	// Create some fake encrypted data
	fakeEncrypted := make([]byte, 100)
	emptyPassword := []byte{}

	_, err := Decrypt(fakeEncrypted, emptyPassword)
	if err == nil {
		t.Error("Decrypt should fail with empty password, but succeeded")
	}

	if !strings.Contains(err.Error(), "password") {
		t.Errorf("Error message should mention password: %v", err)
	}

	t.Logf("Decrypt correctly rejected empty password: %v ✓", err)
}

func TestDecryptWithTooShortData(t *testing.T) {
	// Data shorter than salt + nonce (16 + 12 = 28 bytes)
	shortData := make([]byte, 20)
	password := []byte("test-password")

	_, err := Decrypt(shortData, password)
	if err == nil {
		t.Error("Decrypt should fail with too short data, but succeeded")
	}

	if !strings.Contains(err.Error(), "short") && !strings.Contains(err.Error(), "invalid") {
		t.Errorf("Error message should mention short/invalid data: %v", err)
	}

	t.Logf("Decrypt correctly rejected too short data: %v ✓", err)
}

func TestDecryptWithCorruptedData(t *testing.T) {
	plaintext := "Test message"
	password := []byte("test-password")

	// Encrypt
	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	// Corrupt the data by flipping some bits in the ciphertext portion
	// (after salt(16) and nonce(12))
	if len(encrypted) > 30 {
		encrypted[30] ^= 0xFF // Flip bits
	}

	// Try to decrypt corrupted data
	_, err = Decrypt(encrypted, password)
	if err == nil {
		t.Error("Decrypt should fail with corrupted data, but succeeded")
	}

	t.Logf("Decrypt correctly detected corrupted data: %v ✓", err)
}

func TestEncryptDecryptWithDifferentPasswordLengths(t *testing.T) {
	plaintext := "Test message for different password lengths"

	passwords := [][]byte{
		[]byte("a"),                          // 1 byte
		[]byte("short"),                      // 5 bytes
		[]byte("medium-password"),            // 15 bytes
		[]byte("a-longer-password-for-test"), // 27 bytes
		[]byte(strings.Repeat("x", 100)),     // 100 bytes
	}

	for _, password := range passwords {
		t.Run(string(password[:min(len(password), 10)]), func(t *testing.T) {
			encrypted, err := Encrypt(plaintext, password)
			if err != nil {
				t.Fatalf("Encrypt failed with %d byte password: %v", len(password), err)
			}

			decrypted, err := Decrypt(encrypted, password)
			if err != nil {
				t.Fatalf("Decrypt failed with %d byte password: %v", len(password), err)
			}

			if decrypted != plaintext {
				t.Errorf("Round-trip failed with %d byte password", len(password))
			}

			t.Logf("Password length %d works correctly ✓", len(password))
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
