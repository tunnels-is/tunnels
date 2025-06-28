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

func reloadBlockLists(sleep bool) {
	defer RecoverAndLogToFile()
	if sleep {
		time.Sleep(1 * time.Hour)
	}
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
		if v.URL == "" && v.Disk == "" {
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
		go processBlockList(i, wg, newMap)
	}
	wg.Wait()

	DEBUG("finished updating blocklists")
	DNSBlockList.Store(&newMap)
	err := writeConfigToDisk()
	if err != nil {
		ERROR("unable to write config to disk post blocklist update", err)
	}
}

func processBlockList(index int, wg *sync.WaitGroup, nm *xsync.MapOf[string, bool]) {
	defer func() {
		wg.Done()
	}()
	defer RecoverAndLogToFile()
	config := CONFIG.Load()
	bl := config.DNSBlockLists[index]
	if bl == nil {
		return
	}

	var err error
	var listBytes []byte
	state := STATE.Load()
	fname := state.BlockListPath + bl.Tag + ".txt"

	if time.Since(bl.LastDownload).Hours() > 24 && bl.URL != "" {
		if CheckIfURL(bl.URL) {
			listBytes, err = downloadList(bl.URL)
			if err != nil {
				ERROR("Could not download", bl.URL, err)
				return
			}
			err = os.WriteFile(fname, listBytes, 0o666)
			if err != nil {
				ERROR("Could not save", bl.URL, err)
				return
			}
			bl.Disk = fname
		}
	} else if bl.Disk != "" {
		listBytes, err = os.ReadFile(bl.Disk)
		if err != nil {
			ERROR("Could not read blocklist", bl.Disk, err)
			listBytes, err = downloadList(bl.URL)
			if err != nil {
				ERROR("Could not read from disk or download", bl.URL, err)
				return
			}

			err = os.WriteFile(fname, listBytes, 0o666)
			if err != nil {
				ERROR("Could not save", bl.URL, err)
				return
			}
			bl.Disk = fname
		}
	}

	if len(listBytes) == 0 {
		ERROR("No bytes in DNS blocklist: ", bl.URL, bl.Disk)
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
	defer RecoverAndLogToFile()

	DEBUG("Downloading Blocklist: ", url)
	start := time.Now()
	defer func() {
		DEBUG(url, " : Download time > ", time.Since(start).Seconds(), " seconds")
	}()

	var tries int

retry:
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
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

	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return bb, nil
}

func GetDefaultBlockLists() []*BlockList {
	bl := []*BlockList{
		{
			Tag: "Ads",
			URL: "https://raw.githubusercontent.com/n00bady/bluam/master/dns/merged/ads.txt",
		},
		{
			Tag: "AdultContent",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/adult.txt",
		},
		{
			Tag: "CryptoCurrency",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/crypto.txt",
		},
		{
			Tag: "Drugs",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/drugs.txt",
		},
		{
			Tag: "FakeNews",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/fakenews.txt",
		},
		{
			Tag: "Fraud",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/fraud.txt",
		},
		{
			Tag: "Gambling",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/gambling.txt",
		},
		{
			Tag: "Malware",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/malware.txt",
		},
		{
			Tag: "SocialMedia",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/socialmedia.txt",
		},
		{
			Tag: "Surveillance",
			URL: "https://github.com/n00bady/bluam/raw/master/dns/merged/surveillance.txt",
		},
	}

	dlt := time.Now().AddDate(-2, 0, 0)
	for i := range bl {
		bl[i].LastDownload = dlt
	}

	return bl
}

func CheckIfURL(s string) bool {
	switch {
	case strings.HasPrefix(s, "http"):
		return true
	case strings.HasPrefix(s, "https"):
		return true
	default:
		return false
	}
}
