package client

import (
	"strings"

	"github.com/tunnels-is/tunnels/types"
)

// GetDomainAndSubDomain ...
func GetDomainAndSubDomain(name string) (domain, subdomain string) {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	if name == "" {
		return "", ""
	}

	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		return parts[0], ""
	}
	if len(parts) == 2 {
		return name, ""
	}

	domain = strings.Join(parts[len(parts)-3:], ".")
	subdomain = strings.Join(parts[:len(parts)-3], ".")
	return
}

// DNSAMapping ...
func DNSAMapping(records []*types.DNSRecord, fullDomain string) *types.DNSRecord {
	query := normalizeDNSName(fullDomain)
	if query == "" {
		return nil
	}

	var best *types.DNSRecord
	bestLen := -1
	for i, record := range records {
		if record == nil {
			continue
		}
		zone := normalizeDNSName(record.Domain)
		if !dnsZoneMatch(record, query, zone) {
			continue
		}
		// A longer zone wins. At the same length, a name that is not a
		// wildcard wins over one that is. The first of those stays.
		if best == nil || len(zone) > bestLen || (len(zone) == bestLen && best.Wildcard && !record.Wildcard) {
			best = records[i]
			bestLen = len(zone)
		}
	}
	return best
}

// DNSWildcardCovers ...
func DNSWildcardCovers(record *types.DNSRecord) bool {
	if record == nil || !record.Wildcard {
		return false
	}
	zone := normalizeDNSName(record.Domain)
	return zone != "" && wildcardDepthAllowed(record, zone)
}

// A dotted wildcard covers every name under the zone. A one-label wildcard
// covers only its own name unless ShortWildcard is set. The apex always matches.
func dnsZoneMatch(record *types.DNSRecord, query, zone string) bool {
	if zone == "" {
		return false
	}
	if query == zone {
		return true
	}
	return record.Wildcard && wildcardDepthAllowed(record, zone) && strings.HasSuffix(query, "."+zone)
}

func wildcardDepthAllowed(record *types.DNSRecord, zone string) bool {
	if strings.Contains(zone, ".") {
		return true
	}
	return record.ShortWildcard
}

func normalizeDNSName(name string) string {
	name = strings.TrimSpace(name)
	for strings.HasSuffix(name, ".") {
		name = strings.TrimSuffix(name, ".")
		name = strings.TrimSpace(name)
	}
	return strings.ToLower(name)
}

// CheckIfPlainDomain ...
func CheckIfPlainDomain(s string) bool {
	return strings.Contains(s, ".")
}
