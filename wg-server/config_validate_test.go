package wgserver

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidIfaceName(t *testing.T) {
	ok := []string{"eth0", "wg0", "ens18", "br-ex"}
	for _, n := range ok {
		if !validIfaceName(n) {
			t.Errorf("%q should be valid", n)
		}
	}
	bad := []string{"", "+", "eth+", "wg0+", "*", "thisnameiswaytoolong"}
	for _, n := range bad {
		if validIfaceName(n) {
			t.Errorf("%q should be invalid", n)
		}
	}
}

func TestCriticalConfigChanged(t *testing.T) {
	a := &Config{WireGuardSubnet: "10.0.0.0/24", WireGuardIface: "wg0", InternetIface: "eth0", WireGuardPort: 51820, ServerID: "a"}
	b := *a
	if criticalConfigChanged(a, &b) {
		t.Fatal("identical configs")
	}
	b.EnableFirewall = true
	if criticalConfigChanged(a, &b) {
		t.Fatal("firewall-only change is not critical")
	}
	b.InternetIface = "ens18"
	if !criticalConfigChanged(a, &b) {
		t.Fatal("InternetIface change is critical")
	}
}

func TestLoadOrGenerateLocalPrivKey_RefusesInsecurePerms(t *testing.T) {
	dir := t.TempDir()
	setPKPathFromConfig(filepath.Join(dir, "wg-config.json"))
	path := pkPath()
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(make([]byte, 32))), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrGenerateLocalPrivKey(true); err == nil {
		t.Fatal("insecure .pk must refuse to start")
	}
}

func TestAddPeerIPC_ReplacesAllowedIPs(t *testing.T) {
	got := fmtAddPeer("deadbeef", "10.0.0.2/32")
	if !containsAll(got, "replace_allowed_ips=true", "allowed_ip=10.0.0.2/32") {
		t.Fatalf("AddPeer IPC missing replace_allowed_ips: %q", got)
	}
}

func fmtAddPeer(pubKeyHex string, allowedIPs ...string) string {
	conf := "public_key=" + sanitizeIPC(pubKeyHex) + "\nreplace_allowed_ips=true\n"
	for _, aip := range allowedIPs {
		conf += "allowed_ip=" + sanitizeIPC(aip) + "\n"
	}
	return conf + "\n"
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
