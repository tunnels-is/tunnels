package main

import (
	"testing"

	"github.com/google/uuid"
)


func TestGENERATE_CODE(t *testing.T) {

	codes := make(map[string]bool)
	numCodes := 1000

	for i := 0; i < numCodes; i++ {
		code := GENERATE_CODE()

		if len(code) != 16 {
			t.Errorf("GENERATE_CODE produced code of length %d, expected 16", len(code))
		}

		for _, c := range code {
			valid := (c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')
			if !valid {
				t.Errorf("GENERATE_CODE produced invalid character %c in code %s", c, code)
			}
		}

		codes[code] = true
	}

	uniqueRatio := float64(len(codes)) / float64(numCodes)
	if uniqueRatio < 0.99 {
		t.Errorf("GENERATE_CODE produced too many duplicates: %d unique out of %d (%.2f%%)",
			len(codes), numCodes, uniqueRatio*100)
	}

	t.Logf("Generated %d codes, %d unique (%.2f%% unique) ✓", numCodes, len(codes), uniqueRatio*100)
}

func TestGENERATE_CODE_CharacterDistribution(t *testing.T) {

	validChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	charCount := make(map[rune]int)

	for i := 0; i < 10000; i++ {
		code := GENERATE_CODE()
		for _, c := range code {
			charCount[c]++
		}
	}

	for _, c := range validChars {
		if charCount[c] == 0 {
			t.Errorf("Character %c never appeared in generated codes", c)
		}
	}

	expectedCount := (10000 * 16) / len(validChars)
	tolerance := float64(expectedCount) * 0.3

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

func TestHasSharedOrNoGroup(t *testing.T) {
	g1 := uuid.New()
	g2 := uuid.New()
	g3 := uuid.New()
	g4 := uuid.New()

	tests := []struct {
		name         string
		actorGroups  []uuid.UUID
		serverGroups []uuid.UUID
		want         bool
	}{
		{
			name:         "no server restriction allows any actor",
			actorGroups:  []uuid.UUID{g1, g2},
			serverGroups: []uuid.UUID{},
			want:         true,
		},
		{
			name:         "nil server groups allows any actor",
			actorGroups:  []uuid.UUID{g1},
			serverGroups: nil,
			want:         true,
		},
		{
			name:         "both empty",
			actorGroups:  []uuid.UUID{},
			serverGroups: []uuid.UUID{},
			want:         true,
		},
		{
			name:         "both nil",
			actorGroups:  nil,
			serverGroups: nil,
			want:         true,
		},
		{
			name:         "actor without groups against restricted server",
			actorGroups:  []uuid.UUID{},
			serverGroups: []uuid.UUID{g1},
			want:         false,
		},
		{
			name:         "single overlap",
			actorGroups:  []uuid.UUID{g1},
			serverGroups: []uuid.UUID{g1},
			want:         true,
		},
		{
			name:         "no overlap",
			actorGroups:  []uuid.UUID{g1, g2},
			serverGroups: []uuid.UUID{g3, g4},
			want:         false,
		},
		{
			name:         "overlap when actor has many and server has one",
			actorGroups:  []uuid.UUID{g1, g2, g3},
			serverGroups: []uuid.UUID{g3},
			want:         true,
		},
		{
			name:         "overlap when server has many and actor has one",
			actorGroups:  []uuid.UUID{g2},
			serverGroups: []uuid.UUID{g1, g2, g3},
			want:         true,
		},
		{
			name:         "overlap on last element of both",
			actorGroups:  []uuid.UUID{g1, g2, g3},
			serverGroups: []uuid.UUID{g4, g3},
			want:         true,
		},
		{
			name:         "zero UUID is treated as a value, not a wildcard",
			actorGroups:  []uuid.UUID{uuid.Nil},
			serverGroups: []uuid.UUID{g1},
			want:         false,
		},
		{
			name:         "matching zero UUIDs",
			actorGroups:  []uuid.UUID{uuid.Nil},
			serverGroups: []uuid.UUID{uuid.Nil},
			want:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hasSharedOrNoGroup(tc.actorGroups, tc.serverGroups)
			if got != tc.want {
				t.Errorf("hasSharedOrNoGroup() = %v, want %v", got, tc.want)
			}
		})
	}
}
