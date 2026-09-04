package ui

import "testing"

func TestFormatWindowTitleDisconnected(t *testing.T) {
	got := formatWindowTitle(false, 0, 0, nil)
	if got != "disconnected" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatWindowTitleConnectedRateAndNames(t *testing.T) {
	got := formatWindowTitle(true, 12_000, 3_000, []string{"home", "work"})
	want := "12 KB/s - 3 KB/s - home - work"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatWindowTitleConnectedNoNames(t *testing.T) {
	got := formatWindowTitle(true, 0, 0, nil)
	want := "0 B/s - 0 B/s"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
