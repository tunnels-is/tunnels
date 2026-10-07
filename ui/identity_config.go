package ui

import (
	"os"
	"path/filepath"
	"runtime"
)

// migrateLegacyFyneConfig moves Fyne's settings directory from legacyAppID
// to appID. Fyne reads that directory inside NewWithID, so this has to run
// first. An existing new directory is left alone.
func migrateLegacyFyneConfig() {
	root := fyneConfigRoot()
	if root == "" {
		return
	}
	_ = migrateFyneConfigDir(root, legacyAppID, appID)
}

func fyneConfigRoot() string {
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		dir, err := os.UserConfigDir()
		if err != nil || dir == "" {
			return ""
		}
		return filepath.Join(dir, "fyne")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		return filepath.Join(home, "Library", "Preferences", "fyne")
	case "windows":
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return ""
		}
		return filepath.Join(home, "AppData", "Roaming", "fyne")
	default:
		return ""
	}
}

func migrateFyneConfigDir(root, from, to string) error {
	if root == "" || from == "" || from == to {
		return nil
	}
	oldPath := filepath.Join(root, from)
	newPath := filepath.Join(root, to)
	if _, err := os.Stat(newPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Stat(oldPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Rename(oldPath, newPath)
}
