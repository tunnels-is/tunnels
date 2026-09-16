package client

import (
	"strings"

	"github.com/tunnels-is/tunnels/types"
)

func GetDomainAndSubDomain(domain string) (d, s string) {
	parts := strings.Split(domain, ".")

	if len(parts) == 2 {
		d = strings.Join(parts[len(parts)-2:], ".")
	} else if len(parts) > 2 {
		d = strings.Join(parts[len(parts)-3:], ".")
		s = strings.Join(parts[:len(parts)-3], ".")
	} else {
		return "", ""
	}

	return
}

func DNSAMapping(DNS []*types.DNSRecord, fullDomain string) *types.DNSRecord {
	domain, subdomain := GetDomainAndSubDomain(fullDomain)
	if domain == "" {
		return nil
	}
	domain = strings.TrimSuffix(domain, ".")

	for i, record := range DNS {

		if record == nil {
			continue
		}
		if subdomain != "" {
			if record.Domain == subdomain+"."+domain {
				return DNS[i]
			}
		}

		if record.Domain == domain {
			if subdomain == "" {
				return DNS[i]
			} else if record.Wildcard {
				return DNS[i]
			}
		}

	}

	return nil
}

func CheckIfPlainDomain(s string) bool {
	return strings.Contains(s, ".")
}
