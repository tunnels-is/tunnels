package client

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckIfURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{

			name:     "http URL",
			input:    "http://example.com",
			expected: false,
		},
		{
			name:     "https URL",
			input:    "https://example.com",
			expected: true,
		},
		{
			name:     "https URL with path",
			input:    "https://example.com/path/to/file.txt",
			expected: true,
		},
		{
			name:     "http URL with query",
			input:    "http://example.com?query=param",
			expected: false,
		},
		{
			name:     "not a URL",
			input:    "example.com",
			expected: false,
		},
		{
			name:     "file path",
			input:    "/path/to/file",
			expected: false,
		},
		{
			name:     "ftp URL",
			input:    "ftp://example.com",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "just http",
			input:    "http",
			expected: false,
		},
		{
			name:     "just https",
			input:    "https",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CheckIfURL(tc.input)
			if result != tc.expected {
				t.Errorf("CheckIfURL(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestGetDefaultBlockLists(t *testing.T) {
	lists := GetDefaultBlockLists()

	if len(lists) == 0 {
		t.Error("Default block lists should not be empty")
	}

	for i, bl := range lists {
		if bl == nil {
			t.Errorf("Block list at index %d is nil", i)
			continue
		}

		if bl.Tag == "" {
			t.Errorf("Block list at index %d has empty Tag", i)
		}

		isCustom := bl.Tag == customDNSListTag
		if isCustom {
			if bl.URL != "" {
				t.Errorf("custom block list should have empty URL, got %q", bl.URL)
			}
			if !bl.Enabled {
				t.Error("custom block list should be Enabled")
			}
		} else if bl.URL == "" {
			t.Errorf("Block list at index %d (%s) has empty URL", i, bl.Tag)
		} else if !CheckIfURL(bl.URL) {
			t.Errorf("Block list %s has invalid URL: %s", bl.Tag, bl.URL)
		}

		yearsDiff := time.Since(bl.LastDownload).Hours() / 24 / 365
		if yearsDiff < 1.9 || yearsDiff > 2.1 {
			t.Errorf("Block list %s LastDownload should be ~2 years ago, got %.2f years", bl.Tag, yearsDiff)
		}

		t.Logf("Block list: Tag=%s, URL=%s, Enabled=%v", bl.Tag, bl.URL, bl.Enabled)
	}

	expectedTags := []string{customDNSListTag, "Ads", "AdultContent", "CryptoCurrency", "Drugs", "FakeNews",
		"Fraud", "Gambling", "Malware", "SocialMedia", "Surveillance"}

	foundTags := make(map[string]bool)
	for _, bl := range lists {
		if bl != nil {
			foundTags[bl.Tag] = true
		}
	}

	for _, tag := range expectedTags {
		if !foundTags[tag] {
			t.Errorf("Expected default block list with tag %q not found", tag)
		}
	}

	if lists[0].Tag != customDNSListTag {
		t.Errorf("custom block list should be first, got %q", lists[0].Tag)
	}

	t.Logf("Total default block lists: %d", len(lists))
}

func TestGetDefaultWhiteLists(t *testing.T) {
	lists := GetDefaultWhiteLists()

	if lists == nil {
		t.Fatal("Default white lists should not be nil")
	}
	if len(lists) != 1 {
		t.Fatalf("expected 1 default white list, got %d", len(lists))
	}

	for i, wl := range lists {
		if wl == nil {
			t.Errorf("White list at index %d is nil", i)
			continue
		}

		if wl.Tag == "" && wl.URL == "" {
			t.Errorf("White list at index %d has both empty Tag and URL", i)
		}

		if wl.URL != "" && !CheckIfURL(wl.URL) {
			t.Errorf("White list %s has invalid URL: %s", wl.Tag, wl.URL)
		}

		yearsDiff := time.Since(wl.LastDownload).Hours() / 24 / 365
		if yearsDiff < 1.9 || yearsDiff > 2.1 {
			t.Errorf("White list %s LastDownload should be ~2 years ago, got %.2f years", wl.Tag, yearsDiff)
		}

		t.Logf("White list: Tag=%s, URL=%s, Enabled=%v", wl.Tag, wl.URL, wl.Enabled)
	}

	def := lists[0]
	if def.Tag != customDNSListTag {
		t.Errorf("custom white list tag = %q, want %q", def.Tag, customDNSListTag)
	}
	if !def.Enabled {
		t.Error("custom white list should be Enabled")
	}
	if def.URL != "" {
		t.Errorf("custom white list should have empty URL (local file), got %q", def.URL)
	}

	t.Logf("Total default white lists: %d", len(lists))
}

func TestEnsureCustomDNSListFiles(t *testing.T) {
	cases := []struct {
		name    string
		ensure  func(string) error
		content string
	}{
		{"whitelist", ensureCustomWhiteListFile, customWhiteListFileContent},
		{"blocklist", ensureCustomBlockListFile, customBlockListFileContent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, customDNSListTag)

			if err := tc.ensure(dir); err != nil {
				t.Fatalf("create: %v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if string(data) != tc.content {
				t.Fatalf("unexpected custom file content:\n%s", data)
			}

			// Existing non-empty file must not be overwritten.
			user := []byte("keep.example.com\n")
			if err := os.WriteFile(path, user, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := tc.ensure(dir); err != nil {
				t.Fatalf("re-ensure: %v", err)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != string(user) {
				t.Fatalf("user file was overwritten: %q", data)
			}

			// Empty file is recreated.
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := tc.ensure(dir); err != nil {
				t.Fatalf("recreate empty: %v", err)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tc.content {
				t.Fatalf("empty file was not recreated")
			}
		})
	}
}

func TestEnsureCustomDNSListInConfig(t *testing.T) {
	t.Run("whitelist", func(t *testing.T) {
		cfg := &configV2{}
		if !ensureCustomWhiteListInConfig(cfg) {
			t.Fatal("expected config change when custom whitelist missing")
		}
		if len(cfg.DNSWhiteLists) != 1 || cfg.DNSWhiteLists[0].Tag != customDNSListTag {
			t.Fatalf("custom whitelist not added: %+v", cfg.DNSWhiteLists)
		}
		if !cfg.DNSWhiteLists[0].Enabled {
			t.Fatal("custom whitelist should be enabled on creation")
		}

		cfg.DNSWhiteLists[0].Enabled = false
		if ensureCustomWhiteListInConfig(cfg) {
			t.Fatal("expected no change when custom already present")
		}
		if cfg.DNSWhiteLists[0].Enabled {
			t.Fatal("should not re-enable a user-disabled custom whitelist")
		}
		if len(cfg.DNSWhiteLists) != 1 {
			t.Fatalf("should not duplicate custom, got %d entries", len(cfg.DNSWhiteLists))
		}

		cfg2 := &configV2{DNSWhiteLists: []*BlockList{{Tag: "CUSTOM", Enabled: false}}}
		if ensureCustomWhiteListInConfig(cfg2) {
			t.Fatal("CUSTOM should count as the custom tag")
		}
	})

	t.Run("blocklist", func(t *testing.T) {
		cfg := &configV2{}
		if !ensureCustomBlockListInConfig(cfg) {
			t.Fatal("expected config change when custom blocklist missing")
		}
		if len(cfg.DNSBlockLists) != 1 || cfg.DNSBlockLists[0].Tag != customDNSListTag {
			t.Fatalf("custom blocklist not added: %+v", cfg.DNSBlockLists)
		}
		if !cfg.DNSBlockLists[0].Enabled {
			t.Fatal("custom blocklist should be enabled on creation")
		}

		cfg.DNSBlockLists[0].Enabled = false
		if ensureCustomBlockListInConfig(cfg) {
			t.Fatal("expected no change when custom already present at front")
		}
		if cfg.DNSBlockLists[0].Enabled {
			t.Fatal("should not re-enable a user-disabled custom blocklist")
		}

		cfg2 := &configV2{DNSBlockLists: []*BlockList{{Tag: "Custom", Enabled: false}}}
		if ensureCustomBlockListInConfig(cfg2) {
			t.Fatal("Custom should count as the custom tag")
		}
	})

	t.Run("custom moved to front", func(t *testing.T) {
		cfg := &configV2{
			DNSBlockLists: []*BlockList{
				{Tag: "Ads", Enabled: true},
				{Tag: "custom", Enabled: false},
				{Tag: "Malware", Enabled: true},
			},
		}
		if !ensureCustomBlockListInConfig(cfg) {
			t.Fatal("expected reorder when custom not first")
		}
		if len(cfg.DNSBlockLists) != 3 {
			t.Fatalf("len=%d", len(cfg.DNSBlockLists))
		}
		if cfg.DNSBlockLists[0].Tag != "custom" {
			t.Fatalf("custom should be first, got %q", cfg.DNSBlockLists[0].Tag)
		}
		if cfg.DNSBlockLists[0].Enabled {
			t.Fatal("reorder must not re-enable custom")
		}
		if cfg.DNSBlockLists[1].Tag != "Ads" || cfg.DNSBlockLists[2].Tag != "Malware" {
			t.Fatalf("unexpected order: %+v", cfg.DNSBlockLists)
		}
		// Already first → no change.
		if ensureCustomBlockListInConfig(cfg) {
			t.Fatal("expected no change when custom already first")
		}
	})

	t.Run("custom prepended before existing lists", func(t *testing.T) {
		cfg := &configV2{
			DNSBlockLists: []*BlockList{{Tag: "Ads", Enabled: true}},
		}
		if !ensureCustomBlockListInConfig(cfg) {
			t.Fatal("expected prepend")
		}
		if len(cfg.DNSBlockLists) != 2 || cfg.DNSBlockLists[0].Tag != customDNSListTag || cfg.DNSBlockLists[1].Tag != "Ads" {
			t.Fatalf("unexpected lists: %+v", cfg.DNSBlockLists)
		}
	})
}
