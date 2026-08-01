package client

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

func InitBaseFoldersAndPaths() {
	defer RecoverAndLog()
	DEBUG("Creating base folders and paths")
	s := STATE.Load()

	basePath := s.BasePath
	basePath, _ = strings.CutSuffix(basePath, string(os.PathSeparator))

	if basePath != "" {
		basePath = basePath + string(os.PathSeparator)
	} else {
		ex, err := os.Executable()
		if err != nil {
			wd, err := os.Getwd()
			if err != nil {
				panic(err)
			}
			basePath = wd + string(os.PathSeparator)
		} else {
			basePath = filepath.Dir(ex) + string(os.PathSeparator)
		}
	}

	s.BasePath = basePath
	CreateFolder(s.BasePath)
	s.ConfigFileName = s.BasePath + "tunnels" + configFileSuffix

	// Per-account workspaces: accounts/<hash>/{user,tunnels/,devices/}
	// TunnelsPath/DevicesPath stay empty until an account is activated (saveUser / connect).
	s.AccountsPath = s.BasePath + accountsDirName + string(os.PathSeparator)
	CreateFolder(s.AccountsPath)
	s.UserPath = s.AccountsPath
	s.TunnelsPath = ""
	s.DevicesPath = ""
	s.ActiveAccountHash = ""

	s.LogPath = s.BasePath + "logs" + string(os.PathSeparator)
	CreateFolder(s.LogPath)
	s.LogFileName = s.LogPath + time.Now().Format("2006-01-02") + ".log"

	s.BlockListPath = s.BasePath + "blocklists" + string(os.PathSeparator)
	CreateFolder(s.BlockListPath)

	s.WhiteListPath = s.BasePath + "whitelists" + string(os.PathSeparator)
	CreateFolder(s.WhiteListPath)

	STATE.Store(s)
}

func RenameFile(oldName, newName string) (err error) {
	_, err = os.Stat(oldName)
	if err != nil {
		if os.IsNotExist(err) {
			DEBUG("File does not exist: ", oldName)
			return nil
		}
		ERROR("Unable to check file: ", err)
		return
	}

	err = os.Rename(oldName, newName)
	if err != nil {
		ERROR("Unable to rename file: ", err)
		return
	}

	DEBUG("File renamed: ", oldName, " -> ", newName)
	return nil
}

func CreateFile(file string) (f *os.File, err error) {
	f, err = os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	if err != nil {
		ERROR("Unable to open file: ", err)
		return
	}

	DEBUG("File opened: ", f.Name())
	return
}

func writeFileWithBackup(path string, newContent []byte) (err error) {
	existing, err := os.ReadFile(path)
	if err == nil && sha256.Sum256(existing) == sha256.Sum256(newContent) {
		DEBUG("File content unchanged, skipping write: ", path)
		return nil
	}

	err = checkFileOwnership(path)
	if err != nil {
		return fmt.Errorf("refusing to modify file: %w", err)
	}

	err = RenameFile(path, path+backupFileSuffix)
	if err != nil {
		ERROR("Unable to rename file: ", err)
	}

	f, err := CreateFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(newContent)
	if err != nil {
		return err
	}

	return f.Sync()
}

func createFolder(path string) error {
	err := os.Mkdir(path, 0o700)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func CreateFolder(path string) {
	if err := createFolder(path); err != nil {
		ERROR("Unable to create folder: ", path, " ", err)
		os.Exit(1)
	}
	DEBUG("New directory:", path)
}

func verifyAndWriteFile(diskPath string, expected []byte) (bool, error) {
	expectedHash := sha256.Sum256(expected)

	diskBytes, err := os.ReadFile(diskPath)
	if err == nil {
		diskHash := sha256.Sum256(diskBytes)
		if diskHash == expectedHash {
			return false, nil
		}
	}

	_ = os.Remove(diskPath)

	f, err := os.OpenFile(diskPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return false, fmt.Errorf("create %s: %w", diskPath, err)
	}

	n, err := f.Write(expected)
	if err != nil {
		f.Close()
		return false, fmt.Errorf("write %s: %w", diskPath, err)
	}
	f.Close()

	if n != len(expected) {
		return false, fmt.Errorf("short write for %s: %d/%d", diskPath, n, len(expected))
	}

	return true, nil
}

func IsDefaultConnection(IFName string) bool {
	return strings.EqualFold(IFName, DefaultTunnelName)
}

func RecoverAndLog() {
	if r := recover(); r != nil {
		ERROR(r, string(debug.Stack()))
	}
}

func GetDomainAndSubDomain(domain string) (d, s string) {
	parts := strings.Split(domain, ".")

	if len(parts) == 2 {
		d = strings.Join(parts[len(parts)-2:], ".")
	} else if len(parts) > 2 {
		d = strings.Join(parts[len(parts)-3:], ".")
		s = strings.Join(parts[:len(parts)-3], ".")
	} else {
		return "", ""
	}

	return
}

func DNSAMapping(DNS []*types.DNSRecord, fullDomain string) *types.DNSRecord {
	domain, subdomain := GetDomainAndSubDomain(fullDomain)
	if domain == "" {
		return nil
	}
	domain = strings.TrimSuffix(domain, ".")

	for i, record := range DNS {

		if record == nil {
			continue
		}
		if subdomain != "" {
			if record.Domain == subdomain+"."+domain {
				return DNS[i]
			}
		}

		if record.Domain == domain {
			if subdomain == "" {
				return DNS[i]
			} else if record.Wildcard {
				return DNS[i]
			}
		}

	}

	return nil
}

func CheckIfPlainDomain(s string) bool {
	return strings.Contains(s, ".")
}

func tunnelMapRange(do func(tun *TUN) bool) {
	TunnelMap.Range(func(key string, value *TUN) bool {
		return do(value)
	})
}

func tunnelMetaMapRange(do func(tun *TunnelMETA) bool) {
	TunnelMetaMap.Range(func(key string, value *TunnelMETA) bool {
		return do(value)
	})
}

func doEvent(channel chan *event, method func()) {
	defer RecoverAndLog()
	select {
	case channel <- &event{
		method: method,
	}:
	default:
		panic("priority channel full")
	}
}

func newConcurrentSignal(tag string, ctx context.Context, method func()) {
	defer RecoverAndLog()
	select {
	case concurrencyMonitor <- &goSignal{
		monitor: concurrencyMonitor,
		tag:     tag,
		ctx:     ctx,
		method:  method,
	}:
	default:
		panic("concurrency monitor is full")
	}
}

func (s *goSignal) execute() {
	defer RecoverAndLog()
	s.method()
	time.Sleep(1 * time.Second)

	select {
	case s.monitor <- s:
	default:
		panic("monitor channel is full")
	}
}
