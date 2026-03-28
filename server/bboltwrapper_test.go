package main

import (
	"testing"

	"github.com/google/uuid"
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

func Test_uuidToString(t *testing.T) {
	id := uuid.New()

	tests := []struct {
		name     string
		input    uuid.UUID
		expected string
	}{
		{
			name:     "standard UUID",
			input:    id,
			expected: id.String(),
		},
		{
			name:     "nil UUID",
			input:    uuid.Nil,
			expected: "00000000-0000-0000-0000-000000000000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.input.String()
			if result != tc.expected {
				t.Errorf("uuid.UUID.String() = %q, expected %q", result, tc.expected)
			}
			t.Logf("uuid.UUID.String() = %q ✓", result)
		})
	}
}

func Test_uuidSliceToString(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	tests := []struct {
		name     string
		input    []uuid.UUID
		expected []string
	}{
		{
			name:     "three UUIDs",
			input:    []uuid.UUID{id1, id2, id3},
			expected: []string{id1.String(), id2.String(), id3.String()},
		},
		{
			name:     "empty slice",
			input:    []uuid.UUID{},
			expected: []string{},
		},
		{
			name:     "single UUID",
			input:    []uuid.UUID{id1},
			expected: []string{id1.String()},
		},
		{
			name:     "nil UUID in slice",
			input:    []uuid.UUID{uuid.Nil},
			expected: []string{"00000000-0000-0000-0000-000000000000"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := uuidSliceToString(tc.input)

			if len(result) != len(tc.expected) {
				t.Errorf("uuidSliceToString length mismatch: got %d, expected %d", len(result), len(tc.expected))
			}

			for i := range result {
				if i >= len(tc.expected) || result[i] != tc.expected[i] {
					t.Errorf("uuidSliceToString mismatch at index %d: got %q, expected %q",
						i, result[i], tc.expected[i])
				}
			}

			t.Logf("uuidSliceToString(%d UUIDs) -> %d strings ✓", len(tc.input), len(result))
		})
	}
}

func Test_stringSliceToUUID(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	tests := []struct {
		name          string
		input         []string
		expectedCount int
		validate      func([]uuid.UUID) bool
	}{
		{
			name:          "valid UUID strings",
			input:         []string{id1.String(), id2.String(), id3.String()},
			expectedCount: 3,
			validate: func(result []uuid.UUID) bool {
				return result[0] == id1 && result[1] == id2 && result[2] == id3
			},
		},
		{
			name:          "single valid UUID string",
			input:         []string{id1.String()},
			expectedCount: 1,
			validate: func(result []uuid.UUID) bool {
				return result[0] == id1
			},
		},
		{
			name:          "invalid string - skipped",
			input:         []string{"not-a-uuid"},
			expectedCount: 0,
			validate:      func(result []uuid.UUID) bool { return len(result) == 0 },
		},
		{
			name:          "mixed valid and invalid",
			input:         []string{id1.String(), "invalid", id2.String()},
			expectedCount: 2,
			validate: func(result []uuid.UUID) bool {
				return len(result) == 2 && result[0] == id1 && result[1] == id2
			},
		},
		{
			name:          "empty slice",
			input:         []string{},
			expectedCount: 0,
			validate:      func(result []uuid.UUID) bool { return len(result) == 0 },
		},
		{
			name:          "empty strings - skipped",
			input:         []string{"", "", ""},
			expectedCount: 0,
			validate:      func(result []uuid.UUID) bool { return len(result) == 0 },
		},
		{
			name:          "short hex string - skipped",
			input:         []string{"abc123"},
			expectedCount: 0,
			validate:      func(result []uuid.UUID) bool { return len(result) == 0 },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stringSliceToUUID(tc.input)

			if len(result) != tc.expectedCount {
				t.Errorf("stringSliceToUUID length mismatch: got %d, expected %d", len(result), tc.expectedCount)
			}

			if tc.validate != nil && !tc.validate(result) {
				t.Errorf("stringSliceToUUID validation failed for input %v", tc.input)
			}

			t.Logf("stringSliceToUUID(%d strings) -> %d UUIDs ✓", len(tc.input), len(result))
		})
	}
}

func Test_stringSliceToUUID_Formats(t *testing.T) {
	validUUID := uuid.New().String()

	tests := []struct {
		name          string
		input         string
		shouldConvert bool
	}{
		{
			name:          "valid UUID with hyphens",
			input:         validUUID,
			shouldConvert: true,
		},
		{
			name:          "valid UUID uppercase",
			input:         "550E8400-E29B-41D4-A716-446655440000",
			shouldConvert: true,
		},
		{
			name:          "valid UUID lowercase",
			input:         "550e8400-e29b-41d4-a716-446655440000",
			shouldConvert: true,
		},
		{
			name:          "valid - missing hyphens (raw hex accepted by uuid.Parse)",
			input:         "550e8400e29b41d4a716446655440000",
			shouldConvert: true,
		},
		{
			name:          "invalid - too short",
			input:         "550e8400-e29b-41d4-a716",
			shouldConvert: false,
		},
		{
			name:          "invalid - too long",
			input:         "550e8400-e29b-41d4-a716-4466554400001",
			shouldConvert: false,
		},
		{
			name:          "invalid - random string",
			input:         "not-a-uuid-at-all",
			shouldConvert: false,
		},
		{
			name:          "invalid - empty string",
			input:         "",
			shouldConvert: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stringSliceToUUID([]string{tc.input})

			if tc.shouldConvert {
				if len(result) != 1 {
					t.Errorf("Expected successful conversion for %q, got %d results", tc.input, len(result))
				}
			} else {
				if len(result) != 0 {
					t.Errorf("Expected failed conversion for %q, but got successful conversion", tc.input)
				}
			}

			t.Logf("UUID format %q: shouldConvert=%v, got %d results ✓", tc.input, tc.shouldConvert, len(result))
		})
	}
}
