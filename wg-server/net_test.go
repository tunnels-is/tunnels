package wgserver

import (
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
	// Drain (-D) must match install (-A) exactly except for the action.
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
	// PublicIP is bind-only now, so egress NAT is always MASQUERADE — even when
	// PublicIP is set, no SNAT rule is emitted.
	cfg := &Config{
		WireGuardSubnet:  "10.0.0.0/22",
		WireGuardSubnet6: "fd00::/64",
		WireGuardIface:   "wg0",
		WireGuardPort:    51820,
		InternetIface:    "eth0",
		PublicIP:         "74.63.223.157",
	}
	want := []string{
		"iptables -A INPUT -p udp --dport 51820 -j ACCEPT",
		"ip6tables -A INPUT -p udp --dport 51820 -j ACCEPT",
		"iptables -A FORWARD -i wg0 -o wg0 -j ACCEPT",
		"iptables -A FORWARD -i wg0 -o eth0 -j ACCEPT",
		"iptables -A FORWARD -i eth0 -o wg0 -m state --state RELATED,ESTABLISHED -j ACCEPT",
		"iptables -A FORWARD -i eth0 -o wg0 -j DROP",
		"ip6tables -A FORWARD -i wg0 -o wg0 -j ACCEPT",
		"ip6tables -A FORWARD -i wg0 -o eth0 -j ACCEPT",
		"ip6tables -A FORWARD -i eth0 -o wg0 -m state --state RELATED,ESTABLISHED -j ACCEPT",
		"ip6tables -A FORWARD -i eth0 -o wg0 -j DROP",
		"iptables -t nat -A POSTROUTING -s 10.0.0.0/22 -o eth0 -j MASQUERADE",
		"ip6tables -t nat -A POSTROUTING -s fd00::/64 -o eth0 -j MASQUERADE",
	}
	got := PreviewRules(cfg)
	if !slices.Equal(got, want) {
		t.Fatalf("rule preview mismatch\nGOT:\n%s\nWANT:\n%s",
			joinLines(got), joinLines(want))
	}
	// No SNAT rule may appear, regardless of PublicIP.
	for _, line := range got {
		if strings.Contains(line, "SNAT") {
			t.Fatalf("SNAT rule leaked: %s", line)
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
	// No ip6tables MASQUERADE rule when there's no v6 subnet.
	for _, line := range got {
		if line == "ip6tables -t nat -A POSTROUTING -s  -o eth0 -j MASQUERADE" {
			t.Fatalf("v6 MASQUERADE leaked with empty subnet6")
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
