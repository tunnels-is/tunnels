package client

import (
	"strings"

	"github.com/tunnels-is/tunnels/types"
)

func GetDomainAndSubDomain(name string) (domain, subdomain string) {
	parts := strings.Split(name, ".")

	if len(parts) == 2 {
		domain = strings.Join(parts[len(parts)-2:], ".")
	} else if len(parts) > 2 {
		domain = strings.Join(parts[len(parts)-3:], ".")
		subdomain = strings.Join(parts[:len(parts)-3], ".")
	} else {
		return "", ""
	}

	return
}

func DNSAMapping(records []*types.DNSRecord, fullDomain string) *types.DNSRecord {
	domain, subdomain := GetDomainAndSubDomain(fullDomain)
	if domain == "" {
		return nil
	}
	domain = strings.TrimSuffix(domain, ".")

	for i, record := range records {
		if record == nil {
			continue
		}
		if subdomain != "" {
			if record.Domain == subdomain+"."+domain {
				return records[i]
			}
		}

		if record.Domain == domain {
			if subdomain == "" {
				return records[i]
			} else if record.Wildcard {
				return records[i]
			}
		}
	}

	return nil
}

func CheckIfPlainDomain(s string) bool {
	return strings.Contains(s, ".")
}
