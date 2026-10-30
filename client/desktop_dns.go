package client

// UpdateBlockLists ...
func UpdateBlockLists() []*BlockList {
	forceReloadBlockLists()
	return CONFIG.Load().DNSBlockLists
}

// UpdateWhiteLists ...
func UpdateWhiteLists() []*BlockList {
	forceReloadWhiteLists()
	return CONFIG.Load().DNSWhiteLists
}

// GetDNSListContent ...
func GetDNSListContent(kind string) (*DNSListContent, error) {
	return getCustomDNSListContent(kind)
}

// SetDNSListContent ...
func SetDNSListContent(kind, content string) (*DNSListContent, error) {
	return setCustomDNSListContent(kind, content)
}

// GetDNSStats ...
func GetDNSStats() map[string]*DNSStats {
	stats := make(map[string]*DNSStats)
	DNSStatsMap.Range(func(key string, value any) bool {
		if s, ok := value.(*DNSStats); ok {
			stats[key] = s
		}
		return true
	})
	return stats
}
