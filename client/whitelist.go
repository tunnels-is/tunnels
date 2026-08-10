package client

import (
	"os"
	"strings"
	"sync"
	"time"
)

// customWhiteListFileContent is written when whitelists/custom is missing.
// Comments keep the file non-empty so local load does not treat it as absent.
const customWhiteListFileContent = `# Tunnels custom DNS whitelist
# Domains listed here always resolve, even if they appear on a block list.
# One domain per line. Lines starting with # are ignored.
#
# Example:
# example.com
# cdn.mycompany.com
`

type listLoadResult struct {
	tag          string
	set          *DomainSet
	count        int
	lastDownload time.Time
	didReuse     bool
	updateMeta   bool
}

type indexedListResult struct {
	i int
	r listLoadResult
}

func reloadWhiteLists(sleep bool) {
	reloadWhiteListsEx(sleep, false)
}

func forceReloadWhiteLists() {
	reloadWhiteListsEx(false, true)
}

func reloadWhiteListsEx(sleep bool, force bool) {
	defer RecoverAndLog()
	if sleep {
		time.Sleep(1 * time.Hour)
	}
	listReloadMu.Lock()
	defer listReloadMu.Unlock()
	config := CONFIG.Load()
	state := STATE.Load()

	if err := ensureCustomWhiteListFile(state.WhiteListPath); err != nil {
		ERROR("unable to create custom whitelist file: ", err)
	}
	configChanged := ensureCustomWhiteListInConfig(config)

	if config.DisableBlockLists {
		DNSWhiteList.Store(EmptyCatalog())
		if configChanged {
			if err := writeConfigToDisk(); err != nil {
				ERROR("unable to write config to disk post whitelist ensure", err)
			}
		}
		return
	}

	if len(config.DNSWhiteLists) == 0 {
		config.DNSWhiteLists = GetDefaultWhiteLists()
		configChanged = true
	}
	badList := false
	for _, v := range config.DNSWhiteLists {
		if v == nil {
			badList = true
			break
		}
		if v.URL == "" && v.Tag == "" {
			badList = true
		}
	}
	if badList {
		config.DNSWhiteLists = GetDefaultWhiteLists()
		configChanged = true
	}
	// Re-ensure after any full replacement of the list slice.
	if ensureCustomWhiteListInConfig(config) {
		configChanged = true
	}

	prev := DNSWhiteList.Load()
	prevByTag := prev.Snapshot()

	lists := config.DNSWhiteLists
	n := len(lists)
	if n == 0 {
		DNSWhiteList.Store(EmptyCatalog())
		if configChanged {
			if err := writeConfigToDisk(); err != nil {
				ERROR("unable to write config to disk post whitelist ensure", err)
			}
		}
		return
	}

	ch := make(chan indexedListResult, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int, wl *BlockList) {
			defer wg.Done()
			ch <- indexedListResult{i: i, r: processWhiteList(wl, force, prevByTag)}
		}(i, lists[i])
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	results := make([]listLoadResult, n)
	for ir := range ch {
		results[ir.i] = ir.r
	}

	tags := make([]string, n)
	sets := make([]*DomainSet, n)
	for i := 0; i < n; i++ {
		r := results[i]
		tags[i] = r.tag
		sets[i] = r.set
		if r.updateMeta && lists[i] != nil {
			lists[i].Count = r.count
			if !r.didReuse {
				lists[i].LastDownload = r.lastDownload
			}
		}
	}

	cat := NewCatalog(tags, sets)
	DEBUG("finished updating whitelists, domains=", cat.Len(), " lists=", cat.ListCount())
	DNSWhiteList.Store(cat)
	err := writeConfigToDisk()
	if err != nil {
		ERROR("unable to write config to disk post whitelist update", err)
	}
}

func processWhiteList(wl *BlockList, force bool, prevByTag map[string]*DomainSet) listLoadResult {
	defer RecoverAndLog()
	if wl == nil {
		return listLoadResult{}
	}
	tag := wl.Tag
	if !wl.Enabled {
		return listLoadResult{tag: tag}
	}

	state := STATE.Load()
	path, pathErr := listFilePath(state.WhiteListPath, wl.Tag)
	if pathErr != nil {
		ERROR("Invalid DNS whitelist tag, refusing path: ", wl.Tag, pathErr)
		return listLoadResult{tag: tag}
	}
	lowerTag := strings.ToLower(wl.Tag)

	// Local custom list: recreate the starter file if someone deleted it.
	if wl.URL == "" && strings.EqualFold(wl.Tag, customDNSListTag) {
		if err := ensureCustomWhiteListFile(state.WhiteListPath); err != nil {
			ERROR("unable to create custom whitelist file: ", err)
		}
	}

	downloaded := false
	if (force || time.Since(wl.LastDownload).Hours() > 24) && wl.URL != "" {
		if err := downloadListToFile(wl.URL, path); err != nil {
			ERROR("Could not download whitelist", wl.URL, err)
			if !fileExistsNonEmpty(path) {
				ERROR("Could not read from disk or download whitelist", wl.URL, err)
				return listLoadResult{tag: tag}
			}
		} else {
			downloaded = true
		}
	} else if wl.Tag != "" {
		if !fileExistsNonEmpty(path) {
			if wl.URL == "" {
				ERROR("No bytes in DNS whitelist: ", wl.URL, lowerTag)
				return listLoadResult{tag: tag}
			}
			if err := downloadListToFile(wl.URL, path); err != nil {
				ERROR("Could not read from disk or download whitelist", wl.URL, err)
				return listLoadResult{tag: tag}
			}
			downloaded = true
		}
	} else {
		return listLoadResult{tag: tag}
	}

	if !fileExistsNonEmpty(path) {
		ERROR("No bytes in DNS whitelist: ", wl.URL, lowerTag)
		return listLoadResult{tag: tag}
	}

	if !force && !downloaded && prevByTag != nil {
		if old := prevByTag[tag]; old != nil && old.Len() > 0 {
			return listLoadResult{
				tag:        tag,
				set:        old,
				count:      old.Len(),
				didReuse:   true,
				updateMeta: true,
			}
		}
	}

	var capHint int
	if fi, err := os.Stat(path); err == nil {
		capHint = estimateDomainCapacity(fi.Size())
	}
	loaded, count, badLines, err := loadDomainSetFromFile(path, capHint)
	if err != nil {
		ERROR("Could not parse whitelist", path, err)
		return listLoadResult{tag: tag}
	}
	if badLines > 0 {
		DEBUG(badLines, " invalid lines in list: ", wl.URL)
	}
	return listLoadResult{
		tag:          tag,
		set:          loaded,
		count:        count,
		lastDownload: time.Now(),
		updateMeta:   true,
	}
}

func ensureCustomWhiteListFile(whiteListDir string) error {
	return ensureCustomDNSListFile(whiteListDir, customWhiteListFileContent)
}

// ensureCustomWhiteListInConfig adds the "custom" whitelist (Enabled) when
// no entry with that tag is present. Existing entries are not modified, so a
// user who disabled "custom" stays disabled.
func ensureCustomWhiteListInConfig(config *configV2) bool {
	if config == nil {
		return false
	}
	return ensureCustomDNSListInSlice(&config.DNSWhiteLists)
}

func GetDefaultWhiteLists() []*BlockList {
	return []*BlockList{newCustomDNSList()}
}
