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
	"sync/atomic"
	"time"
)

func reloadBlockLists(sleep bool, saveConfig bool) {
	defer func() {
		if sleep {
			time.Sleep(1 * time.Hour)
		}
	}()
	defer RecoverAndLogToFile()
	config := CONFIG.Load()

	if config.DisableBlockLists {
		return
	}

	if len(config.DNSBlockLists) == 0 {
		config.DNSBlockLists = GetDefaultBlockLists()
	}

	newMap := new(sync.Map)

	wg := new(sync.WaitGroup)
	for i := range config.DNSBlockLists {
		wg.Add(1)
		go processBlockList(i, wg, newMap)
	}
	wg.Wait()
	DEBUG("finished updating blocklists")
	DNSBlockList.Store(newMap)
	if saveConfig {
		err := writeConfigToDisk()
		if err != nil {
			ERROR("unable to write config to disk post blocklist update", err)
		}
	}
}

func processBlockList(index int, wg *sync.WaitGroup, nm *sync.Map) {
	defer func() {
		wg.Done()
	}()
	defer RecoverAndLogToFile()
	config := CONFIG.Load()
	bl := config.DNSBlockLists[index].Load()

	var listFile *os.File
	var listBytes []byte
	var badLines int
	var err error

	state := STATE.Load()
	if time.Since(bl.LastDownload).Hours() > 24 {
		if CheckIfURL(bl.URL) {
			listBytes, err = downloadList(bl.URL)
			if err != nil {
				ERROR("Could not download", bl.URL, err)
				return
			}

			_ = RemoveFile(state.BlockListPath + bl.Tag + ".txt")
			listFile, err = CreateFile(state.BlockListPath + bl.Tag + ".txt")
			if err != nil {
				ERROR("Could not save", bl.URL, err)
				return
			}
			defer listFile.Close()
			_, err = listFile.Write(listBytes)
			if err != nil {
				ERROR("unable to write dns block list:", err)
			}
			bl.Disk = listFile.Name()
		}
	} else if bl.Disk != "" {
		listBytes, err = os.ReadFile(bl.Disk)
		if err != nil {
			listBytes, err = downloadList(bl.URL)
			if err != nil {
				ERROR("Could not download", bl.URL, err)
				return
			}

			_ = RemoveFile(state.BlockListPath + bl.Tag + ".txt")
			listFile, err = CreateFile(state.BlockListPath + bl.Tag + ".txt")
			if err != nil {
				ERROR("Could not save", bl.URL, err)
				return
			}
			defer listFile.Close()
			_, err = listFile.Write(listBytes)
			if err != nil {
				ERROR("unable to write dns block list:", err)
			}
			bl.Disk = listFile.Name()
		}
	}

	if len(listBytes) == 0 {
		ERROR("No bytes in DNS blocklist: ", bl.URL)
		return
	}

	bl.Count = 0
	buff := bytes.NewBuffer(listBytes)
	scanner := bufio.NewScanner(buff)
	for scanner.Scan() {
		d := scanner.Text()
		if CheckIfPlainDomain(d) {
			if bl.Enabled {
				nm.Store(d, bl)
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
	config.DNSBlockLists[index].Store(bl)
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

func GetDefaultBlockLists() []atomic.Pointer[BlockList] {
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

	abl := make([]atomic.Pointer[BlockList], len(bl))
	dlt := time.Now().AddDate(-2, 0, 0)
	for i := range bl {
		bl[i].LastDownload = dlt
		abl[i].Store(bl[i])
	}

	return abl
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
