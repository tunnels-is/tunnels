package client

import (
	"net/http"
	"os"
	"path/filepath"
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
		{"wails.localhost", false},
		{"wails", false},
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
	if isLocalRequest(&http.Request{Host: "wails.localhost"}) {
		t.Error("wails.localhost must not be treated as local")
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

func TestSafeListTag(t *testing.T) {
	// Default blocklist tags and path-traversal attempts.
	good := []string{"Ads", "AdultContent", "Malware", "my_list-1"}
	bad := []string{"", "../tunnels.conf", "..\\windows", "a/b", "dot.name", "has space"}
	for _, g := range good {
		if !safeListTag(g) {
			t.Errorf("safeListTag(%q) = false, want true", g)
		}
	}
	for _, b := range bad {
		if safeListTag(b) {
			t.Errorf("safeListTag(%q) = true, want false", b)
		}
	}
}

func TestListFilePath(t *testing.T) {
	base := filepath.Join(t.TempDir(), "blocklists") + string(os.PathSeparator)

	path, err := listFilePath(base, "Ads")
	if err != nil {
		t.Fatalf("listFilePath(Ads): %v", err)
	}
	want := filepath.Join(filepath.Clean(base), "ads")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}

	for _, tag := range []string{"../etc/passwd", "a/b", "..", "x.y"} {
		if _, err := listFilePath(base, tag); err == nil {
			t.Errorf("listFilePath(%q) should fail", tag)
		}
	}
}

func TestValidateDNSListConfig_RejectsTraversalTags(t *testing.T) {
	err := validateDNSListConfig(&configV2{
		DNSBlockLists: []*BlockList{{Tag: "../evil", URL: "https://example.com/list"}},
	})
	if err == nil {
		t.Fatal("expected error for traversal blocklist tag")
	}

	err = validateDNSListConfig(&configV2{
		DNSWhiteLists: []*BlockList{{Tag: "ok-list"}},
	})
	if err != nil {
		t.Fatalf("valid whitelist tag rejected: %v", err)
	}
}

func TestProcessBlockList_RejectsBadTag(t *testing.T) {
	prev := STATE.Load()
	t.Cleanup(func() { STATE.Store(prev) })
	dir := t.TempDir()
	STATE.Store(&stateV2{BlockListPath: dir + string(os.PathSeparator)})

	r := processBlockList(&BlockList{
		Tag:     "../escape",
		Enabled: true,
		URL:     "https://example.com/list.txt",
	}, true, nil)
	if r.set != nil {
		t.Fatal("processBlockList should not load domains for a bad tag")
	}
	// Ensure no file was written outside the blocklist dir.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("expected no files under blocklist dir, got %v", entries)
	}
}
