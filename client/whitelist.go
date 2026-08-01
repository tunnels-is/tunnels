package client

import (
	"os"
	"strings"
	"sync"
	"time"
)

func reloadWhiteLists(sleep bool) {
	defer RecoverAndLog()
	if sleep {
		time.Sleep(1 * time.Hour)
	}
	listReloadMu.Lock()
	defer listReloadMu.Unlock()
	config := CONFIG.Load()

	if config.DisableBlockLists {
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

	partials := make([]*DomainSet, len(config.DNSWhiteLists))
	wg := new(sync.WaitGroup)
	for i := range config.DNSWhiteLists {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			partials[i] = processWhiteList(i)
		}(i)
	}
	wg.Wait()

	totalCap := 0
	for _, p := range partials {
		if p != nil {
			totalCap += p.Len()
		}
	}
	final := NewDomainSet(totalCap)
	for _, p := range partials {
		final.MergeFrom(p)
	}

	DEBUG("finished updating whitelists, domains=", final.Len())
	DNSWhiteList.Store(final)
	err := writeConfigToDisk()
	if err != nil {
		ERROR("unable to write config to disk post whitelist update", err)
	}
}

func processWhiteList(index int) *DomainSet {
	defer RecoverAndLog()
	config := CONFIG.Load()
	wl := config.DNSWhiteLists[index]
	if wl == nil {
		return nil
	}

	state := STATE.Load()
	lowerTag := strings.ToLower(wl.Tag)
	path := state.WhiteListPath + lowerTag

	if time.Since(wl.LastDownload).Hours() > 24 && wl.URL != "" {
		if err := downloadListToFile(wl.URL, path); err != nil {
			ERROR("Could not download whitelist", wl.URL, err)
			if !fileExistsNonEmpty(path) {
				ERROR("Could not read from disk or download whitelist", wl.URL, err)
				return nil
			}
		}
	} else if wl.Tag != "" {
		if !fileExistsNonEmpty(path) {
			if wl.URL == "" {
				ERROR("No bytes in DNS whitelist: ", wl.URL, lowerTag)
				return nil
			}
			if err := downloadListToFile(wl.URL, path); err != nil {
				ERROR("Could not read from disk or download whitelist", wl.URL, err)
				return nil
			}
		}
	} else {
		return nil
	}

	if !fileExistsNonEmpty(path) {
		ERROR("No bytes in DNS whitelist: ", wl.URL, lowerTag)
		return nil
	}

	var capHint int
	if fi, err := os.Stat(path); err == nil {
		capHint = estimateDomainCapacity(fi.Size())
	}
	set := NewDomainSet(capHint)
	count, badLines, err := loadDomainsFromFile(path, wl.Enabled, set)
	if err != nil {
		ERROR("Could not parse whitelist", path, err)
		return nil
	}

	wl.Count = count
	wl.LastDownload = time.Now()
	if badLines > 0 {
		DEBUG(badLines, " invalid lines in list: ", wl.URL)
	}
	config.DNSWhiteLists[index] = wl
	if !wl.Enabled {
		return nil
	}
	return set
}

func GetDefaultWhiteLists() []*BlockList {
	wl := []*BlockList{}

	dlt := time.Now().AddDate(-2, 0, 0)
	for i := range wl {
		wl[i].LastDownload = dlt
	}

	return wl
}
