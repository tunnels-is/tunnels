package client

import (
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func TestDNSAMapping(t *testing.T) {
	// Create test DNS records
	dnsRecords := []*types.DNSRecord{
		{
			Domain:   "example.com",
			Wildcard: false,
			IP:       []string{"192.168.1.1"},
		},
		{
			Domain:   "test.example.com",
			Wildcard: false,
			IP:       []string{"192.168.1.2"},
		},
		{
			Domain:   "wildcard.example.com",
			Wildcard: true,
			IP:       []string{"192.168.1.3"},
		},
		{
			Domain:   "api.service.com",
			Wildcard: false,
			IP:       []string{"10.0.0.1"},
		},
	}

	tests := []struct {
		name           string
		fullDomain     string
		expectedDomain string
		shouldFind     bool
	}{
		{
			name:           "exact match - simple domain",
			fullDomain:     "example.com",
			expectedDomain: "example.com",
			shouldFind:     true,
		},
		{
			name:           "exact match - with subdomain",
			fullDomain:     "test.example.com",
			expectedDomain: "test.example.com",
			shouldFind:     true,
		},
		{
			name:           "wildcard match",
			fullDomain:     "anything.wildcard.example.com",
			expectedDomain: "wildcard.example.com",
			shouldFind:     true,
		},
		{
			name:           "no match - different domain",
			fullDomain:     "notfound.com",
			expectedDomain: "",
			shouldFind:     false,
		},
		{
			name:           "no match - subdomain without wildcard",
			fullDomain:     "sub.example.com",
			expectedDomain: "",
			shouldFind:     false,
		},
		{
			name:           "with trailing dot",
			fullDomain:     "example.com.",
			expectedDomain: "example.com",
			shouldFind:     true,
		},
		{
			name:           "multi-level subdomain",
			fullDomain:     "api.service.com",
			expectedDomain: "api.service.com",
			shouldFind:     true,
		},
		{
			name:       "empty domain",
			fullDomain: "",
			shouldFind: false,
		},
		{
			name:       "single word (invalid)",
			fullDomain: "localhost",
			shouldFind: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DNSAMapping(dnsRecords, tc.fullDomain)

			if tc.shouldFind {
				if result == nil {
					t.Errorf("Expected to find DNS record for %q, but got nil", tc.fullDomain)
					return
				}
				if result.Domain != tc.expectedDomain {
					t.Errorf("Expected domain %q, got %q", tc.expectedDomain, result.Domain)
				}
				t.Logf("Found DNS record: %q -> %v ✓", tc.fullDomain, result.IP)
			} else {
				if result != nil {
					t.Errorf("Expected nil for %q, but found record: %+v", tc.fullDomain, result)
				}
				t.Logf("Correctly returned nil for %q ✓", tc.fullDomain)
			}
		})
	}
}

func TestDNSAMappingWithNilRecords(t *testing.T) {
	// Test with nil records in the array
	dnsRecords := []*types.DNSRecord{
		{
			Domain:   "valid.com",
			Wildcard: false,
			IP:       []string{"192.168.1.1"},
		},
		nil, // nil record should be handled gracefully
		{
			Domain:   "another.com",
			Wildcard: false,
			IP:       []string{"192.168.1.2"},
		},
	}

	result := DNSAMapping(dnsRecords, "valid.com")
	if result == nil {
		t.Error("Should find valid.com despite nil record in array")
	}

	result = DNSAMapping(dnsRecords, "another.com")
	if result == nil {
		t.Error("Should find another.com despite nil record in array")
	}

	t.Log("Nil records handled correctly ✓")
}

func TestDNSAMappingWildcardBehavior(t *testing.T) {
	dnsRecords := []*types.DNSRecord{
		{
			Domain:   "exact.api.example.com",
			Wildcard: false,
			IP:       []string{"192.168.1.1"},
		},
		{
			Domain:   "api.example.com",
			Wildcard: true,
			IP:       []string{"192.168.1.100"},
		},
	}

	tests := []struct {
		name           string
		fullDomain     string
		expectedDomain string
		expectedIP     string
		shouldFind     bool
	}{
		{
			name:           "exact match when domain recorded",
			fullDomain:     "exact.api.example.com",
			expectedDomain: "exact.api.example.com",
			expectedIP:     "192.168.1.1",
			shouldFind:     true,
		},
		{
			name:           "wildcard matches with extracted subdomain",
			fullDomain:     "anything.api.example.com",
			expectedDomain: "api.example.com",
			expectedIP:     "192.168.1.100",
			// GetDomainAndSubDomain("anything.api.example.com") with 4 parts -> domain="api.example.com", subdomain="anything"
			// record.Domain="api.example.com" matches, subdomain!="", record.Wildcard=true -> MATCH
			shouldFind: true,
		},
		{
			name:       "3-part domain doesn't extract subdomain",
			fullDomain: "api.example.com",
			// GetDomainAndSubDomain("api.example.com") with 3 parts -> domain="api.example.com", subdomain=""
			// record.Domain="api.example.com" matches, subdomain=="", no wildcard needed -> MATCH
			expectedDomain: "api.example.com",
			expectedIP:     "192.168.1.100",
			shouldFind:     true,
		},
		{
			name:       "5-part domain with deeper subdomain",
			fullDomain: "very.deep.api.example.com",
			// GetDomainAndSubDomain splits "very.deep.api.example.com" with 5 parts
			// Takes last 3: domain="api.example.com", subdomain="very.deep"
			// record.Domain="api.example.com" matches, subdomain!="", record.Wildcard=true -> MATCH
			expectedDomain: "api.example.com",
			expectedIP:     "192.168.1.100",
			shouldFind:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DNSAMapping(dnsRecords, tc.fullDomain)

			if !tc.shouldFind {
				if result != nil {
					t.Errorf("Expected nil for %q, but found: %+v", tc.fullDomain, result)
				}
				t.Logf("Correctly returned nil for %q ✓", tc.fullDomain)
				return
			}

			if result == nil {
				t.Errorf("Expected to find record for %q", tc.fullDomain)
				return
			}
			if result.Domain != tc.expectedDomain {
				t.Errorf("Expected domain %q, got %q", tc.expectedDomain, result.Domain)
			}
			if len(result.IP) > 0 && result.IP[0] != tc.expectedIP {
				t.Errorf("Expected IP %q, got %q", tc.expectedIP, result.IP[0])
			}
			t.Logf("Wildcard behavior correct: %q -> %s ✓", tc.fullDomain, tc.expectedDomain)
		})
	}
}

func TestDNSAMappingEmptyArray(t *testing.T) {
	// Test with empty DNS records array
	emptyRecords := []*types.DNSRecord{}

	result := DNSAMapping(emptyRecords, "example.com")
	if result != nil {
		t.Error("Should return nil for empty DNS records array")
	}

	t.Log("Empty array handled correctly ✓")
}

func TestDNSAMappingComplexDomains(t *testing.T) {
	dnsRecords := []*types.DNSRecord{
		{
			Domain:   "a.b.c.d.example.com",
			Wildcard: false,
			IP:       []string{"192.168.1.1"},
		},
		{
			Domain:   "d.example.com",
			Wildcard: true,
			IP:       []string{"192.168.1.2"},
		},
	}

	tests := []struct {
		name           string
		fullDomain     string
		expectedDomain string
		expectedIP     string
		shouldFind     bool
	}{
		{
			name:           "exact deep domain match",
			fullDomain:     "a.b.c.d.example.com",
			expectedDomain: "a.b.c.d.example.com",
			expectedIP:     "192.168.1.1",
			shouldFind:     true,
		},
		{
			name:           "wildcard matches 4-part subdomain",
			fullDomain:     "x.d.example.com",
			expectedDomain: "d.example.com",
			expectedIP:     "192.168.1.2",
			shouldFind:     true,
		},
		{
			name:           "wildcard matches multi-level subdomain",
			fullDomain:     "b.c.d.example.com",
			// GetDomainAndSubDomain("b.c.d.example.com") with 5 parts -> domain="d.example.com", subdomain="b.c"
			// record.Domain="d.example.com" matches, subdomain!="", record.Wildcard=true -> MATCH
			expectedDomain: "d.example.com",
			expectedIP:     "192.168.1.2",
			shouldFind:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DNSAMapping(dnsRecords, tc.fullDomain)

			if tc.shouldFind {
				if result == nil {
					t.Errorf("Expected to find DNS record for %q", tc.fullDomain)
					return
				}
				if result.Domain != tc.expectedDomain {
					t.Errorf("Expected domain %q, got %q", tc.expectedDomain, result.Domain)
				}
				if tc.expectedIP != "" && len(result.IP) > 0 && result.IP[0] != tc.expectedIP {
					t.Errorf("Expected IP %q, got %q", tc.expectedIP, result.IP[0])
				}
			} else {
				if result != nil {
					t.Errorf("Expected nil for %q, but found record: %+v", tc.fullDomain, result)
				}
			}
		})
	}
}
