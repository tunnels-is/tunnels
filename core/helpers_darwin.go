//go:build darwin

package core

import (
	"os"
	"os/exec"
)

func openURL(url string) error {
	var cmd string
	var args []string
	cmd = "open"
	args = []string{url}
	if len(args) > 1 {
		args = append(args[:1], append([]string{""}, args[1:]...)...)
	}
	return exec.Command(cmd, args...).Start()
}

func OSSpecificInit() error {
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

	// if !NATIVE {
	// 	base := "."
	// 	ex, err := os.Executable()
	// 	if err != nil {
	// 		ERROR("Unable to find working directory: ", err.Error())
	// 	} else {
	// 		base = filepath.Dir(ex)
	// 	}
	//
	// 	return base + string(os.PathSeparator) + "files" + string(os.PathSeparator)
	// }

	return GetPWD() + string(os.PathSeparator) + "tunnels" + string(os.PathSeparator)
}

func CreateBaseFolder() {
	// GLOBAL_STATE.BasePath = GenerateBaseFolderPath()
	// GLOBAL_STATE.BackupPath = GLOBAL_STATE.BasePath

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

func GetHOME() string {
	HOMEPATH := os.Getenv("HOME")
	if HOMEPATH == "" {
		HOMEPATH = "/tmp"
	}
	return HOMEPATH
}

func GetPWD() string {
	HOMEPATH := os.Getenv("HOME")
	if HOMEPATH == "" {
		HOMEPATH = "/tmp"
	}
	return HOMEPATH
}

func AdminCheck() error {
	DEBUG("Admin check")
	GLOBAL_STATE.IsAdmin = true
	return nil
}
