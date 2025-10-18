package main

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Test_contains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		search   string
		expected bool
	}{
		{
			name:     "found at beginning",
			slice:    []string{"apple", "banana", "cherry"},
			search:   "apple",
			expected: true,
		},
		{
			name:     "found in middle",
			slice:    []string{"apple", "banana", "cherry"},
			search:   "banana",
			expected: true,
		},
		{
			name:     "found at end",
			slice:    []string{"apple", "banana", "cherry"},
			search:   "cherry",
			expected: true,
		},
		{
			name:     "not found",
			slice:    []string{"apple", "banana", "cherry"},
			search:   "orange",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			search:   "apple",
			expected: false,
		},
		{
			name:     "single element - found",
			slice:    []string{"apple"},
			search:   "apple",
			expected: true,
		},
		{
			name:     "single element - not found",
			slice:    []string{"apple"},
			search:   "banana",
			expected: false,
		},
		{
			name:     "empty string search - found",
			slice:    []string{"", "apple", "banana"},
			search:   "",
			expected: true,
		},
		{
			name:     "empty string search - not found",
			slice:    []string{"apple", "banana"},
			search:   "",
			expected: false,
		},
		{
			name:     "case sensitive",
			slice:    []string{"Apple", "Banana", "Cherry"},
			search:   "apple",
			expected: false,
		},
		{
			name:     "duplicate values",
			slice:    []string{"apple", "apple", "banana"},
			search:   "apple",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := contains(tc.slice, tc.search)
			if result != tc.expected {
				t.Errorf("contains(%v, %q) = %v, expected %v", tc.slice, tc.search, result, tc.expected)
			}
			t.Logf("contains(%v, %q) = %v ✓", tc.slice, tc.search, result)
		})
	}
}

func Test_removeString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		remove   string
		expected []string
	}{
		{
			name:     "remove from beginning",
			slice:    []string{"apple", "banana", "cherry"},
			remove:   "apple",
			expected: []string{"banana", "cherry"},
		},
		{
			name:     "remove from middle",
			slice:    []string{"apple", "banana", "cherry"},
			remove:   "banana",
			expected: []string{"apple", "cherry"},
		},
		{
			name:     "remove from end",
			slice:    []string{"apple", "banana", "cherry"},
			remove:   "cherry",
			expected: []string{"apple", "banana"},
		},
		{
			name:     "remove non-existent",
			slice:    []string{"apple", "banana", "cherry"},
			remove:   "orange",
			expected: []string{"apple", "banana", "cherry"},
		},
		{
			name:     "remove from single element",
			slice:    []string{"apple"},
			remove:   "apple",
			expected: []string{},
		},
		{
			name:     "remove from empty slice",
			slice:    []string{},
			remove:   "apple",
			expected: []string{},
		},
		{
			name:     "remove duplicate values",
			slice:    []string{"apple", "apple", "banana", "apple"},
			remove:   "apple",
			expected: []string{"banana"},
		},
		{
			name:     "remove empty string",
			slice:    []string{"", "apple", "banana", ""},
			remove:   "",
			expected: []string{"apple", "banana"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := removeString(tc.slice, tc.remove)

			if len(result) != len(tc.expected) {
				t.Errorf("removeString length mismatch: got %d, expected %d", len(result), len(tc.expected))
			}

			for i := range result {
				if i >= len(tc.expected) || result[i] != tc.expected[i] {
					t.Errorf("removeString(%v, %q) = %v, expected %v", tc.slice, tc.remove, result, tc.expected)
					break
				}
			}

			t.Logf("removeString(%v, %q) = %v ✓", tc.slice, tc.remove, result)
		})
	}
}

func Test_objectIDToString(t *testing.T) {
	// Create a test ObjectID
	testOID := primitive.NewObjectID()

	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "ObjectID type",
			input:    testOID,
			expected: testOID.Hex(),
		},
		{
			name:     "string type",
			input:    "already-a-string",
			expected: "already-a-string",
		},
		{
			name:     "byte array [12]byte",
			input:    testOID,
			expected: testOID.Hex(),
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "number falls back to fmt.Sprintf",
			input:    12345,
			expected: "12345",
		},
		{
			name:     "nil value",
			input:    nil,
			expected: "<nil>",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := objectIDToString(tc.input)
			if result != tc.expected {
				t.Errorf("objectIDToString(%v) = %q, expected %q", tc.input, result, tc.expected)
			}
			t.Logf("objectIDToString(%T) = %q ✓", tc.input, result)
		})
	}
}

func Test_objectIDSliceToString(t *testing.T) {
	// Create test ObjectIDs
	oid1 := primitive.NewObjectID()
	oid2 := primitive.NewObjectID()
	oid3 := primitive.NewObjectID()

	tests := []struct {
		name     string
		input    interface{}
		expected []string
	}{
		{
			name:     "ObjectID slice",
			input:    []primitive.ObjectID{oid1, oid2, oid3},
			expected: []string{oid1.Hex(), oid2.Hex(), oid3.Hex()},
		},
		{
			name:     "string slice - passthrough",
			input:    []string{"str1", "str2", "str3"},
			expected: []string{"str1", "str2", "str3"},
		},
		{
			name:     "empty ObjectID slice",
			input:    []primitive.ObjectID{},
			expected: nil,
		},
		{
			name:     "empty string slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single ObjectID",
			input:    []primitive.ObjectID{oid1},
			expected: []string{oid1.Hex()},
		},
		{
			name:     "single string",
			input:    []string{"single"},
			expected: []string{"single"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := objectIDSliceToString(tc.input)

			if len(result) != len(tc.expected) {
				t.Errorf("objectIDSliceToString length mismatch: got %d, expected %d", len(result), len(tc.expected))
			}

			for i := range result {
				if i >= len(tc.expected) || result[i] != tc.expected[i] {
					t.Errorf("objectIDSliceToString(%T) mismatch at index %d: got %q, expected %q",
						tc.input, i, result[i], tc.expected[i])
				}
			}

			t.Logf("objectIDSliceToString(%T) -> %d strings ✓", tc.input, len(result))
		})
	}
}

func Test_stringSliceToObjectID(t *testing.T) {
	// Create valid hex strings
	oid1 := primitive.NewObjectID()
	oid2 := primitive.NewObjectID()
	oid3 := primitive.NewObjectID()

	tests := []struct {
		name          string
		input         []string
		expectedCount int
		validate      func([]primitive.ObjectID) bool
	}{
		{
			name:          "valid hex strings",
			input:         []string{oid1.Hex(), oid2.Hex(), oid3.Hex()},
			expectedCount: 3,
			validate: func(result []primitive.ObjectID) bool {
				return result[0] == oid1 && result[1] == oid2 && result[2] == oid3
			},
		},
		{
			name:          "single valid hex string",
			input:         []string{oid1.Hex()},
			expectedCount: 1,
			validate: func(result []primitive.ObjectID) bool {
				return result[0] == oid1
			},
		},
		{
			name:          "invalid hex string - skipped",
			input:         []string{"invalid-hex"},
			expectedCount: 0,
			validate:      func(result []primitive.ObjectID) bool { return len(result) == 0 },
		},
		{
			name:          "mixed valid and invalid",
			input:         []string{oid1.Hex(), "invalid", oid2.Hex()},
			expectedCount: 2,
			validate: func(result []primitive.ObjectID) bool {
				return len(result) == 2 && result[0] == oid1 && result[1] == oid2
			},
		},
		{
			name:          "empty slice",
			input:         []string{},
			expectedCount: 0,
			validate:      func(result []primitive.ObjectID) bool { return len(result) == 0 },
		},
		{
			name:          "empty strings - skipped",
			input:         []string{"", "", ""},
			expectedCount: 0,
			validate:      func(result []primitive.ObjectID) bool { return len(result) == 0 },
		},
		{
			name:          "wrong length hex string - skipped",
			input:         []string{"abc123"},
			expectedCount: 0,
			validate:      func(result []primitive.ObjectID) bool { return len(result) == 0 },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stringSliceToObjectID(tc.input)

			if len(result) != tc.expectedCount {
				t.Errorf("stringSliceToObjectID length mismatch: got %d, expected %d", len(result), tc.expectedCount)
			}

			if tc.validate != nil && !tc.validate(result) {
				t.Errorf("stringSliceToObjectID validation failed for input %v", tc.input)
			}

			t.Logf("stringSliceToObjectID(%d strings) -> %d ObjectIDs ✓", len(tc.input), len(result))
		})
	}
}

func Test_stringSliceToObjectID_HexFormats(t *testing.T) {
	// Test different hex string formats
	oid := primitive.NewObjectID()
	validHex := oid.Hex()

	tests := []struct {
		name          string
		input         string
		shouldConvert bool
	}{
		{
			name:          "valid 24 char hex",
			input:         validHex,
			shouldConvert: true,
		},
		{
			name:          "uppercase hex",
			input:         "507F1F77BCFB6A8D69098765",
			shouldConvert: true,
		},
		{
			name:          "lowercase hex",
			input:         "507f1f77bcfb6a8d69098765",
			shouldConvert: true,
		},
		{
			name:          "mixed case hex",
			input:         "507F1f77BcFb6A8d69098765",
			shouldConvert: true,
		},
		{
			name:          "invalid char 'G'",
			input:         "507G1F77BCFB6A8D69098765",
			shouldConvert: false,
		},
		{
			name:          "too short",
			input:         "507F1F77BCFB6A8D6909876",
			shouldConvert: false,
		},
		{
			name:          "too long",
			input:         "507F1F77BCFB6A8D690987650",
			shouldConvert: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stringSliceToObjectID([]string{tc.input})

			if tc.shouldConvert {
				if len(result) != 1 {
					t.Errorf("Expected successful conversion for %q, got %d results", tc.input, len(result))
				}
			} else {
				if len(result) != 0 {
					t.Errorf("Expected failed conversion for %q, but got successful conversion", tc.input)
				}
			}

			t.Logf("Hex format %q: shouldConvert=%v, got %d results ✓", tc.input, tc.shouldConvert, len(result))
		})
	}
}
