package client

import (
	"net/http"
	"testing"
)

func TestIsLocalRequest(t *testing.T) {

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

	CONFIG.Store(&configV2{APIIP: "192.168.1.10", APIPort: "7777"})
	if !isLocalRequest(&http.Request{Host: "192.168.1.10:7777"}) {
		t.Error("configured bind IP should be accepted")
	}
	if isLocalRequest(&http.Request{Host: "evil.com:7777"}) {
		t.Error("attacker Host must still be rejected under a concrete bind")
	}

	CONFIG.Store(&configV2{APIIP: "0.0.0.0", APIPort: "7777"})
	if !isLocalRequest(&http.Request{Host: "anything.example:7777"}) {
		t.Error("wildcard bind should accept any Host")
	}
}

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
