package client

import (
	"strings"

	"github.com/tunnels-is/tunnels/types"
)

func GetDomainAndSubDomain(name string) (domain, subdomain string) {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	if name == "" {
		return "", ""
	}

	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		// A single label has no public suffix. It is a local zone name.
		return parts[0], ""
	}
	if len(parts) == 2 {
		return name, ""
	}

	domain = strings.Join(parts[len(parts)-3:], ".")
	subdomain = strings.Join(parts[:len(parts)-3], ".")
	return
}

// DNSAMapping returns the local record for fullDomain.
// The longest matching record wins. An exact name always matches. A wildcard
// record also matches names strictly under it, so a one-label zone such as
// "loganisgay" covers "host.loganisgay" when Wildcard is set. Those names
// exist only in this resolver.
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

func CheckIfPlainDomain(s string) bool {
	return strings.Contains(s, ".")
}
