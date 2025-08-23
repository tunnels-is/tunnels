//go:build windows

package client

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func openURL(url string) error {
	return windows.ShellExecute(0, nil, windows.StringToUTF16Ptr(url), nil, nil, windows.SW_SHOWNORMAL)
}

func ValidateAdapterID(meta *TunnelMETA) error {
	_, err := windows.GUIDFromString(meta.WindowsGUID)
	if err != nil {
		return errors.New("invalid windows GUID on connection, err: " + err.Error())
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
	state := STATE.Load()
	ifid := int(state.DefaultInterfaceID.Load())
	INFO("restoring dns: 1.1.1.1, 1.0.0.1")
	_ = DNS_Del(strconv.Itoa(ifid))
	if ifid != 0 {
		_ = DNS_Set(strconv.Itoa(ifid), "1.1.1.1", "1")
		_ = DNS_Set(strconv.Itoa(ifid), "1.0.0.1", "2")
	} else {
		ERROR("unable to restore dns, could not find default interface")
	}
}

// func RestoreDNSOnClose() {
// 	INFO("restoring dns: ", DEFAULT_DNS_SERVERS)
// 	_ = DNS_Del(strconv.Itoa(DEFAULT_INTERFACE_ID))
// 	if len(DEFAULT_DNS_SERVERS) == 1 {
// 		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[0], "1")
// 	} else if len(DEFAULT_DNS_SERVERS) > 1 {
// 		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[0], "1")
// 		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[1], "2")
// 	}
// }

// func RestoreDNSOnDisconnect() {
// 	INFO("restoring dns: ", DEFAULT_DNS_SERVERS)
// 	_ = DNS_Del(strconv.Itoa(DEFAULT_INTERFACE_ID))
// 	if len(DEFAULT_DNS_SERVERS) == 1 {
// 		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[0], "1")
// 	} else if len(DEFAULT_DNS_SERVERS) > 1 {
// 		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[0], "1")
// 		_ = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), DEFAULT_DNS_SERVERS[1], "2")
// 	}
// }

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

// https://coolaj86.com/articles/golang-and-windows-and-admins-oh-my/
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
