package core

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

func InitBlockListPath() {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	GLOBAL_STATE.BlockListPath = GLOBAL_STATE.BasePath + "blocklists" + string(os.PathSeparator)

	err := os.Mkdir(GLOBAL_STATE.BlockListPath, 0o777)
	if err != nil {
		if !strings.Contains(err.Error(), "exists") {
			ERROR("Unable to create blocklist path", err)
		}
		return
	}

	DEBUG("Block list path initialized: ", GLOBAL_STATE.BlockListPath)
}

func ReBuildBlockLists(config *Config) (listError error) {
	defer RecoverAndLogToFile()
	defer runtime.GC()

	mapLock := new(sync.Mutex)
	newMap := make(map[string]struct{})

	wg := new(sync.WaitGroup)

	doList := func(index int) (err error) {
		defer func() {
			wg.Done()
		}()

		var listFile *os.File
		var didDownload bool
		var listBytes []byte
		var badLines int

		defer RecoverAndLogToFile()
		badLines = 0
		bl := config.AvailableBlockLists[index]

		if CheckIfURL(bl.FullPath) {
			_, err := os.Stat(bl.DiskPath)
			if err != nil {
				DEBUG("Could not find DNS blocklist on disk, downloading: ", bl.DiskPath)
				bl.LastRefresh = time.Now().AddDate(-2, 0, 0)
			}

			if time.Since(bl.LastRefresh).Hours() > 12 {
				listBytes, err = downloadList(bl.FullPath)
				if err != nil {
					listError = err
					ERROR("Could not download ", bl.FullPath, " ", err)
					return err
				}
				didDownload = true

				fileName := fmt.Sprintf("%s%s.txt",
					GLOBAL_STATE.BlockListPath,
					bl.Tag,
				)

				_ = os.Remove(fileName)
				listFile, err = os.Create(fileName)
				if err != nil {
					listError = err
					ERROR("Count open/create file ", fileName)
					return err
				}
				defer listFile.Close()
				bl.DiskPath = fileName
			} else {
				listBytes, err = os.ReadFile(bl.DiskPath)
				if err != nil {
					listError = err
					ERROR("Could not open ", bl.FullPath, " ", err)
					return err
				}
			}
		} else {
			listBytes, err = os.ReadFile(bl.FullPath)
			if err != nil {
				listError = err
				ERROR("Could not open ", bl.FullPath, err)
				return err
			}
		}

		if listBytes == nil {
			ERROR("No bytes from blocklist @ ", bl.FullPath, err)
			listError = fmt.Errorf("no bytes from blocklist @ %s ", bl.FullPath)
			return listError
		}

		bl.Count = 0
		buff := bytes.NewBuffer(listBytes)
		scanner := bufio.NewScanner(buff)
		for scanner.Scan() {
			d := scanner.Text()
			if CheckIfPlainDomain(d) {
				if didDownload && listFile != nil {
					_, err = listFile.WriteString(d + "\n")
					if err != nil {
						listError = err
						ERROR("Unable to write domain to file: ", err, " ", listFile.Name())
						return err
					}
				}
				if bl.Enabled {
					mapLock.Lock()
					newMap[d] = struct{}{}
					mapLock.Unlock()
				}
				bl.Count++
			} else {
				badLines++
			}
		}

		err = scanner.Err()
		if err != nil {
			listError = err
			ERROR("Error reading file ", bl.FullPath, " : ", err)
			return err
		}

		config.AvailableBlockLists[index].LastRefresh = time.Now()
		if badLines > 0 {
			DEBUG(badLines, " invalid lines in list: ", bl.FullPath)
		}
		return
	}

	for i := range config.AvailableBlockLists {
		wg.Add(1)
		go doList(i)
	}

	wg.Wait()

	DNSBlockLock.Lock()
	DNSBlockList = newMap
	DNSBlockLock.Unlock()

	return listError
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
			time.Sleep(1 * time.Second)
			tries++
			DEBUG("Unable to load list (retrying): ", url)
			goto retry
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
	return []*BlockList{
		{
			Tag:         "Ads",
			FullPath:    "https://raw.githubusercontent.com/n00bady/bluam/master/dns/merged/ads.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
		{
			Tag:         "AdultContent",
			FullPath:    "https://github.com/n00bady/bluam/raw/master/dns/merged/adult.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
		{
			Tag:         "CryptoCurrency",
			FullPath:    "https://github.com/n00bady/bluam/raw/master/dns/merged/crypto.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
		{
			Tag:         "Drugs",
			FullPath:    "https://github.com/n00bady/bluam/raw/master/dns/merged/drugs.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
		{
			Tag:         "FakeNews",
			FullPath:    "https://github.com/n00bady/bluam/raw/master/dns/merged/fakenews.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
		{
			Tag:         "Fraud",
			FullPath:    "https://github.com/n00bady/bluam/raw/master/dns/merged/fakenews.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
		{
			Tag:         "Gambling",
			FullPath:    "https://github.com/n00bady/bluam/raw/master/dns/merged/gambling.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
		{
			Tag:         "Malware",
			FullPath:    "https://github.com/n00bady/bluam/raw/master/dns/merged/malware.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
		{
			Tag:         "SocialMedia",
			FullPath:    "https://github.com/n00bady/bluam/raw/master/dns/merged/socialmedia.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
		{
			Tag:         "Surveillance",
			FullPath:    "https://github.com/n00bady/bluam/raw/master/dns/merged/surveillance.txt",
			LastRefresh: time.Now().AddDate(-2, 0, 0),
		},
	}
}

// Changed that to checking if the any blocklists toggled on/off
// probably I should rename it and also maybe I should make it
// return a slice of all the blocklists that changed
func CheckBlockListsEquality(oldList, newList []*BlockList) bool {
	defer RecoverAndLogToFile()

	if len(oldList) != len(newList) {
		return false
	}
	for i := range oldList {
		if oldList[i].Enabled != newList[i].Enabled {
			return false
		}
	}

	return true
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
