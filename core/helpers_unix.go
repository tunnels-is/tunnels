//go:build aix || dragonfly || freebsd || (js && wasm) || linux || nacl || netbsd || openbsd || solaris

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tunnels-is/tunnels/setcap"
)

func openURL(url string) error {
	var cmd string
	var args []string
	cmd = "xdg-open"
	args = []string{url}
	if len(args) > 1 {
		args = append(args[:1], append([]string{""}, args[1:]...)...)
	}
	return exec.Command(cmd, args...).Start()
}

func OSSpecificInit() error {
	return AdjustRoutersForTunneling()
}

func ValidateAdapterID(meta *TunnelMETA) error {
	return nil
}

func InitBaseFoldersAndPaths() {
	defer RecoverAndLogToFile()
	DEBUG("Creating base folders and paths")
	s := STATE.Load()

	basePath := s.BasePath
	basePath, _ = strings.CutSuffix(basePath, string(os.PathSeparator))

	if basePath != "" {
		basePath = s.BasePath + string(os.PathSeparator)
	} else {
		ex, err := os.Executable()
		if err != nil {
			wd, err := os.Getwd()
			if err != nil {
				fmt.Println("Unable to find working directory!", err.Error())
				panic(err)
			}
			basePath = wd + string(os.PathSeparator)
		} else {
			basePath = filepath.Dir(ex) + string(os.PathSeparator)
		}
	}

	s.BasePath = basePath
	s.TunnelsPath = s.BasePath

	CreateFolder(s.BasePath)
	s.ConfigFileName = s.BasePath + "tunnels.json"

	s.LogPath = s.BasePath + "logs" + string(os.PathSeparator)
	CreateFolder(s.LogPath)
	s.LogFileName = s.LogPath + time.Now().Format("2006-01-02") + ".log"

	s.TracePath = s.LogPath
	s.TraceFileName = s.TracePath + time.Now().Format("2006-01-02-15-04-05") + ".trace.log"

	s.BlockListPath = s.BasePath + "blocklists" + string(os.PathSeparator)
	CreateFolder(s.BlockListPath)
}

func CreateFile(file string) (f *os.File, err error) {
	f, err = os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o777)
	if err != nil {
		ERROR("Unable to open file: ", err)
		return
	}

	DEBUG("New file opened: ", f.Name())
	return
}

func CreateFolder(path string) {
	_, err := os.Stat(path)
	if err != nil {
		err = os.Mkdir(path, 0o777)
		if err != nil {
			ERROR("Unable to create base folder: ", err)
			return
		}
	}
}

func AdminCheck() {
	err := setcap.CheckCapabilities()
	if err != nil {
		ERROR("Tunnels does not have the proper permissions: ", err)
	} else {
		s := STATE.Load()
		s.adminState = true
	}
}
