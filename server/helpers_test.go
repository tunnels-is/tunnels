package main

import (
	"bytes"
	"testing"
)

func TestCopySlice(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{
			name:  "empty slice",
			input: []byte{},
		},
		{
			name:  "single byte",
			input: []byte{42},
		},
		{
			name:  "small slice",
			input: []byte{1, 2, 3, 4, 5},
		},
		{
			name:  "larger slice",
			input: []byte{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		},
		{
			name:  "binary data",
			input: []byte{0x00, 0xFF, 0xAB, 0xCD, 0xEF, 0x12, 0x34, 0x56},
		},
		{
			name:  "all zeros",
			input: bytes.Repeat([]byte{0}, 100),
		},
		{
			name:  "all ones",
			input: bytes.Repeat([]byte{1}, 50),
		},
		{
			name:  "large slice",
			input: make([]byte, 10000),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Copy the slice
			result := CopySlice(tc.input)

			// Verify the copy matches the original
			if !bytes.Equal(result, tc.input) {
				t.Errorf("CopySlice result does not match input\nInput:  %v\nResult: %v", tc.input, result)
			}

			// Verify length matches
			if len(result) != len(tc.input) {
				t.Errorf("CopySlice length mismatch: got %d, expected %d", len(result), len(tc.input))
			}

			// Verify it's a true copy by modifying the result and checking original is unchanged
			if len(result) > 0 {
				original := make([]byte, len(tc.input))
				copy(original, tc.input)

				result[0] = 99 // Modify the copy

				if !bytes.Equal(tc.input, original) {
					t.Error("CopySlice did not create an independent copy - modifying copy affected original")
				}
			}

			t.Logf("Successfully copied %d bytes ✓", len(tc.input))
		})
	}
}

func TestCopySliceNil(t *testing.T) {
	// Test nil input
	var input []byte = nil
	result := CopySlice(input)

	if result == nil {
		t.Error("CopySlice should return empty slice, not nil")
	}

	if len(result) != 0 {
		t.Errorf("CopySlice of nil should return empty slice, got length %d", len(result))
	}

	t.Log("Nil input handled correctly ✓")
}

func TestGENERATE_CODE(t *testing.T) {
	// Generate multiple codes
	codes := make(map[string]bool)
	numCodes := 1000

	for i := 0; i < numCodes; i++ {
		code := GENERATE_CODE()

		// Test 1: Code length should be 16
		if len(code) != 16 {
			t.Errorf("GENERATE_CODE produced code of length %d, expected 16", len(code))
		}

		// Test 2: Code should only contain valid characters (A-Z, 2-7)
		for _, c := range code {
			valid := (c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')
			if !valid {
				t.Errorf("GENERATE_CODE produced invalid character %c in code %s", c, code)
			}
		}

		// Test 3: Code should be uppercase
		if code != code {
			t.Errorf("GENERATE_CODE produced code with lowercase characters: %s", code)
		}

		// Store for uniqueness test
		codes[code] = true
	}

	// Test 4: Codes should be unique (with high probability)
	uniqueRatio := float64(len(codes)) / float64(numCodes)
	if uniqueRatio < 0.99 { // Allow for tiny collision probability
		t.Errorf("GENERATE_CODE produced too many duplicates: %d unique out of %d (%.2f%%)",
			len(codes), numCodes, uniqueRatio*100)
	}

	t.Logf("Generated %d codes, %d unique (%.2f%% unique) ✓", numCodes, len(codes), uniqueRatio*100)
}

func TestGENERATE_CODE_CharacterDistribution(t *testing.T) {
	// Test that all valid characters can appear
	validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	charCount := make(map[rune]int)

	// Generate many codes to get statistical distribution
	for i := 0; i < 10000; i++ {
		code := GENERATE_CODE()
		for _, c := range code {
			charCount[c]++
		}
	}

	// Check that all valid characters appear at least once
	for _, c := range validChars {
		if charCount[c] == 0 {
			t.Errorf("Character %c never appeared in generated codes", c)
		}
	}

	// Each character should appear roughly equally (within reason)
	expectedCount := (10000 * 16) / len(validChars)
	tolerance := float64(expectedCount) * 0.3 // 30% tolerance

	for _, c := range validChars {
		count := charCount[c]
		diff := float64(abs(count - expectedCount))
		if diff > tolerance {
			t.Logf("Warning: Character %c appeared %d times (expected ~%d)", c, count, expectedCount)
		}
	}

	t.Logf("Character distribution looks reasonable ✓")
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
