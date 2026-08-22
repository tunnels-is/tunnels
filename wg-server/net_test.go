package wgserver

import (
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"
)

func TestMasqueradeArgs_AlwaysMASQUERADE(t *testing.T) {
	got := masqueradeArgs("-A", "10.0.0.0/24", "eth0")
	want := []string{
		"-t", "nat", "-A", "POSTROUTING",
		"-s", "10.0.0.0/24", "-o", "eth0",
		"-j", "MASQUERADE",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMasqueradeArgs_DrainShape(t *testing.T) {

	install := masqueradeArgs("-A", "10.0.0.0/24", "eth0")
	drain := masqueradeArgs("-D", "10.0.0.0/24", "eth0")

	if len(install) != len(drain) {
		t.Fatalf("install/drain length mismatch")
	}
	for i := range install {
		if install[i] == "-A" {
			if drain[i] != "-D" {
				t.Fatalf("expected -D at index %d, got %q", i, drain[i])
			}
			continue
		}
		if install[i] != drain[i] {
			t.Fatalf("mismatch at %d: install=%q drain=%q", i, install[i], drain[i])
		}
	}
}

func TestPreviewRules_MASQUERADEEvenWithPublicIP(t *testing.T) {

	cfg := &Config{
		WireGuardSubnet:  "10.0.0.0/22",
		WireGuardSubnet6: "fd00::/64",
		WireGuardIface:   "wg0",
		WireGuardPort:    51820,
		InternetIface:    "eth0",
		PublicIP:         "74.63.223.157",
	}
	got := PreviewRules(cfg)
	for _, want := range []string{
		"iptables -A INPUT -p udp --dport 51820 -j ACCEPT",
		"iptables -I INPUT -i wg0 -j DROP",
		"ip6tables -A INPUT -p udp --dport 51820 -j ACCEPT",
		"ip6tables -I INPUT -i wg0 -j DROP",
		"iptables -I FORWARD 1 -j TUNNELS_FWD_wg0",
		"iptables -A TUNNELS_FWD_wg0 -o eth0 -d 169.254.0.0/16 -j DROP",
		"iptables -A TUNNELS_FWD_wg0 -i wg0 -o wg0 -j ACCEPT",
		"iptables -A TUNNELS_FWD_wg0 -i wg0 -o eth0 -j ACCEPT",
		"iptables -A TUNNELS_FWD_wg0 -i eth0 -o wg0 -m state --state RELATED,ESTABLISHED -j ACCEPT",
		"iptables -A TUNNELS_FWD_wg0 -i eth0 -o wg0 -j DROP",
		"iptables -A TUNNELS_FWD_wg0 -i wg0 -j DROP",
		"ip6tables -I FORWARD 1 -j TUNNELS_FWD_wg0",
		"ip6tables -A TUNNELS_FWD_wg0 -o eth0 -d fe80::/10 -j DROP",
		"ip6tables -A TUNNELS_FWD_wg0 -i wg0 -o wg0 -j ACCEPT",
		"ip6tables -A TUNNELS_FWD_wg0 -i wg0 -o eth0 -j ACCEPT",
		"ip6tables -A TUNNELS_FWD_wg0 -i eth0 -o wg0 -m state --state RELATED,ESTABLISHED -j ACCEPT",
		"ip6tables -A TUNNELS_FWD_wg0 -i eth0 -o wg0 -j DROP",
		"ip6tables -A TUNNELS_FWD_wg0 -i wg0 -j DROP",
		"iptables -t nat -A POSTROUTING -s 10.0.0.0/22 -o eth0 -j MASQUERADE",
		"ip6tables -t nat -A POSTROUTING -s fd00::/64 -o eth0 -j MASQUERADE",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("expected %q in preview, got:\n%s", want, joinLines(got))
		}
	}

	for _, line := range got {
		if strings.Contains(line, "SNAT") {
			t.Fatalf("SNAT rule leaked: %s", line)
		}
	}
	mustDestDropsBeforeInternetAccept(t, got, "iptables", "TUNNELS_FWD_wg0", "eth0")
	mustPreviewDropCover(t, got, "iptables", "TUNNELS_FWD_wg0", "eth0", "168.63.129.16")
	mustPreviewDropCover(t, got, "iptables", "TUNNELS_FWD_wg0", "eth0", "100.100.100.200")
	mustPreviewDropCover(t, got, "ip6tables", "TUNNELS_FWD_wg0", "eth0", "fd00:ec2::254")
	mustPreviewDropCover(t, got, "ip6tables", "TUNNELS_FWD_wg0", "eth0", "fe80::a9fe:a9fe")
}

func TestPreviewRules_HostInputDrop(t *testing.T) {
	cfg := &Config{
		WireGuardSubnet:   "10.0.0.0/22",
		WireGuardIface:    "wg0",
		WireGuardPort:     51820,
		WireGuardMeshPort: 51821,
		InternetIface:     "eth0",
	}
	got := PreviewRules(cfg)
	for _, want := range []string{
		"iptables -I INPUT -i wg0 -j DROP",
		"ip6tables -I INPUT -i wg0 -j DROP",
		"iptables -I INPUT -i wg0mesh -j DROP",
		"ip6tables -I INPUT -i wg0mesh -j DROP",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("expected %q in preview, got:\n%s", want, joinLines(got))
		}
	}
	for _, line := range got {
		if strings.Contains(line, "INPUT") && strings.Contains(line, "-i wg0 -j DROP") && !strings.Contains(line, "-I INPUT") {
			t.Fatalf("host INPUT drop must be inserted (-I), got: %s", line)
		}
	}
}

func TestPreviewRules_MASQUERADEWhenNoPublicIP(t *testing.T) {
	cfg := &Config{
		WireGuardSubnet: "10.0.0.0/22",
		WireGuardIface:  "wg0",
		WireGuardPort:   51820,
		InternetIface:   "eth0",
	}
	got := PreviewRules(cfg)

	wantNAT := "iptables -t nat -A POSTROUTING -s 10.0.0.0/22 -o eth0 -j MASQUERADE"
	if !slices.Contains(got, wantNAT) {
		t.Fatalf("expected %q in output, got:\n%s", wantNAT, joinLines(got))
	}
}

func TestPreviewRules_NoSubnet6_EmitsDropRules(t *testing.T) {
	cfg := &Config{
		WireGuardSubnet: "10.0.0.0/22",
		WireGuardIface:  "wg0",
		WireGuardPort:   51820,
		InternetIface:   "eth0",
	}
	got := PreviewRules(cfg)

	for _, want := range []string{
		"ip6tables -A FORWARD -i wg0 -j DROP",
		"ip6tables -A FORWARD -o wg0 -j DROP",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("expected %q in output, got:\n%s", want, joinLines(got))
		}
	}

	for _, line := range got {
		if line == "ip6tables -t nat -A POSTROUTING -s  -o eth0 -j MASQUERADE" {
			t.Fatalf("v6 MASQUERADE leaked with empty subnet6")
		}
	}
}

func TestTunnelsFwdChainName_PerIface(t *testing.T) {
	a := tunnelsFwdChainName("wg0")
	b := tunnelsFwdChainName("wg1")
	if a == b || a != "TUNNELS_FWD_wg0" || b != "TUNNELS_FWD_wg1" {
		t.Fatalf("got %q %q", a, b)
	}
	if tunnelsFwdChainName("br-ex") != "TUNNELS_FWD_br_ex" {
		t.Fatalf("got %q", tunnelsFwdChainName("br-ex"))
	}
}

func TestFirstHost_MasksUnalignedPrefix(t *testing.T) {
	_, n, err := net.ParseCIDR("10.0.0.5/22")
	if err != nil {
		t.Fatal(err)
	}
	if got := firstHost(n).String(); got != "10.0.0.1" {
		t.Fatalf("firstHost = %s, want 10.0.0.1", got)
	}
}

func TestPreviewRules_IMDSAndDefaultDeny(t *testing.T) {
	cfg := &Config{
		WireGuardSubnet: "10.0.0.0/22",
		WireGuardIface:  "wg0",
		WireGuardPort:   51820,
		InternetIface:   "eth0",
	}
	got := PreviewRules(cfg)
	for _, want := range []string{
		"iptables -A TUNNELS_FWD_wg0 -o eth0 -d 169.254.0.0/16 -j DROP",
		"iptables -A TUNNELS_FWD_wg0 -i wg0 -j DROP",
		"iptables -I FORWARD 1 -j TUNNELS_FWD_wg0",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("expected %q in preview, got:\n%s", want, joinLines(got))
		}
	}
	mustDestDropsBeforeInternetAccept(t, got, "iptables", "TUNNELS_FWD_wg0", "eth0")
}

func mustPreviewDropCover(t *testing.T, lines []string, bin, chain, netIface, ip string) {
	t.Helper()
	dest := net.ParseIP(ip)
	if dest == nil {
		t.Fatalf("bad ip %q", ip)
	}
	prefix := fmt.Sprintf("%s -A %s -o %s -d ", bin, chain, netIface)
	for _, line := range lines {
		if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, " -j DROP") {
			continue
		}
		cidr := strings.TrimSuffix(strings.TrimPrefix(line, prefix), " -j DROP")
		_, n, err := net.ParseCIDR(cidr)
		if err == nil && n.Contains(dest) {
			return
		}
	}
	t.Fatalf("no dest DROP on %s covers %s in:\n%s", netIface, ip, joinLines(lines))
}

func mustDestDropsBeforeInternetAccept(t *testing.T, lines []string, bin, chain, netIface string) {
	t.Helper()
	accept := fmt.Sprintf("%s -A %s -i wg0 -o %s -j ACCEPT", bin, chain, netIface)
	acceptAt := -1
	var dropAt []int
	prefix := fmt.Sprintf("%s -A %s -o %s -d ", bin, chain, netIface)
	suffix := " -j DROP"
	for i, line := range lines {
		if line == accept {
			acceptAt = i
		}
		if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, suffix) {
			dropAt = append(dropAt, i)
		}
	}
	if acceptAt < 0 {
		t.Fatalf("missing internet ACCEPT %q in:\n%s", accept, joinLines(lines))
	}
	if len(dropAt) == 0 {
		t.Fatalf("missing dest DROPs on %s in:\n%s", netIface, joinLines(lines))
	}
	for _, i := range dropAt {
		if i > acceptAt {
			t.Fatalf("dest DROP after internet ACCEPT:\n  %s\n  %s", lines[i], lines[acceptAt])
		}
	}
}

func joinLines(ss []string) string {
	out := ""
	for _, s := range ss {
		out += "  " + s + "\n"
	}
	return out
}
