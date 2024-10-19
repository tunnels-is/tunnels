//go:build aix || dragonfly || freebsd || (js && wasm) || linux || nacl || netbsd || openbsd || solaris

package core

import (
	"os"
	"path/filepath"

	"github.com/tunnels-is/tunnels/setcap"
)

func OSSpecificInit() error {
	AdjustRoutersForTunneling()
	return nil
}

func ValidateAdapterID(meta *TunnelMETA) error {
	return nil
}

func GenerateBaseFolderPath() string {
	defer RecoverAndLogToFile()
	if BASE_PATH != "" {
		return BASE_PATH + string(os.PathSeparator)
	}

	base := "."
	ex, err := os.Executable()
	if err != nil {
		ERROR("Unable to find working directory: ", err.Error())
	} else {
		base = filepath.Dir(ex)
	}

	return base + string(os.PathSeparator) + "files" + string(os.PathSeparator)
}

func CreateBaseFolder() {
	_, err := os.Stat(GLOBAL_STATE.BasePath)
	if err != nil {
		err = os.Mkdir(GLOBAL_STATE.BasePath, 0o777)
		if err != nil {
			ERROR("Unable to create base folder: ", err)
			return
		}
	}

	GLOBAL_STATE.BaseFolderInitialized = true
}

func AdminCheck() {
	err := setcap.CheckCapabilities()
	if err != nil {
		ERROR("Tunnels does not have the proper permissions: ", err)
	} else {
		GLOBAL_STATE.IsAdmin = true
	}
}
