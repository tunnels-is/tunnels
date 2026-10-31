package client

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/types"
)

func dnsQueryMsg(name string) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeA)
	return m
}

// TestDNSListDecision_WhitelistTakesPriority ...
func TestDNSListDecision_WhitelistTakesPriority(t *testing.T) {
	prevBL := DNSBlockList.Load()
	prevWL := DNSWhiteList.Load()
	t.Cleanup(func() {
		DNSBlockList.Store(prevBL)
		DNSWhiteList.Store(prevWL)
	})

	blockedDomain := "both.example.com"
	onlyBlocked := "ads.tracker.example"
	onlyWhite := "safe.partner.example"
	clean := "clean.example.com"

	DNSBlockList.Store(NewCatalog(
		[]string{"Ads"},
		[]*DomainSet{DomainSetFromDomains([]string{blockedDomain, onlyBlocked})},
	))
	DNSWhiteList.Store(NewCatalog(
		[]string{"custom"},
		[]*DomainSet{DomainSetFromDomains([]string{blockedDomain, onlyWhite})},
	))

	blocked, tag := dnsListDecision(dnsQueryMsg(blockedDomain))
	if blocked {
		t.Fatalf("%s should be allowed (whitelist priority), got blocked tag=%q", blockedDomain, tag)
	}
	if tag != "" {
		t.Fatalf("whitelisted decision should not return a block tag, got %q", tag)
	}
	if !isWhitelisted(dnsQueryMsg(blockedDomain)) {
		t.Fatal("expected isWhitelisted true")
	}
	if ok, _ := isBlocked(dnsQueryMsg(blockedDomain)); !ok {
		t.Fatal("expected isBlocked true for dual-listed domain (decision still must allow)")
	}

	blocked, tag = dnsListDecision(dnsQueryMsg(onlyBlocked))
	if !blocked {
		t.Fatalf("%s should be blocked", onlyBlocked)
	}
	if tag != "Ads" {
		t.Fatalf("block tag = %q, want Ads", tag)
	}

	blocked, tag = dnsListDecision(dnsQueryMsg(onlyWhite))
	if blocked {
		t.Fatalf("%s should be allowed (whitelist only), tag=%q", onlyWhite, tag)
	}

	blocked, tag = dnsListDecision(dnsQueryMsg(clean))
	if blocked {
		t.Fatalf("%s should be allowed (not listed), tag=%q", clean, tag)
	}

	for _, name := range []string{"BOTH.EXAMPLE.COM", "both.example.com.", "Both.Example.Com."} {
		blocked, tag = dnsListDecision(dnsQueryMsg(name))
		if blocked {
			t.Fatalf("%q should be allowed via whitelist, tag=%q", name, tag)
		}
	}
}

// TestIsWhitelistedAndIsBlocked_NilCatalogs ...
func TestIsWhitelistedAndIsBlocked_NilCatalogs(t *testing.T) {
	prevBL := DNSBlockList.Load()
	prevWL := DNSWhiteList.Load()
	t.Cleanup(func() {
		DNSBlockList.Store(prevBL)
		DNSWhiteList.Store(prevWL)
	})

	DNSBlockList.Store(nil)
	DNSWhiteList.Store(nil)
	m := dnsQueryMsg("anything.example.com")
	if isWhitelisted(m) {
		t.Fatal("nil whitelist catalog should not match")
	}
	if ok, _ := isBlocked(m); ok {
		t.Fatal("nil blocklist catalog should not match")
	}
	blocked, _ := dnsListDecision(m)
	if blocked {
		t.Fatal("nil catalogs should not block")
	}

	DNSBlockList.Store(EmptyCatalog())
	DNSWhiteList.Store(EmptyCatalog())
	if isWhitelisted(m) {
		t.Fatal("empty whitelist should not match")
	}
	if ok, _ := isBlocked(m); ok {
		t.Fatal("empty blocklist should not match")
	}
}

// TestDNSAMapping ...
func TestDNSAMapping(t *testing.T) {

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
			name:       "single word with no record",
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

// TestDNSAMappingWithNilRecords ...
func TestDNSAMappingWithNilRecords(t *testing.T) {

	dnsRecords := []*types.DNSRecord{
		{
			Domain:   "valid.com",
			Wildcard: false,
			IP:       []string{"192.168.1.1"},
		},
		nil,
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

// TestDNSAMappingWildcardBehavior ...
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

			shouldFind: true,
		},
		{
			name:       "3-part domain doesn't extract subdomain",
			fullDomain: "api.example.com",

			expectedDomain: "api.example.com",
			expectedIP:     "192.168.1.100",
			shouldFind:     true,
		},
		{
			name:       "5-part domain with deeper subdomain",
			fullDomain: "very.deep.api.example.com",

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

// TestDNSAMappingEmptyArray ...
func TestDNSAMappingEmptyArray(t *testing.T) {

	emptyRecords := []*types.DNSRecord{}

	result := DNSAMapping(emptyRecords, "example.com")
	if result != nil {
		t.Error("Should return nil for empty DNS records array")
	}

	t.Log("Empty array handled correctly ✓")
}

// TestDNSAMappingComplexDomains ...
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
			name:       "wildcard matches multi-level subdomain",
			fullDomain: "b.c.d.example.com",

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

// TestDNSAMappingSingleLabel ...
func TestDNSAMappingSingleLabel(t *testing.T) {
	records := []*types.DNSRecord{
		{
			Domain:   "loganisgay",
			Wildcard: true,
			IP:       []string{"10.0.1.3"},
		},
		{
			Domain:   "Example.COM",
			Wildcard: true,
			IP:       []string{"192.0.2.1"},
		},
	}

	tests := []struct {
		name           string
		fullDomain     string
		expectedDomain string
		shouldFind     bool
	}{
		{
			name:           "exact single label",
			fullDomain:     "loganisgay",
			expectedDomain: "loganisgay",
			shouldFind:     true,
		},
		{
			name:           "exact single label with trailing dot",
			fullDomain:     "loganisgay.",
			expectedDomain: "loganisgay",
			shouldFind:     true,
		},
		{
			name:       "single label wildcard does not cover deeper names",
			fullDomain: "_8080._https.loganisgay.",
			shouldFind: false,
		},
		{
			name:           "case insensitive public name",
			fullDomain:     "WWW.Example.COM.",
			expectedDomain: "Example.COM",
			shouldFind:     true,
		},
		{
			name:       "different single label",
			fullDomain: "notthis",
			shouldFind: false,
		},
		{
			name:       "suffix without a label boundary",
			fullDomain: "notloganisgay.",
			shouldFind: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := DNSAMapping(records, tc.fullDomain)
			if !tc.shouldFind {
				if result != nil {
					t.Errorf("Expected nil for %q, got %+v", tc.fullDomain, result)
				}
				return
			}
			if result == nil {
				t.Errorf("Expected record for %q", tc.fullDomain)
				return
			}
			if result.Domain != tc.expectedDomain {
				t.Errorf("Expected domain %q, got %q", tc.expectedDomain, result.Domain)
			}
		})
	}

	exact := []*types.DNSRecord{{
		Domain:   "loganisgay",
		Wildcard: false,
		IP:       []string{"10.0.1.3"},
	}}
	if got := DNSAMapping(exact, "_8080._https.loganisgay."); got != nil {
		t.Fatalf("subdomain must not match a single-label record without wildcard, got %+v", got)
	}
	if got := DNSAMapping(exact, "loganisgay."); got == nil {
		t.Fatal("apex of a single-label record must match without wildcard")
	}

	opted := []*types.DNSRecord{{
		Domain:        "loganisgay",
		Wildcard:      true,
		ShortWildcard: true,
		IP:            []string{"10.0.1.3"},
	}}
	if got := DNSAMapping(opted, "_8080._https.loganisgay."); got == nil || got.Domain != "loganisgay" {
		t.Fatalf("opted-in single-label wildcard should cover the deeper name, got %+v", got)
	}
}

func TestDNSAMapping_ShortZoneAndExactTie(t *testing.T) {
	short := []*types.DNSRecord{{
		Domain:   "com",
		Wildcard: true,
		IP:       []string{"192.0.2.1"},
	}}
	if got := DNSAMapping(short, "example.com"); got != nil {
		t.Fatalf("one-label wildcard covered example.com: %+v", got)
	}
	if got := DNSAMapping(short, "com."); got == nil {
		t.Fatal("the one-label name itself must still match")
	}
	short[0].ShortWildcard = true
	if got := DNSAMapping(short, "a.b.com."); got == nil || got.IP[0] != "192.0.2.1" {
		t.Fatalf("opted-in short wildcard missed a.b.com: %+v", got)
	}

	both := []*types.DNSRecord{
		{Domain: "example.com", Wildcard: true, IP: []string{"192.0.2.1"}},
		{Domain: "example.com", Wildcard: false, IP: []string{"192.0.2.9"}},
	}
	if got := DNSAMapping(both, "example.com"); got == nil || got.IP[0] != "192.0.2.9" {
		t.Fatalf("exact record lost the tie: %+v", got)
	}
	if got := DNSAMapping(both, "www.example.com"); got == nil || got.IP[0] != "192.0.2.1" {
		t.Fatalf("dotted wildcard should still cover www: %+v", got)
	}

	exactFirst := []*types.DNSRecord{
		{Domain: "example.com", Wildcard: false, IP: []string{"192.0.2.9"}},
		{Domain: "example.com", Wildcard: true, IP: []string{"192.0.2.1"}},
	}
	if got := DNSAMapping(exactFirst, "example.com"); got == nil || got.IP[0] != "192.0.2.9" {
		t.Fatalf("exact record listed first lost the tie: %+v", got)
	}
}
