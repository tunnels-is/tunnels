package ui

import (
	"os"
	"runtime"
	"testing"
)

func withEnv(t *testing.T, key, value string, unset bool) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
	if unset {
		_ = os.Unsetenv(key)
		return
	}
	_ = os.Setenv(key, value)
}

func TestHardenFyneSetsDPIEnvWhenUnset(t *testing.T) {
	withEnv(t, fyneDisableDPIEnv, "", true)
	withEnv(t, fynePlatformEnv, "", true)
	withEnv(t, "WAYLAND_DISPLAY", "", true)

	hardenFyne()
	got := os.Getenv(fyneDisableDPIEnv)
	if runtime.GOOS == "darwin" {
		if got != "1" {
			t.Fatalf("FYNE_DISABLE_DPI_DETECTION: got %q, want 1 on darwin", got)
		}
		return
	}
	if got != "" {
		t.Fatalf("FYNE_DISABLE_DPI_DETECTION must stay unset on %s, got %q", runtime.GOOS, got)
	}
}

func TestHardenFyneLeavesExistingDPIEnv(t *testing.T) {
	withEnv(t, fyneDisableDPIEnv, "0", false)
	withEnv(t, fynePlatformEnv, "", true)
	withEnv(t, "WAYLAND_DISPLAY", "", true)

	hardenFyne()
	if got := os.Getenv(fyneDisableDPIEnv); got != "0" {
		t.Fatalf("existing value must be kept, got %q", got)
	}
}

func TestHardenFynePrefersWaylandWhenSessionIsWayland(t *testing.T) {
	withEnv(t, fynePlatformEnv, "", true)
	withEnv(t, "WAYLAND_DISPLAY", "wayland-0", false)

	hardenFyne()
	got := os.Getenv(fynePlatformEnv)
	if runtime.GOOS != "linux" {
		if got != "" {
			t.Fatalf("FYNE_PLATFORM must stay unset on %s, got %q", runtime.GOOS, got)
		}
		return
	}
	if got != "wayland" {
		t.Fatalf("FYNE_PLATFORM: got %q, want wayland", got)
	}
}

func TestHardenFyneLeavesExistingPlatform(t *testing.T) {
	withEnv(t, fynePlatformEnv, "x11", false)
	withEnv(t, "WAYLAND_DISPLAY", "wayland-0", false)

	hardenFyne()
	if got := os.Getenv(fynePlatformEnv); got != "x11" {
		t.Fatalf("existing FYNE_PLATFORM must be kept, got %q", got)
	}
}

func TestHardenFyneSkipsWaylandWithoutSession(t *testing.T) {
	withEnv(t, fynePlatformEnv, "", true)
	withEnv(t, "WAYLAND_DISPLAY", "", true)

	hardenFyne()
	if got := os.Getenv(fynePlatformEnv); got != "" {
		t.Fatalf("FYNE_PLATFORM must stay unset without WAYLAND_DISPLAY, got %q", got)
	}
}
