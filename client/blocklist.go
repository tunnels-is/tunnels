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
)

var listReloadMu sync.Mutex

var domainDot = []byte(".")

const (
	maxDNSListSize    = 128 * 1024 * 1024
	dnsListScanBuf    = 64 * 1024
	dnsListMaxLineLen = 1024 * 1024
)

var listHTTPClient = &http.Client{Timeout: 5 * time.Minute}

func reloadBlockLists(sleep bool) {
	reloadBlockListsEx(sleep, false)
}

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

	// Load each list into its own map (no concurrent writes), then merge once.
	// Avoids xsync concurrent-map overhead which dominated retained heap (~250MB+).
	partials := make([]*DomainSet, len(config.DNSBlockLists))
	wg := new(sync.WaitGroup)
	for i := range config.DNSBlockLists {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			partials[i] = processBlockList(i, force)
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

	DEBUG("finished updating blocklists, domains=", final.Len())
	DNSBlockList.Store(final)
	err := writeConfigToDisk()
	if err != nil {
		ERROR("unable to write config to disk post blocklist update", err)
	}
}

// processBlockList loads one list into a private DomainSet (or nil on failure).
func processBlockList(index int, force bool) *DomainSet {
	defer RecoverAndLog()
	config := CONFIG.Load()
	bl := config.DNSBlockLists[index]
	if bl == nil {
		return nil
	}

	state := STATE.Load()
	lowerTag := strings.ToLower(bl.Tag)
	path := state.BlockListPath + lowerTag

	if (force || time.Since(bl.LastDownload).Hours() > 24) && bl.URL != "" {
		if err := downloadListToFile(bl.URL, path); err != nil {
			ERROR("Could not download bocklist", bl.URL, err)
			if !fileExistsNonEmpty(path) {
				ERROR("Could not read from disk or download blocklist", bl.URL, err)
				return nil
			}
		}
	} else if bl.Tag != "" {
		if !fileExistsNonEmpty(path) {
			if bl.URL == "" {
				ERROR("No bytes in DNS blocklist: ", bl.URL, lowerTag)
				return nil
			}
			if err := downloadListToFile(bl.URL, path); err != nil {
				ERROR("Could not read from disk or download blocklist", bl.URL, err)
				return nil
			}
		}
	} else {
		return nil
	}

	if !fileExistsNonEmpty(path) {
		ERROR("No bytes in DNS blocklist: ", bl.URL, lowerTag)
		return nil
	}

	var capHint int
	if fi, err := os.Stat(path); err == nil {
		capHint = estimateDomainCapacity(fi.Size())
	}
	set := NewDomainSet(capHint)
	count, badLines, err := loadDomainsFromFile(path, bl.Enabled, set)
	if err != nil {
		ERROR("Could not parse blocklist", path, err)
		return nil
	}

	bl.Count = count
	bl.LastDownload = time.Now()
	if badLines > 0 {
		DEBUG(badLines, " invalid lines in list: ", bl.URL)
	}
	config.DNSBlockLists[index] = bl
	if !bl.Enabled {
		return nil // counted but not stored
	}
	return set
}

func fileExistsNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

// loadDomainsFromFile streams a list file line-by-line without loading the whole file.
func loadDomainsFromFile(path string, enabled bool, set *DomainSet) (count, bad int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	return loadDomainsFromReader(f, enabled, set)
}

func loadDomainsFromReader(r io.Reader, enabled bool, set *DomainSet) (count, bad int, err error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, dnsListScanBuf)
	scanner.Buffer(buf, dnsListMaxLineLen)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 || line[0] == '#' {
			bad++
			continue
		}
		if !bytes.Contains(line, domainDot) {
			bad++
			continue
		}
		count++
		if enabled && set != nil {
			set.Add(string(line))
		}
	}
	if err := scanner.Err(); err != nil {
		return count, bad, err
	}
	return count, bad, nil
}

// downloadListToFile streams a remote list straight to path (capped at maxDNSListSize).
func downloadListToFile(url, path string) error {
	defer RecoverAndLog()
	if !CheckIfURL(url) {
		return fmt.Errorf("invalid list URL")
	}

	DEBUG("Downloading Blocklist: ", url)
	start := time.Now()
	defer func() {
		DEBUG(url, " : Download time > ", time.Since(start).Seconds(), " seconds")
	}()

	var tries int
retry:
	resp, err := listHTTPClient.Get(url)
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
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
			return fmt.Errorf("failed to download list: %d %s ", resp.StatusCode, err)
		}
		return fmt.Errorf("failed to download list:  %s ", err)
	}
	defer resp.Body.Close()

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}

	written, copyErr := io.Copy(f, io.LimitReader(resp.Body, maxDNSListSize))
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if written == 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("empty list body")
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
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

func CheckIfURL(s string) bool {
	if strings.HasPrefix(s, "https://") {
		return true
	}
	if strings.HasPrefix(s, "http://") {
		ERROR("list URLs must use https:// — refusing to download: ", s)
	}
	return false
}
