package ui

import (
	"os"
	"testing"
)

func TestHardenFyneSetsDPIEnvWhenUnset(t *testing.T) {
	prev, had := os.LookupEnv(fyneDisableDPIEnv)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(fyneDisableDPIEnv, prev)
		} else {
			_ = os.Unsetenv(fyneDisableDPIEnv)
		}
	})
	_ = os.Unsetenv(fyneDisableDPIEnv)

	hardenFyne()
	if got := os.Getenv(fyneDisableDPIEnv); got != "1" {
		t.Fatalf("FYNE_DISABLE_DPI_DETECTION: got %q, want 1", got)
	}
}

func TestHardenFyneLeavesExistingDPIEnv(t *testing.T) {
	prev, had := os.LookupEnv(fyneDisableDPIEnv)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(fyneDisableDPIEnv, prev)
		} else {
			_ = os.Unsetenv(fyneDisableDPIEnv)
		}
	})
	_ = os.Setenv(fyneDisableDPIEnv, "0")

	hardenFyne()
	if got := os.Getenv(fyneDisableDPIEnv); got != "0" {
		t.Fatalf("existing value must be kept, got %q", got)
	}
}
