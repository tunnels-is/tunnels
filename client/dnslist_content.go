package client

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// maxCustomDNSListContent caps the editable custom list size (UI + API).
// Request bodies are already limited to 2 MiB; this is an extra safety bound.
const maxCustomDNSListContent = 1 << 20 // 1 MiB

// DNSListContent is the get/set payload for the local custom block/white list files.
type DNSListContent struct {
	// Kind is "blocklist" or "whitelist".
	Kind string
	// Content is the raw file text (one domain per line).
	Content string
	// Count is the number of valid domain lines after parse (set on response).
	Count int
}

func normalizeDNSListKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "blocklist", "block", "dnsblocklists":
		return "blocklist", nil
	case "whitelist", "white", "dnswhitelists":
		return "whitelist", nil
	default:
		return "", fmt.Errorf("kind must be \"blocklist\" or \"whitelist\"")
	}
}

func customDNSListDirAndEnsure(kind string) (dir string, starter string, ensureCfg func(*configV2) bool, err error) {
	state := STATE.Load()
	if state == nil {
		return "", "", nil, fmt.Errorf("state not initialized")
	}
	switch kind {
	case "blocklist":
		return state.BlockListPath, customBlockListFileContent, ensureCustomBlockListInConfig, nil
	case "whitelist":
		return state.WhiteListPath, customWhiteListFileContent, ensureCustomWhiteListInConfig, nil
	default:
		return "", "", nil, fmt.Errorf("kind must be \"blocklist\" or \"whitelist\"")
	}
}

// getCustomDNSListContent returns the on-disk custom list file for kind.
func getCustomDNSListContent(kind string) (*DNSListContent, error) {
	kind, err := normalizeDNSListKind(kind)
	if err != nil {
		return nil, err
	}
	dir, starter, _, err := customDNSListDirAndEnsure(kind)
	if err != nil {
		return nil, err
	}
	if err := ensureCustomDNSListFile(dir, starter); err != nil {
		return nil, err
	}
	path, err := listFilePath(dir, customDNSListTag)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	count := 0
	if set, c, _, perr := loadDomainSetFromReader(strings.NewReader(string(data)), 0); perr == nil {
		count = c
		_ = set
	}
	return &DNSListContent{Kind: kind, Content: string(data), Count: count}, nil
}

// setCustomDNSListContent writes the custom list file and hot-reloads that list
// into the in-memory catalog without re-downloading remote lists.
func setCustomDNSListContent(kind, content string) (*DNSListContent, error) {
	kind, err := normalizeDNSListKind(kind)
	if err != nil {
		return nil, err
	}
	if len(content) > maxCustomDNSListContent {
		return nil, fmt.Errorf("list content too large (max %d bytes)", maxCustomDNSListContent)
	}

	dir, starter, ensureCfg, err := customDNSListDirAndEnsure(kind)
	if err != nil {
		return nil, err
	}

	listReloadMu.Lock()
	defer listReloadMu.Unlock()

	config := CONFIG.Load()
	if config == nil {
		return nil, fmt.Errorf("config not initialized")
	}
	if ensureCfg != nil {
		_ = ensureCfg(config)
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path, err := listFilePath(dir, customDNSListTag)
	if err != nil {
		return nil, err
	}

	// Empty save re-seeds the starter template so the file stays non-empty for
	// reload logic and users still get format hints.
	toWrite := content
	if strings.TrimSpace(toWrite) == "" {
		toWrite = starter
	}
	if err := os.WriteFile(path, []byte(toWrite), 0o600); err != nil {
		return nil, err
	}

	var capHint int
	if fi, err := os.Stat(path); err == nil {
		capHint = estimateDomainCapacity(fi.Size())
	}
	loaded, count, _, err := loadDomainSetFromFile(path, capHint)
	if err != nil {
		return nil, fmt.Errorf("saved but could not parse list: %w", err)
	}

	// Update Count / LastDownload on the custom config entry.
	var lists []*BlockList
	switch kind {
	case "blocklist":
		lists = config.DNSBlockLists
	case "whitelist":
		lists = config.DNSWhiteLists
	}
	for _, l := range lists {
		if l != nil && strings.EqualFold(l.Tag, customDNSListTag) {
			l.Count = count
			l.LastDownload = time.Now()
			break
		}
	}

	if err := applyCustomListToCatalog(kind, config, loaded); err != nil {
		return nil, err
	}
	if err := writeConfigToDisk(); err != nil {
		ERROR("unable to write config after custom list update: ", err)
	}

	return &DNSListContent{Kind: kind, Content: toWrite, Count: count}, nil
}

// applyCustomListToCatalog replaces the custom entry in the live catalog while
// keeping other already-loaded lists (no remote re-download).
func applyCustomListToCatalog(kind string, config *configV2, customSet *DomainSet) error {
	var lists []*BlockList
	var prev *DomainCatalog
	var store func(*DomainCatalog)

	switch kind {
	case "blocklist":
		lists = config.DNSBlockLists
		prev = DNSBlockList.Load()
		store = func(c *DomainCatalog) { DNSBlockList.Store(c) }
	case "whitelist":
		lists = config.DNSWhiteLists
		prev = DNSWhiteList.Load()
		store = func(c *DomainCatalog) { DNSWhiteList.Store(c) }
	default:
		return fmt.Errorf("invalid kind")
	}

	if config.DisableBlockLists {
		store(EmptyCatalog())
		return nil
	}

	// Whether custom is enabled in config (default true if missing).
	customEnabled := true
	customTag := customDNSListTag
	for _, l := range lists {
		if l != nil && strings.EqualFold(l.Tag, customDNSListTag) {
			customEnabled = l.Enabled
			customTag = l.Tag
			break
		}
	}

	// Preserve every already-loaded non-custom list from the previous catalog.
	prevByTag := prev.Snapshot()
	if prevByTag == nil {
		prevByTag = map[string]*DomainSet{}
	}
	for k := range prevByTag {
		if strings.EqualFold(k, customDNSListTag) {
			delete(prevByTag, k)
		}
	}

	tags := make([]string, 0, len(prevByTag)+1)
	sets := make([]*DomainSet, 0, len(prevByTag)+1)
	if customEnabled {
		tags = append(tags, customTag)
		sets = append(sets, customSet)
	}
	for tag, set := range prevByTag {
		tags = append(tags, tag)
		sets = append(sets, set)
	}
	store(NewCatalog(tags, sets))
	return nil
}
