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
		if zone == "" {
			continue
		}
		if query != zone && !(record.Wildcard && strings.HasSuffix(query, "."+zone)) {
			continue
		}
		if len(zone) > bestLen {
			best = records[i]
			bestLen = len(zone)
		}
	}
	return best
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
