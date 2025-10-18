package main

import (
	"testing"
)

func TestHashIdentifierSHA3_Consistency(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "device token",
			input: "test-device-token-123",
		},
		{
			name:  "user id",
			input: "507f1f77bcf86cd799439011",
		},
		{
			name:  "device key",
			input: "my-secure-device-key-456",
		},
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "special characters",
			input: "!@#$%^&*()_+-={}[]|:;<>,.?/~`",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Hash the same value twice
			hash1 := HashIdentifier(tc.input)
			hash2 := HashIdentifier(tc.input)

			// Verify they are identical as strings
			if hash1 != hash2 {
				t.Errorf("Hashes do not match!\nFirst hash:  %s\nSecond hash: %s", hash1, hash2)
			}

			// Verify the hash is not empty (unless input is empty)
			if tc.input != "" && hash1 == "" {
				t.Errorf("Hash should not be empty for non-empty input")
			}

			// Verify the hash length (SHA3-256 produces 64 hex characters)
			expectedLength := 64
			if len(hash1) != expectedLength {
				t.Errorf("Hash length is incorrect. Expected %d, got %d", expectedLength, len(hash1))
			}

			t.Logf("Input: %q\nHash:  %s", tc.input, hash1)
		})
	}
}

func TestHashIdentifierSHA3_Uniqueness(t *testing.T) {
	// Test that different inputs produce different hashes
	input1 := "device-token-1"
	input2 := "device-token-2"

	hash1 := HashIdentifier(input1)
	hash2 := HashIdentifier(input2)

	if hash1 == hash2 {
		t.Errorf("Different inputs produced the same hash!\nInput1: %s\nInput2: %s\nHash: %s",
			input1, input2, hash1)
	}

	t.Logf("Input1: %q -> Hash: %s", input1, hash1)
	t.Logf("Input2: %q -> Hash: %s", input2, hash2)
}

func TestHashIdentifierSHA3_KnownValue(t *testing.T) {
	// Test against a known SHA3-256 value
	input := "test-device-token-123"
	expectedHash := "4d3d1ccc7011a8d1fa523c9e2bd91c188fbe657f77c8bc3f36981e11ca011e04"

	hash := HashIdentifier(input)

	if hash != expectedHash {
		t.Errorf("Hash does not match expected value!\nExpected: %s\nGot:      %s", expectedHash, hash)
	}

	t.Logf("Input: %q\nHash:  %s\nMatch: ✓", input, hash)
}
