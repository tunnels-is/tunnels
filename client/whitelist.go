package client

import (
	"os"
	"strings"
	"sync"
	"time"
)


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
	defer RecoverAndLog()
	if sleep {
		time.Sleep(1 * time.Hour)
	}
	listReloadMu.Lock()
	defer listReloadMu.Unlock()
	config := CONFIG.Load()

	if config.DisableBlockLists {
		DNSWhiteList.Store(EmptyCatalog())
		return
	}

	if len(config.DNSWhiteLists) == 0 {
		config.DNSWhiteLists = GetDefaultWhiteLists()
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
	}

	prev := DNSWhiteList.Load()
	prevByTag := prev.Snapshot()


	lists := config.DNSWhiteLists
	n := len(lists)
	if n == 0 {
		DNSWhiteList.Store(EmptyCatalog())
		return
	}



	ch := make(chan indexedListResult, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int, wl *BlockList) {
			defer wg.Done()
			ch <- indexedListResult{i: i, r: processWhiteList(wl, prevByTag)}
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


func processWhiteList(wl *BlockList, prevByTag map[string]*DomainSet) listLoadResult {
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

	downloaded := false
	if time.Since(wl.LastDownload).Hours() > 24 && wl.URL != "" {
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

	if !downloaded && prevByTag != nil {
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

func GetDefaultWhiteLists() []*BlockList {
	wl := []*BlockList{}

	dlt := time.Now().AddDate(-2, 0, 0)
	for i := range wl {
		wl[i].LastDownload = dlt
	}

	return wl
}
