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

			encrypted, err := Encrypt(tc.plaintext, tc.password)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			if len(encrypted) == 0 {
				t.Error("Encrypted data is empty")
			}

			if len(encrypted) < len(tc.plaintext)+28 {
				t.Errorf("Encrypted data seems too short: got %d bytes for %d byte plaintext",
					len(encrypted), len(tc.plaintext))
			}

			decrypted, err := Decrypt(encrypted, tc.password)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

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

	encrypted1, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("First encryption failed: %v", err)
	}

	encrypted2, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Second encryption failed: %v", err)
	}

	if string(encrypted1) == string(encrypted2) {
		t.Error("Two encryptions of the same plaintext produced identical ciphertext (salt/nonce should be random)")
	}

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

	encrypted, err := Encrypt(plaintext, correctPassword)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	_, err = Decrypt(encrypted, wrongPassword)
	if err == nil {
		t.Error("Decryption with wrong password should fail, but succeeded")
	}

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

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	if len(encrypted) > 30 {
		encrypted[30] ^= 0xFF
	}

	_, err = Decrypt(encrypted, password)
	if err == nil {
		t.Error("Decrypt should fail with corrupted data, but succeeded")
	}

	t.Logf("Decrypt correctly detected corrupted data: %v ✓", err)
}

func TestEncryptDecryptWithDifferentPasswordLengths(t *testing.T) {
	plaintext := "Test message for different password lengths"

	passwords := [][]byte{
		[]byte("a"),
		[]byte("short"),
		[]byte("medium-password"),
		[]byte("a-longer-password-for-test"),
		[]byte(strings.Repeat("x", 100)),
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
