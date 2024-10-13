//go:build aix || dragonfly || freebsd || (js && wasm) || linux || nacl || netbsd || openbsd || solaris

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
	"kernel.org/pub/linux/libs/security/libcap/cap"
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

func CheckCapabilities() {
	orig := cap.GetProc()
	defer orig.SetProc() // restore original caps on exit.

	c, err := orig.Dup()
	if err != nil {
		ERROR("failed to get capabilities", err)
		return
	}

	missingFlags := false
	on, _ := c.GetFlag(cap.Permitted, cap.NET_BIND_SERVICE)
	DEBUG("CAP_NET_BIND_SERVICE: ", on)
	if !on {
		missingFlags = true
	}

	on, _ = c.GetFlag(cap.Permitted, cap.NET_ADMIN)
	DEBUG("CAP_NET_ADMIN: ", on)
	if !on {
		missingFlags = true
	}

	on, _ = c.GetFlag(cap.Permitted, cap.NET_RAW)
	DEBUG("CAP_NET_RAW:", on)
	if !on {
		missingFlags = true
	}

	if !missingFlags {
		DEBUG("All capabilities are present")
		return
	}

	fmt.Println("Tunnels needs access to manage network capabilities, this access does NOT include root/sudo access.")
	fmt.Println("Enter Password: ")

	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err == nil {
		password := string(bytePassword)
		cmd := exec.Command("sudo", "-S", "/usr/sbin/setcap", "cap_net_raw,cap_net_bind_service,cap_net_admin+eip", os.Args[0])
		cmd.Stdin = strings.NewReader(password + "\n")
		cmd.Stdout = os.Stdout
		err = cmd.Run()
		_ = exec.Command("sudo", "-kK")
		if err != nil {
			fmt.Println("Unable to setcap: ", err)
			fmt.Println("RUN: `sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' [BINARY]` in order to give it permissions to change and manage networks")
			fmt.Println("RUN: `sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' [BINARY]` in order to give it permissions to change and manage networks")
			fmt.Println("RUN: `sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' [BINARY]` in order to give it permissions to change and manage networks")
			os.Exit(1)
		} else {
			// Reload binary after applying set cap
			argv0, _ := exec.LookPath(os.Args[0])
			syscall.Exec(argv0, os.Args, os.Environ())
			os.Exit(1)
		}
	}
}

func AdminCheck() {
	CheckCapabilities()
	GLOBAL_STATE.IsAdmin = true
}
