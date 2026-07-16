package client

import (
	"net/http"
	"testing"
)

// C1: only loopback Host headers may reach the local API (anti-DNS-rebinding).
func TestIsLocalRequest(t *testing.T) {
	// Default (loopback) API bind: only loopback/localhost/wails Hosts pass.
	prev := CONFIG.Load()
	t.Cleanup(func() { CONFIG.Store(prev) })
	CONFIG.Store(&configV2{APIIP: "127.0.0.1", APIPort: "7777"})

	cases := []struct {
		host string
		want bool
	}{
		{"127.0.0.1:7777", true},
		{"127.0.0.1", true},
		{"localhost:7777", true},
		{"localhost", true},
		{"[::1]:7777", true},
		{"wails.localhost", true},
		{"evil.com:7777", false},
		{"evil.com", false},
		{"192.168.1.10:7777", false},
		{"8.8.8.8", false},
		{"", false},
	}
	for _, c := range cases {
		r := &http.Request{Host: c.host}
		if got := isLocalRequest(r); got != c.want {
			t.Errorf("isLocalRequest(Host=%q) = %v, want %v", c.host, got, c.want)
		}
	}

	// A deliberate concrete non-loopback bind allows exactly that host (P1 fix),
	// but still rejects a rebound attacker Host.
	CONFIG.Store(&configV2{APIIP: "192.168.1.10", APIPort: "7777"})
	if !isLocalRequest(&http.Request{Host: "192.168.1.10:7777"}) {
		t.Error("configured bind IP should be accepted")
	}
	if isLocalRequest(&http.Request{Host: "evil.com:7777"}) {
		t.Error("attacker Host must still be rejected under a concrete bind")
	}

	// A wildcard bind means remote access was opted into — Host check can't help.
	CONFIG.Store(&configV2{APIIP: "0.0.0.0", APIPort: "7777"})
	if !isLocalRequest(&http.Request{Host: "anything.example:7777"}) {
		t.Error("wildcard bind should accept any Host")
	}
}

// C5: tunnel tags must be safe filename components (no traversal).
func TestSafeTunnelTag(t *testing.T) {
	good := []string{"tunnels", "tunnel-123", "my_tun", "A1"}
	bad := []string{"", "../etc/passwd", "a/b", "a\\b", "..", "with space", "dot.name"}
	for _, g := range good {
		if !safeTunnelTag(g) {
			t.Errorf("safeTunnelTag(%q) = false, want true", g)
		}
	}
	for _, b := range bad {
		if safeTunnelTag(b) {
			t.Errorf("safeTunnelTag(%q) = true, want false", b)
		}
	}
}

// C7: downgrade protection — version comparison.
func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2.3", "1.2.4", -1},
		{"1.3.0", "1.2.9", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.2", "1.2.0", 0},
		{"v1.2.0", "1.2.0", 0},
		{"1.2.0", "1.10.0", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
