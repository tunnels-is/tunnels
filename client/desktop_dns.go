package client

func UpdateBlockLists() []*BlockList {
	forceReloadBlockLists()
	return CONFIG.Load().DNSBlockLists
}

func UpdateWhiteLists() []*BlockList {
	forceReloadWhiteLists()
	return CONFIG.Load().DNSWhiteLists
}

func GetDNSListContent(kind string) (*DNSListContent, error) {
	return getCustomDNSListContent(kind)
}

func SetDNSListContent(kind, content string) (*DNSListContent, error) {
	return setCustomDNSListContent(kind, content)
}

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
