package ui

import "testing"

// TestParseMTUText ...
func TestParseMTUText(t *testing.T) {
	if got := parseMTUText("", 1420); got != 0 {
		t.Fatalf("empty = %d, want 0", got)
	}
	if got := parseMTUText("  0 ", 1420); got != 0 {
		t.Fatalf("zero = %d", got)
	}
	if got := parseMTUText("1440", 1420); got != 1440 {
		t.Fatalf("set = %d", got)
	}
	if got := parseMTUText("nope", 1420); got != 1420 {
		t.Fatalf("bad text = %d", got)
	}
	if got := parseMTUText("-1", 1420); got != 1420 {
		t.Fatalf("negative = %d", got)
	}
}
