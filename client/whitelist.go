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

func reloadWhiteLists(sleep bool) {
	defer RecoverAndLog()
	if sleep {
		time.Sleep(1 * time.Hour)
	}
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

	var err error
	var listBytes []byte
	state := STATE.Load()
	lowerTag := strings.ToLower(wl.Tag)

	if time.Since(wl.LastDownload).Hours() > 24 && wl.URL != "" {
		listBytes, err = downloadWhiteList(wl.URL)
		if err != nil {
			ERROR("Could not download whitelist", wl.URL, err)
			listBytes, err = os.ReadFile(state.WhiteListPath + lowerTag)
			if err != nil {
				ERROR("Could not read from disk or download whitelist", wl.URL, err)
				return
			}
		}
	} else if wl.Tag != "" {
		listBytes, err = os.ReadFile(state.WhiteListPath + lowerTag)
		if err != nil {
			ERROR("Could not read whitelist", lowerTag, err)
			listBytes, err = downloadWhiteList(wl.URL)
			if err != nil {
				ERROR("Could not read from disk or download whitelist", wl.URL, err)
				return
			}
		}
	}

	if len(listBytes) == 0 {
		ERROR("No bytes in DNS whitelist: ", wl.URL, lowerTag)
		return
	}

	err = os.WriteFile(state.WhiteListPath+lowerTag, listBytes, 0o666)
	if err != nil {
		ERROR("Could not save", wl.URL, err)
		return
	}

	wl.Count = 0
	var badLines int
	buff := bytes.NewBuffer(listBytes)
	scanner := bufio.NewScanner(buff)
	for scanner.Scan() {
		d := scanner.Text()
		if CheckIfPlainDomain(d) {
			if wl.Enabled {
				nm.Store(d, true)
			}
			wl.Count++
		} else {
			badLines++
		}
	}

	wl.LastDownload = time.Now()
	if badLines > 0 {
		DEBUG(badLines, " invalid lines in list: ", wl.URL)
	}
	config.DNSWhiteLists[index] = wl
}

func downloadWhiteList(url string) ([]byte, error) {
	defer RecoverAndLog()
	if !CheckIfURL(url) {
		return nil, nil
	}

	DEBUG("Downloading Whitelist: ", url)
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

func GetDefaultWhiteLists() []*BlockList {
	wl := []*BlockList{
		// Add default whitelists here if needed
		// Example:
		// {
		// 	Tag: "CommonServices",
		// 	URL: "https://example.com/whitelist.txt",
		// },
	}

	dlt := time.Now().AddDate(-2, 0, 0)
	for i := range wl {
		wl[i].LastDownload = dlt
	}

	return wl
}
