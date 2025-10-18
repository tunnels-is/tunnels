package client

import (
	"strings"
	"testing"
)

func TestCreateConnectionUUID(t *testing.T) {
	uuid1 := CreateConnectionUUID()
	uuid2 := CreateConnectionUUID()

	// Test format: should be {XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}
	if !strings.HasPrefix(uuid1, "{") || !strings.HasSuffix(uuid1, "}") {
		t.Errorf("UUID should be wrapped in braces, got: %s", uuid1)
	}

	// Test length: {8-4-4-4-12} + braces = 38 characters
	if len(uuid1) != 38 {
		t.Errorf("UUID length should be 38, got: %d", len(uuid1))
	}

	// Test uniqueness
	if uuid1 == uuid2 {
		t.Errorf("Two UUIDs should not be the same: %s == %s", uuid1, uuid2)
	}

	// Test uppercase
	inner := strings.Trim(uuid1, "{}")
	if inner != strings.ToUpper(inner) {
		t.Errorf("UUID should be uppercase, got: %s", uuid1)
	}

	t.Logf("UUID1: %s", uuid1)
	t.Logf("UUID2: %s", uuid2)
}

func TestIsAlphanumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "lowercase letters only",
			input:    "abcdefghijklmnopqrstuvwxyz",
			expected: true,
		},
		{
			name:     "numbers only",
			input:    "0123456789",
			expected: true,
		},
		{
			name:     "mixed lowercase and numbers",
			input:    "abc123xyz789",
			expected: true,
		},
		{
			name:     "with uppercase letters",
			input:    "Abc123",
			expected: false,
		},
		{
			name:     "with special characters",
			input:    "abc-123",
			expected: false,
		},
		{
			name:     "with spaces",
			input:    "abc 123",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "with underscores",
			input:    "abc_123",
			expected: false,
		},
		{
			name:     "with dots",
			input:    "abc.123",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsAlphanumeric(tc.input)
			if result != tc.expected {
				t.Errorf("IsAlphanumeric(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestCopySlice(t *testing.T) {
	original := []byte{1, 2, 3, 4, 5}
	copied := CopySlice(original)

	// Test that contents are the same
	if len(copied) != len(original) {
		t.Errorf("Copied slice length %d != original length %d", len(copied), len(original))
	}

	for i := range original {
		if copied[i] != original[i] {
			t.Errorf("Copied[%d] = %d, expected %d", i, copied[i], original[i])
		}
	}

	// Test that they are different slices (not same reference)
	original[0] = 99
	if copied[0] == 99 {
		t.Error("Modifying original should not affect the copy")
	}

	t.Logf("Original: %v", original)
	t.Logf("Copied:   %v", copied)
}

func TestGetDomainAndSubDomain(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedDomain   string
		expectedSubdomain string
	}{
		{
			name:             "simple domain",
			input:            "example.com",
			expectedDomain:   "example.com",
			expectedSubdomain: "",
		},
		{
			name:             "domain with subdomain",
			input:            "www.example.com",
			expectedDomain:   "www.example.com",
			expectedSubdomain: "",
		},
		{
			name:             "domain with multiple subdomains",
			input:            "api.v2.example.com",
			expectedDomain:   "v2.example.com",
			expectedSubdomain: "api",
		},
		{
			name:             "domain with many subdomains",
			input:            "a.b.c.d.example.com",
			expectedDomain:   "d.example.com",
			expectedSubdomain: "a.b.c",
		},
		{
			name:             "single word (invalid)",
			input:            "localhost",
			expectedDomain:   "",
			expectedSubdomain: "",
		},
		{
			name:             "empty string",
			input:            "",
			expectedDomain:   "",
			expectedSubdomain: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			domain, subdomain := GetDomainAndSubDomain(tc.input)
			if domain != tc.expectedDomain {
				t.Errorf("Domain = %q, expected %q", domain, tc.expectedDomain)
			}
			if subdomain != tc.expectedSubdomain {
				t.Errorf("Subdomain = %q, expected %q", subdomain, tc.expectedSubdomain)
			}
			t.Logf("Input: %q -> Domain: %q, Subdomain: %q", tc.input, domain, subdomain)
		})
	}
}

func TestCheckIfPlainDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid domain",
			input:    "example.com",
			expected: true,
		},
		{
			name:     "domain with subdomain",
			input:    "www.example.com",
			expected: true,
		},
		{
			name:     "single word without dot",
			input:    "localhost",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "domain with multiple dots",
			input:    "api.v2.example.com",
			expected: true,
		},
		{
			name:     "IP address",
			input:    "192.168.1.1",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CheckIfPlainDomain(tc.input)
			if result != tc.expected {
				t.Errorf("CheckIfPlainDomain(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsDefaultConnection(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "exact match lowercase",
			input:    DefaultTunnelName,
			expected: true,
		},
		{
			name:     "exact match uppercase",
			input:    strings.ToUpper(DefaultTunnelName),
			expected: true,
		},
		{
			name:     "exact match mixed case",
			input:    "TuNnElS",
			expected: true,
		},
		{
			name:     "different name",
			input:    "custom",
			expected: false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "with spaces",
			input:    " " + DefaultTunnelName + " ",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsDefaultConnection(tc.input)
			if result != tc.expected {
				t.Errorf("IsDefaultConnection(%q) = %v, expected %v", tc.input, result, tc.expected)
			}
		})
	}
}
