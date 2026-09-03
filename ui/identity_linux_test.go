//go:build linux

package ui

import (
	"strings"
	"testing"
)

func TestLinuxDesktopEntryIdentifiesApp(t *testing.T) {
	got := linuxDesktopEntry("/opt/tunnels/tunnels-app")
	for _, want := range []string{
		"Name=Tunnels",
		`Exec="/opt/tunnels/tunnels-app"`,
		"Icon=" + linuxAppID,
		"StartupWMClass=" + linuxAppID,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("desktop entry missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "StartupWMClass=Tunnels\n") {
		t.Fatal("StartupWMClass must be the Wayland app_id, not the window title")
	}
}

func TestDesktopQuoteEscapes(t *testing.T) {
	got := desktopQuote(`/tmp/foo"bar`)
	if got != `"/tmp/foo\"bar"` {
		t.Fatalf("got %s", got)
	}
}
