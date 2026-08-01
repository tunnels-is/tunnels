package client

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

// listReloadMu serializes the block-list and white-list reloaders. They run as
// separate hourly goroutines and both mutate the shared config
// (DNSBlockLists/DNSWhiteLists) and persist it, so without this they race on the
// shared struct and on the config file.
var listReloadMu sync.Mutex

func reloadBlockLists(sleep bool) {
	reloadBlockListsEx(sleep, false)
}

// forceReloadBlockLists re-downloads every configured list, ignoring the
// usual 24h cache window. Used by the UI "Update" action.
func forceReloadBlockLists() {
	reloadBlockListsEx(false, true)
}

func reloadBlockListsEx(sleep bool, force bool) {
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

	if len(config.DNSBlockLists) == 0 {
		config.DNSBlockLists = GetDefaultBlockLists()
	}
	badList := false
	for _, v := range config.DNSBlockLists {
		if v == nil {
			badList = true
			break
		}
		if v.URL == "" && v.Tag == "" {
			badList = true
		}
	}
	if badList {
		config.DNSBlockLists = GetDefaultBlockLists()
	}
	newMap := xsync.NewMapOf[string, bool]()

	wg := new(sync.WaitGroup)
	for i := range config.DNSBlockLists {
		wg.Add(1)
		go processBlockList(i, wg, newMap, force)
	}
	wg.Wait()

	DEBUG("finished updating blocklists")
	DNSBlockList.Store(&newMap)
	err := writeConfigToDisk()
	if err != nil {
		ERROR("unable to write config to disk post blocklist update", err)
	}
}

func processBlockList(index int, wg *sync.WaitGroup, nm *xsync.MapOf[string, bool], force bool) {
	defer func() {
		wg.Done()
	}()
	defer RecoverAndLog()
	config := CONFIG.Load()
	bl := config.DNSBlockLists[index]
	if bl == nil {
		return
	}

	var err error
	var listBytes []byte
	state := STATE.Load()
	lowerTag := strings.ToLower(bl.Tag)

	// force=true (manual Update) re-downloads even when LastDownload is fresh.
	if (force || time.Since(bl.LastDownload).Hours() > 24) && bl.URL != "" {
		listBytes, err = downloadList(bl.URL)
		if err != nil {
			ERROR("Could not download bocklist", bl.URL, err)
			listBytes, err = os.ReadFile(state.BlockListPath + lowerTag)
			if err != nil {
				ERROR("Could not read from disk or download blocklist", bl.URL, err)
				return
			}
		}
	} else if bl.Tag != "" {
		listBytes, err = os.ReadFile(state.BlockListPath + lowerTag)
		if err != nil {
			ERROR("Could not read blocklist", lowerTag, err)
			listBytes, err = downloadList(bl.URL)
			if err != nil {
				ERROR("Could not read from disk or download blocklist", bl.URL, err)
				return
			}
		}
	}

	if len(listBytes) == 0 {
		ERROR("No bytes in DNS blocklist: ", bl.URL, lowerTag)
		return
	}

	err = os.WriteFile(state.BlockListPath+lowerTag, listBytes, 0o600)
	if err != nil {
		ERROR("Could not save", bl.URL, err)
		return
	}

	bl.Count = 0
	var badLines int
	buff := bytes.NewBuffer(listBytes)
	scanner := bufio.NewScanner(buff)
	for scanner.Scan() {
		d := scanner.Text()
		if CheckIfPlainDomain(d) {
			if bl.Enabled {
				nm.Store(d, true)
			}
			bl.Count++
		} else {
			badLines++
		}
	}

	bl.LastDownload = time.Now()
	if badLines > 0 {
		DEBUG(badLines, " invalid lines in list: ", bl.URL)
	}
	config.DNSBlockLists[index] = bl
}

func downloadList(url string) ([]byte, error) {
	defer RecoverAndLog()
	if !CheckIfURL(url) {
		return nil, nil
	}

	DEBUG("Downloading Blocklist: ", url)
	start := time.Now()
	defer func() {
		DEBUG(url, " : Download time > ", time.Since(start).Seconds(), " seconds")
	}()

	var tries int

retry:
	resp, err := listHTTPClient.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		if tries < 5 {
			time.Sleep(5 * time.Second)
			tries++
			if tries < 5 {
				DEBUG("Unable to load list (retrying): ", url)
				goto retry
			}
		}
		if resp != nil {
			return nil, fmt.Errorf("failed to download list: %d %s ", resp.StatusCode, err)
		} else {
			return nil, fmt.Errorf("failed to download list:  %s ", err)
		}
	}
	defer resp.Body.Close()

	bb, err := io.ReadAll(io.LimitReader(resp.Body, maxDNSListSize))
	if err != nil {
		return nil, err
	}

	return bb, nil
}

func GetDefaultBlockLists() []*BlockList {
	bl := []*BlockList{
		{
			Tag: "Ads",
			URL: "https://raw.githubusercontent.com/n00bady/bluam/master/dns/merged/ads",
		},
		{
			Tag: "AdultContent",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/adult",
		},
		{
			Tag: "CryptoCurrency",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/crypto",
		},
		{
			Tag: "Drugs",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/drugs",
		},
		{
			Tag: "FakeNews",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/fakenews",
		},
		{
			Tag: "Fraud",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/fraud",
		},
		{
			Tag: "Gambling",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/gambling",
		},
		{
			Tag: "Malware",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/malware",
		},
		{
			Tag: "SocialMedia",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/socialmedia",
		},
		{
			Tag: "Surveillance",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/surveillance",
		},
	}

	dlt := time.Now().AddDate(-2, 0, 0)
	for i := range bl {
		bl[i].LastDownload = dlt
	}

	return bl
}

// maxDNSListSize caps a downloaded block/whitelist so a huge (or malicious)
// list cannot exhaust memory; merged lists are typically tens of MB.
const maxDNSListSize = 128 * 1024 * 1024

// listHTTPClient downloads block/whitelists. The default http.Client has no
// timeout; a stalled download would hang the reload goroutine forever.
var listHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// CheckIfURL accepts only https:// list URLs. Plain http:// is rejected: a
// MITM on an http whitelist could inject domains, and whitelisted domains
// bypass the blocklists entirely.
func CheckIfURL(s string) bool {
	if strings.HasPrefix(s, "https://") {
		return true
	}
	if strings.HasPrefix(s, "http://") {
		ERROR("list URLs must use https:// — refusing to download: ", s)
	}
	return false
}
