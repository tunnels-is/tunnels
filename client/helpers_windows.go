//go:build windows

package client

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
