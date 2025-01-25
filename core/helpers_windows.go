//go:build windows

package core

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func openURL(url string) error {
	return windows.ShellExecute(0, nil, windows.StringToUTF16Ptr(url), nil, nil, windows.SW_SHOWNORMAL)
}

func stripSuffix(domain string) string {
	if strings.HasSuffix(domain, ".lan.lan.") {
		domain = strings.TrimSuffix(domain, ".lan.")
		domain += "."
	}
	return domain
}

func ValidateAdapterID(meta *TunnelMETA) error {
	_, err := windows.GUIDFromString(meta.WindowsGUID)
	if err != nil {
		return errors.New("invalid windows GUID on default connection, err: " + err.Error())
	}
	return nil
}

func OSSpecificInit() error {
	_, err := os.Stat("wintun.dll")
	if err != nil {
		_ = os.Remove("wintun.dll")

		fb, err := DLL_EMBED.ReadFile("wintun.dll")
		if err != nil {
			ERROR("unable to load wintun: ", err)
			return err
		}

		f, err := os.Create("wintun.dll")
		if err != nil {
			ERROR("unable to create wintun: ", err)
			return err
		}

		n, err := f.Write(fb)
		if err != nil {
			ERROR("unable to write new wintun: ", err)
			return err
		}

		f.Close()
		if n != len(fb) {
			ERROR("did not write all bytes to wintun.dll: ", n, len(fb))
			return errors.New("")
		}
	}

	return nil
}

func RestoreSaneDNSDefaults() {
	INFO("restoring dns: ", DEFAULT_DNS_SERVERS)
	_ = DNS_Del(strconv.Itoa(DEFAULT_INTERFACE_ID))
	if DEFAULT_INTERFACE_ID != 0 {
		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), "1.1.1.1", "1")
		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), "1.0.0.1", "2")
	} else {
		ERROR("unable to restore dns, could not find default interface")
	}
}

func RestoreDNSOnClose() {
	INFO("restoring dns: ", DEFAULT_DNS_SERVERS)
	_ = DNS_Del(strconv.Itoa(DEFAULT_INTERFACE_ID))
	if len(DEFAULT_DNS_SERVERS) == 1 {
		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[0], "1")
	} else if len(DEFAULT_DNS_SERVERS) > 1 {
		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[0], "1")
		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[1], "2")
	}
}

func RestoreDNSOnDisconnect() {
	INFO("restoring dns: ", DEFAULT_DNS_SERVERS)
	_ = DNS_Del(strconv.Itoa(DEFAULT_INTERFACE_ID))
	if len(DEFAULT_DNS_SERVERS) == 1 {
		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[0], "1")
	} else if len(DEFAULT_DNS_SERVERS) > 1 {
		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[0], "1")
		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[1], "2")
	}
}

func GetDNSServers(intf string) (err error) {
	var out []byte
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "dnsservers", intf)
	out, err = cmd.CombinedOutput()
	if err != nil {
		ERROR("could not find default dns servers: ", err)
		return err
	}

	rxp := `\b(?:(?:25[0-5]|[1-2][0-9]{2}|[0-9]{1,2})\.){3}(?:25[0-5]|[1-2][0-9]{2}|[0-9]{1,2})\b`
	re := regexp.MustCompile(rxp)

	DEFAULT_DNS_SERVERS = re.FindAllString(string(out), -1)

	if DEFAULT_DNS_SERVERS != nil {
		INFO("default dns servers found: ", DEFAULT_DNS_SERVERS)
	} else {
		ERROR("could not find default dns servers")
	}
	return
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
	defer RecoverAndLogToFile()

	DEBUG("Verifying configurations and logging folder")

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

// https://coolaj86.com/articles/golang-and-windows-and-admins-oh-my/
func AdminCheck() {
	defer RecoverAndLogToFile()

	fd, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		GLOBAL_STATE.IsAdmin = false
		ERROR("Tunnels is not running as administrator, please restart as administartor")
		return
	}

	DEBUG("Tunnels is running as admin")
	GLOBAL_STATE.IsAdmin = true
	_ = fd.Close()
}

func IPv6Enabled() bool {
	defer RecoverAndLogToFile()

	cmd := exec.Command("netsh", "interface", "ipv6", "show", "interface", DEFAULT_INTERFACE_NAME)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		ERROR("Unable to detect IPv6 setting // msg: ", err, " // output: ", string(out))
		return true
	}

	// windows will inject a bunch of random control characters
	// but it's always size 24, so if the len(out) is < 30
	// then ipv6 is disabled on the adapter.
	if len(out) > 30 {
		return true
	}
	return false
}
