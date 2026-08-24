package client

import (
	"fmt"
	"os"
	"sync"

	"github.com/tunnels-is/tunnels/argon"
)

const (
	accountsDirName     = "accounts"
	accountUserFileName = "user"
	accountTunnelsDir   = "tunnels"
	accountDevicesDir   = "devices"
)

var accountMu sync.Mutex

func userIDToAccountHash(userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("empty user id")
	}
	h, err := argon.GenerateUserFolderHash(userID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h), nil
}

func accountDir(hash string) string {
	s := STATE.Load()
	return s.AccountsPath + hash + string(os.PathSeparator)
}

func accountUserFile(hash string) string {
	return accountDir(hash) + accountUserFileName
}

func accountTunnelsPath(hash string) string {
	return accountDir(hash) + accountTunnelsDir + string(os.PathSeparator)
}

func accountDevicesPath(hash string) string {
	return accountDir(hash) + accountDevicesDir + string(os.PathSeparator)
}

func ensureAccountDirs(hash string) error {
	if !isHexString(hash) {
		return fmt.Errorf("invalid account hash")
	}
	for _, dir := range []string{
		accountDir(hash),
		accountTunnelsPath(hash),
		accountDevicesPath(hash),
	} {
		if err := CreateFolder(dir); err != nil {
			return err
		}
	}
	return nil
}

func clearTunnelMetaMap() {
	TunnelMetaMap.Range(func(key string, _ *TunnelMETA) bool {
		TunnelMetaMap.Delete(key)
		return true
	})
}


func activateAccountByHash(hash string) error {
	accountMu.Lock()
	defer accountMu.Unlock()

	if !isHexString(hash) {
		return fmt.Errorf("invalid account hash")
	}
	if err := ensureAccountDirs(hash); err != nil {
		return err
	}

	s := STATE.Load()
	if s.ActiveAccountHash == hash && s.TunnelsPath == accountTunnelsPath(hash) {
		return nil
	}

	s.ActiveAccountHash = hash
	s.TunnelsPath = accountTunnelsPath(hash)
	s.DevicesPath = accountDevicesPath(hash)
	STATE.Store(s)

	clearTunnelMetaMap()
	if err := loadTunnelsFromDisk(); err != nil {
		return err
	}
	DEBUG("activated account workspace:", hash)
	return nil
}

func activateAccountByUserID(userID string) error {
	hash, err := userIDToAccountHash(userID)
	if err != nil {
		return err
	}
	return activateAccountByHash(hash)
}
