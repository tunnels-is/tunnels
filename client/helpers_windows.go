//go:build windows

package client

import (
	"os"
	"path/filepath"
)

func wintunDLLPath() string {
	ex, err := os.Executable()
	if err != nil {
		return "wintun.dll"
	}
	return filepath.Join(filepath.Dir(ex), "wintun.dll")
}

func ValidateAdapterID(meta *TunnelMETA) error {
	return nil
}

func OSSpecificInit() error {
	fb, err := DLL_EMBED.ReadFile("wintun.dll")
	if err != nil {
		ERROR("unable to load embedded wintun: ", err)
		return err
	}

	written, err := verifyAndWriteFile(wintunDLLPath(), fb)
	if err != nil {
		ERROR("unable to verify/write wintun.dll: ", err)
		return err
	}
	if written {
		DEBUG("wintun.dll extracted (hash verified)")
	}

	return nil
}

func AdminCheck() {
	defer RecoverAndLog()

	fd, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		s := STATE.Load()
		s.adminState = false
		ERROR("Tunnels is not running as administrator, please restart as administartor")
		return
	}

	DEBUG("Tunnels is running as admin")

	s := STATE.Load()
	s.adminState = true
	_ = fd.Close()
}
