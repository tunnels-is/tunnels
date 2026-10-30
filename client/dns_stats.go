package client

import (
	"time"

	"github.com/miekg/dns"
)

// IncrementDNSStats ...
func IncrementDNSStats(domain string, blocked bool, tag string, answers []dns.RR) {
	defer RecoverAndLog()

	tn := time.Now()
	if _, exists := DNSStatsMap.Load(domain); !exists && DNSStatsMap.Size() >= maxDNSStatsEntries {
		return
	}
	dnsint, ok := DNSStatsMap.LoadOrStore(domain, &DNSStats{})
	dnsStats := dnsint.(*DNSStats)

	dnsStats.m.Lock()
	if !ok {
		dnsStats.FirstSeen = tn
	}
	if blocked {
		dnsStats.LastBlocked = tn
		if tag != "" {
			dnsStats.Tag = tag
		}
	} else {
		dnsStats.LastResolved = tn
	}
	dnsStats.Count++
	dnsStats.LastSeen = tn
	for _, v := range answers {
		dnsStats.Answers = append(dnsStats.Answers, v.String())
	}

	if len(dnsStats.Answers) > maxDNSStatsAnswers {
		n := copy(dnsStats.Answers, dnsStats.Answers[len(dnsStats.Answers)-maxDNSStatsAnswers:])
		dnsStats.Answers = dnsStats.Answers[:n]
	}
	dnsStats.m.Unlock()
}
