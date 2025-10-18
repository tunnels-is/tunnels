package client

import (
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
			expected: true,
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
			expected: true,
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
			expected: true,
		},
		{
			name:     "just https",
			input:    "https",
			expected: true,
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

	// Check that all lists have required fields
	for i, bl := range lists {
		if bl == nil {
			t.Errorf("Block list at index %d is nil", i)
			continue
		}

		if bl.Tag == "" {
			t.Errorf("Block list at index %d has empty Tag", i)
		}

		if bl.URL == "" {
			t.Errorf("Block list at index %d (%s) has empty URL", i, bl.Tag)
		}

		// Check that URL is valid
		if !CheckIfURL(bl.URL) {
			t.Errorf("Block list %s has invalid URL: %s", bl.Tag, bl.URL)
		}

		// Check that LastDownload is set to 2 years ago
		yearsDiff := time.Since(bl.LastDownload).Hours() / 24 / 365
		if yearsDiff < 1.9 || yearsDiff > 2.1 {
			t.Errorf("Block list %s LastDownload should be ~2 years ago, got %.2f years", bl.Tag, yearsDiff)
		}

		t.Logf("Block list: Tag=%s, URL=%s", bl.Tag, bl.URL)
	}

	// Check for expected default lists
	expectedTags := []string{"Ads", "AdultContent", "CryptoCurrency", "Drugs", "FakeNews",
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

	t.Logf("Total default block lists: %d", len(lists))
}

func TestGetDefaultWhiteLists(t *testing.T) {
	lists := GetDefaultWhiteLists()

	// Currently returns empty list, but should be valid
	if lists == nil {
		t.Error("Default white lists should not be nil")
	}

	// Check that all lists have required fields if any exist
	for i, wl := range lists {
		if wl == nil {
			t.Errorf("White list at index %d is nil", i)
			continue
		}

		if wl.Tag == "" && wl.URL == "" {
			t.Errorf("White list at index %d has both empty Tag and URL", i)
		}

		// If URL is provided, check that it's valid
		if wl.URL != "" && !CheckIfURL(wl.URL) {
			t.Errorf("White list %s has invalid URL: %s", wl.Tag, wl.URL)
		}

		// Check that LastDownload is set to 2 years ago
		yearsDiff := time.Since(wl.LastDownload).Hours() / 24 / 365
		if yearsDiff < 1.9 || yearsDiff > 2.1 {
			t.Errorf("White list %s LastDownload should be ~2 years ago, got %.2f years", wl.Tag, yearsDiff)
		}

		t.Logf("White list: Tag=%s, URL=%s", wl.Tag, wl.URL)
	}

	t.Logf("Total default white lists: %d", len(lists))
}
