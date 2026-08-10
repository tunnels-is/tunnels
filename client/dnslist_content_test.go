package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupCustomListTestEnv(t *testing.T) (blockDir, whiteDir string) {
	t.Helper()
	root := t.TempDir()
	blockDir = filepath.Join(root, "blocklists") + string(os.PathSeparator)
	whiteDir = filepath.Join(root, "whitelists") + string(os.PathSeparator)
	if err := os.MkdirAll(blockDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(whiteDir, 0o700); err != nil {
		t.Fatal(err)
	}

	prevState := STATE.Load()
	prevConfig := CONFIG.Load()
	prevBL := DNSBlockList.Load()
	prevWL := DNSWhiteList.Load()
	t.Cleanup(func() {
		STATE.Store(prevState)
		CONFIG.Store(prevConfig)
		DNSBlockList.Store(prevBL)
		DNSWhiteList.Store(prevWL)
	})

	STATE.Store(&stateV2{
		BasePath:       root + string(os.PathSeparator),
		BlockListPath:  blockDir,
		WhiteListPath:  whiteDir,
		ConfigFileName: filepath.Join(root, "tunnels.conf"),
	})
	CONFIG.Store(&configV2{
		DNSBlockLists: GetDefaultBlockLists(),
		DNSWhiteLists: GetDefaultWhiteLists(),
	})
	DNSBlockList.Store(EmptyCatalog())
	DNSWhiteList.Store(EmptyCatalog())
	return blockDir, whiteDir
}

func TestGetSetCustomDNSListContent_Whitelist(t *testing.T) {
	_, whiteDir := setupCustomListTestEnv(t)

	// Missing file is created with starter content.
	got, err := getCustomDNSListContent("whitelist")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "whitelist" {
		t.Fatalf("kind=%q", got.Kind)
	}
	if !strings.Contains(got.Content, "custom DNS whitelist") {
		t.Fatalf("expected starter content, got %q", got.Content)
	}

	content := "# my list\nallow.example.com\nAlso.Example.COM\n\nbadline\n"
	out, err := setCustomDNSListContent("whitelist", content)
	if err != nil {
		t.Fatal(err)
	}
	if out.Count != 2 {
		t.Fatalf("count=%d want 2 (duplicate EXAMPLE.COM collapsed in parse count?)", out.Count)
	}
	// tryAddLine accepts both lines before unique dedup in Build — count is lines accepted.
	// EXAMPLE.COM and allow.example.com both succeed → count 2. Unique set len may be 2.

	data, err := os.ReadFile(filepath.Join(whiteDir, customDNSListTag))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("file content mismatch")
	}

	// Live catalog should include the domain.
	if ok, _ := DNSWhiteList.Load().Has("allow.example.com"); !ok {
		t.Fatal("whitelist catalog missing allow.example.com")
	}
	if ok, _ := DNSWhiteList.Load().Has("also.example.com"); !ok {
		t.Fatal("whitelist catalog missing also.example.com")
	}

	// Config count updated.
	cfg := CONFIG.Load()
	found := false
	for _, wl := range cfg.DNSWhiteLists {
		if wl != nil && strings.EqualFold(wl.Tag, customDNSListTag) {
			found = true
			if wl.Count != out.Count {
				t.Fatalf("config count=%d want %d", wl.Count, out.Count)
			}
		}
	}
	if !found {
		t.Fatal("custom whitelist missing from config")
	}

	// Round-trip get.
	got, err = getCustomDNSListContent("whitelist")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != content {
		t.Fatal("get after set content mismatch")
	}
}

func TestSetCustomDNSListContent_Blocklist(t *testing.T) {
	setupCustomListTestEnv(t)

	// Seed another list in the catalog so we prove it is preserved.
	other := DomainSetFromDomains([]string{"remote.ads.example"})
	DNSBlockList.Store(NewCatalog([]string{"Ads"}, []*DomainSet{other}))

	out, err := setCustomDNSListContent("blocklist", "evil.tracker.example\n")
	if err != nil {
		t.Fatal(err)
	}
	if out.Count != 1 {
		t.Fatalf("count=%d", out.Count)
	}

	cat := DNSBlockList.Load()
	if ok, tag := cat.Has("evil.tracker.example"); !ok || !strings.EqualFold(tag, customDNSListTag) {
		t.Fatalf("custom domain missing: ok=%v tag=%q", ok, tag)
	}
	if ok, tag := cat.Has("remote.ads.example"); !ok || tag != "Ads" {
		t.Fatalf("remote list should be preserved: ok=%v tag=%q", ok, tag)
	}
}

func TestSetCustomDNSListContent_EmptyReseedsTemplate(t *testing.T) {
	setupCustomListTestEnv(t)
	out, err := setCustomDNSListContent("whitelist", "   \n\t  ")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Content, "custom DNS whitelist") {
		t.Fatalf("empty save should reseed template, got %q", out.Content)
	}
}

func TestSetCustomDNSListContent_InvalidKind(t *testing.T) {
	setupCustomListTestEnv(t)
	if _, err := setCustomDNSListContent("nope", "x.com"); err == nil {
		t.Fatal("expected error for invalid kind")
	}
	if _, err := getCustomDNSListContent(""); err == nil {
		t.Fatal("expected error for empty kind")
	}
}

func TestSetCustomDNSListContent_TooLarge(t *testing.T) {
	setupCustomListTestEnv(t)
	big := strings.Repeat("a.example.com\n", maxCustomDNSListContent)
	if _, err := setCustomDNSListContent("whitelist", big); err == nil {
		t.Fatal("expected too-large error")
	}
}

func TestDNSListDecision_AfterCustomWhitelistSave(t *testing.T) {
	setupCustomListTestEnv(t)

	// Domain on both lists — whitelist should win after save.
	DNSBlockList.Store(NewCatalog(
		[]string{"Ads"},
		[]*DomainSet{DomainSetFromDomains([]string{"both.example.com"})},
	))
	if _, err := setCustomDNSListContent("whitelist", "both.example.com\n"); err != nil {
		t.Fatal(err)
	}

	m := dnsQueryMsg("both.example.com")
	blocked, _ := dnsListDecision(m)
	if blocked {
		t.Fatal("whitelisted domain must not be blocked")
	}
}
