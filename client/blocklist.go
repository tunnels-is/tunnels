package client

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var listReloadMu sync.Mutex

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
		DNSBlockList.Store(EmptyCatalog())
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


	prev := DNSBlockList.Load()
	prevByTag := prev.Snapshot()

	lists := config.DNSBlockLists
	n := len(lists)
	if n == 0 {
		DNSBlockList.Store(EmptyCatalog())
		return
	}


	ch := make(chan indexedListResult, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int, bl *BlockList) {
			defer wg.Done()
			ch <- indexedListResult{i: i, r: processBlockList(bl, force, prevByTag)}
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
	var nReuse, nLoad, nSkip int
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
		bl := lists[i]
		if bl == nil || !bl.Enabled {
			nSkip++
			continue
		}
		if r.didReuse {
			nReuse++
		} else if r.set != nil {
			nLoad++
		} else {
			nSkip++
		}
	}

	cat := NewCatalog(tags, sets)
	DNSBlockList.Store(cat)

	DEBUG("finished updating blocklists, domains=", cat.Len(),
		" lists=", cat.ListCount(), " loaded=", nLoad, " reused=", nReuse, " skipped=", nSkip)

	err := writeConfigToDisk()
	if err != nil {
		ERROR("unable to write config to disk post blocklist update", err)
	}
}


func processBlockList(bl *BlockList, force bool, prevByTag map[string]*DomainSet) listLoadResult {
	defer RecoverAndLog()
	if bl == nil {
		return listLoadResult{}
	}
	tag := bl.Tag
	if !bl.Enabled {
		return listLoadResult{tag: tag}
	}

	state := STATE.Load()
	path, pathErr := listFilePath(state.BlockListPath, bl.Tag)
	if pathErr != nil {
		ERROR("Invalid DNS blocklist tag, refusing path: ", bl.Tag, pathErr)
		return listLoadResult{tag: tag}
	}
	lowerTag := strings.ToLower(bl.Tag)

	downloaded := false
	if (force || time.Since(bl.LastDownload).Hours() > 24) && bl.URL != "" {
		if err := downloadListToFile(bl.URL, path); err != nil {
			ERROR("Could not download bocklist", bl.URL, err)
			if !fileExistsNonEmpty(path) {
				ERROR("Could not read from disk or download blocklist", bl.URL, err)
				return listLoadResult{tag: tag}
			}
		} else {
			downloaded = true
		}
	} else if bl.Tag != "" {
		if !fileExistsNonEmpty(path) {
			if bl.URL == "" {
				ERROR("No bytes in DNS blocklist: ", bl.URL, lowerTag)
				return listLoadResult{tag: tag}
			}
			if err := downloadListToFile(bl.URL, path); err != nil {
				ERROR("Could not read from disk or download blocklist", bl.URL, err)
				return listLoadResult{tag: tag}
			}
			downloaded = true
		}
	} else {
		return listLoadResult{tag: tag}
	}

	if !fileExistsNonEmpty(path) {
		ERROR("No bytes in DNS blocklist: ", bl.URL, lowerTag)
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
		ERROR("Could not parse blocklist", path, err)
		return listLoadResult{tag: tag}
	}
	if badLines > 0 {
		DEBUG(badLines, " invalid lines in list: ", bl.URL)
	}
	return listLoadResult{
		tag:          tag,
		set:          loaded,
		count:        count,
		lastDownload: time.Now(),
		updateMeta:   true,
	}
}

func fileExistsNonEmpty(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

func loadDomainSetFromFile(path string, capHint int) (set *DomainSet, count, bad int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, 0, err
	}
	defer f.Close()
	return loadDomainSetFromReader(f, capHint)
}


func loadDomainSetFromReader(r io.Reader, capHint int) (set *DomainSet, count, bad int, err error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, dnsListScanBuf)
	scanner.Buffer(buf, dnsListMaxLineLen)

	b := newDomainBuilder(capHint)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			bad++
			continue
		}
		if !b.tryAddLine(line) {
			bad++
			continue
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return nil, count, bad, err
	}
	return b.Build(), count, bad, nil
}


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
