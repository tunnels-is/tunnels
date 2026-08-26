package client

import (
	"os"
	"path/filepath"
	"testing"
)

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
