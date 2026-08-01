package client

import (
	"strings"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
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
	newMap := xsync.NewMapOf[string, bool]()

	wg := new(sync.WaitGroup)
	for i := range config.DNSWhiteLists {
		wg.Add(1)
		go processWhiteList(i, wg, newMap)
	}
	wg.Wait()

	DEBUG("finished updating whitelists")
	DNSWhiteList.Store(&newMap)
	err := writeConfigToDisk()
	if err != nil {
		ERROR("unable to write config to disk post whitelist update", err)
	}
}

func processWhiteList(index int, wg *sync.WaitGroup, nm *xsync.MapOf[string, bool]) {
	defer func() {
		wg.Done()
	}()
	defer RecoverAndLog()
	config := CONFIG.Load()
	wl := config.DNSWhiteLists[index]
	if wl == nil {
		return
	}

	state := STATE.Load()
	lowerTag := strings.ToLower(wl.Tag)
	path := state.WhiteListPath + lowerTag

	if time.Since(wl.LastDownload).Hours() > 24 && wl.URL != "" {
		if err := downloadListToFile(wl.URL, path); err != nil {
			ERROR("Could not download whitelist", wl.URL, err)
			if !fileExistsNonEmpty(path) {
				ERROR("Could not read from disk or download whitelist", wl.URL, err)
				return
			}
		}
	} else if wl.Tag != "" {
		if !fileExistsNonEmpty(path) {
			if wl.URL == "" {
				ERROR("No bytes in DNS whitelist: ", wl.URL, lowerTag)
				return
			}
			if err := downloadListToFile(wl.URL, path); err != nil {
				ERROR("Could not read from disk or download whitelist", wl.URL, err)
				return
			}
		}
	} else {
		return
	}

	if !fileExistsNonEmpty(path) {
		ERROR("No bytes in DNS whitelist: ", wl.URL, lowerTag)
		return
	}

	count, badLines, err := loadDomainsFromFile(path, wl.Enabled, nm)
	if err != nil {
		ERROR("Could not parse whitelist", path, err)
		return
	}

	wl.Count = count
	wl.LastDownload = time.Now()
	if badLines > 0 {
		DEBUG(badLines, " invalid lines in list: ", wl.URL)
	}
	config.DNSWhiteLists[index] = wl
}

func GetDefaultWhiteLists() []*BlockList {
	wl := []*BlockList{}

	dlt := time.Now().AddDate(-2, 0, 0)
	for i := range wl {
		wl[i].LastDownload = dlt
	}

	return wl
}
