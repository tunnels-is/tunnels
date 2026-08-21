package wgserver

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type fakeDumper map[string]string

func (f fakeDumper) dump(bin, table, chain string) (string, error) {
	if out, ok := f[bin+"|"+table+"|"+chain]; ok {
		return out, nil
	}
	return "", nil
}

func baseCfg() *Config {
	return &Config{
		WireGuardSubnet:  "10.0.0.0/22",
		WireGuardSubnet6: "fd00::/64",
		WireGuardIface:   "wg0",
		WireGuardPort:    51820,
		InternetIface:    "eth0",
		PublicIP:         "74.63.223.157",
	}
}

func TestPreflight_CleanHost(t *testing.T) {
	dump := fakeDumper{}
	if err := preflightIPTablesWith(baseCfg(), dump.dump); err != nil {
		t.Fatalf("expected no conflict on a clean host, got: %v", err)
	}
}

func TestPreflight_StaleSNATWithDifferentPublicIP(t *testing.T) {

	dump := fakeDumper{
		"iptables|nat|POSTROUTING": strings.Join([]string{
			"-P POSTROUTING ACCEPT",
			"-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 63.143.33.106",
		}, "\n"),
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil {
		t.Fatal("expected conflict, got nil")
	}
	if !strings.Contains(err.Error(), "WireGuardSubnet") {
		t.Fatalf("expected subnet-conflict reason, got: %v", err)
	}
}

func TestPreflight_ConflictingWireGuardIface(t *testing.T) {
	dump := fakeDumper{
		"iptables|filter|FORWARD": "-A FORWARD -i wg0 -o eth0 -j ACCEPT\n",
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil || !strings.Contains(err.Error(), "WireGuardIface") {
		t.Fatalf("expected iface conflict, got: %v", err)
	}
}

func TestPreflight_ConflictingPort(t *testing.T) {
	dump := fakeDumper{
		"iptables|filter|INPUT": "-A INPUT -p udp -m udp --dport 51820 -j ACCEPT\n",
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil || !strings.Contains(err.Error(), "WireGuardPort") {
		t.Fatalf("expected port conflict, got: %v", err)
	}
}

func TestPreflight_PortMatchRequiresUDP(t *testing.T) {

	dump := fakeDumper{
		"iptables|filter|INPUT": "-A INPUT -p tcp -m tcp --dport 51820 -j ACCEPT\n",
	}
	if err := preflightIPTablesWith(baseCfg(), dump.dump); err != nil {
		t.Fatalf("tcp/51820 should not conflict with udp wg port, got: %v", err)
	}
}

func TestPreflight_NonWGForwardIsAllowed(t *testing.T) {

	dump := fakeDumper{
		"iptables|filter|FORWARD": strings.Join([]string{
			"-A FORWARD -i wg1 -o eth0 -j ACCEPT",
			"-A FORWARD -i eth0 -o wg1 -m state --state RELATED,ESTABLISHED -j ACCEPT",
		}, "\n"),
		"iptables|nat|POSTROUTING": "-A POSTROUTING -s 10.99.0.0/22 -o eth0 -j SNAT --to-source 1.2.3.4\n",
	}
	if err := preflightIPTablesWith(baseCfg(), dump.dump); err != nil {
		t.Fatalf("sibling wg-server rules should not conflict, got: %v", err)
	}
}

func TestPreflight_IPv6Subnet(t *testing.T) {
	dump := fakeDumper{
		"ip6tables|nat|POSTROUTING": "-A POSTROUTING -s fd00::/64 -o eth0 -j MASQUERADE\n",
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil || !strings.Contains(err.Error(), "WireGuardSubnet") {
		t.Fatalf("expected v6 subnet conflict, got: %v", err)
	}
}

func TestPreflight_ReportsAllConflicts(t *testing.T) {

	dump := fakeDumper{
		"iptables|nat|POSTROUTING": strings.Join([]string{
			"-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 63.143.33.106",
			"-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 74.63.223.157",
		}, "\n"),
		"iptables|filter|FORWARD": "-A FORWARD -i wg0 -o eth0 -j ACCEPT\n",
		"iptables|filter|INPUT":   "-A INPUT -p udp -m udp --dport 51820 -j ACCEPT\n",
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"63.143.33.106",
		"74.63.223.157",
		"wg0",
		"51820",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("conflict report missing %q\nfull message:\n%s", want, msg)
		}
	}
}

func TestPreflight_PropagatesDumpError(t *testing.T) {
	failing := func(bin, table, chain string) (string, error) {
		return "", errors.New("iptables: command not found")
	}
	err := preflightIPTablesWith(baseCfg(), failing)
	if err == nil || !strings.Contains(err.Error(), "command not found") {
		t.Fatalf("expected dump error to propagate, got: %v", err)
	}
}

func TestHasFlagValue_TokenBoundary(t *testing.T) {
	rule := "-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 74.63.223.157"

	if !hasFlagValue(rule, "-s", "10.0.0.0/22") {
		t.Error("should match exact -s value")
	}

	if hasFlagValue(rule, "-s", "10.0.0.0/2") {
		t.Error("must not match a prefix that happens to be a substring")
	}
	if hasFlagValue(rule, "-s", "10.0.0.0/222") {
		t.Error("must not match a suffix extension")
	}
	if !hasFlagValue(rule, "--to-source", "74.63.223.157") {
		t.Error("should match --to-source")
	}
	if hasFlagValue(rule, "-i", "eth0") {
		t.Error("must not pick up an -o value when querying -i")
	}
}

func TestPreflight_ErrorPinpointsLocation(t *testing.T) {

	dump := fakeDumper{
		"iptables|nat|POSTROUTING": strings.Join([]string{
			"-P POSTROUTING ACCEPT",
			"-A POSTROUTING -s 10.99.0.0/16 -o eth0 -j MASQUERADE",
			"-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 63.143.33.106",
		}, "\n"),
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()

	mustContain := []string{
		"iptables nat/POSTROUTING rule #2",
		`WireGuardSubnet clash on "-s 10.0.0.0/22"`,
		"rule:  -A POSTROUTING -s 10.0.0.0/22",
		"drain: iptables -t nat -D POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 63.143.33.106",
		"Config: subnet=10.0.0.0/22",
	}
	for _, want := range mustContain {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q\nfull message:\n%s", want, msg)
		}
	}

	if strings.Contains(msg, "10.99.0.0/16") {
		t.Errorf("non-conflicting rule leaked into the report:\n%s", msg)
	}
}

func TestPreflight_PositionCountsOnlyAppendRules(t *testing.T) {

	dump := fakeDumper{
		"iptables|filter|FORWARD": strings.Join([]string{
			"-P FORWARD ACCEPT",
			"-A FORWARD -i eth0 -o eth1 -j ACCEPT",
			"-A FORWARD -i wg0 -o eth0 -j ACCEPT",
		}, "\n"),
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil || !strings.Contains(err.Error(), "rule #2") {
		t.Fatalf("expected position #2, got: %v", err)
	}
}

func TestShowActiveRules_CatchesAllShapes(t *testing.T) {

	dump := fakeDumper{

		"iptables|filter|INPUT": strings.Join([]string{
			"-A INPUT -p udp -m udp --dport 51820 -j ACCEPT",
			"-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT",
		}, "\n"),

		"iptables|filter|FORWARD": strings.Join([]string{
			"-A FORWARD -i wg0 -o eth0 -j ACCEPT",
			"-A FORWARD -i wg01 -o wg01 -j ACCEPT",
			"-A FORWARD -i br0 -o br1 -j ACCEPT",
		}, "\n"),

		"iptables|nat|POSTROUTING": strings.Join([]string{
			"-A POSTROUTING -s 10.0.4.0/22 -o eth0 -j SNAT --to-source 63.143.33.107",
			"-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 74.63.223.157",
			"-A POSTROUTING -s 192.168.99.0/24 -o eth0 -j RETURN",
		}, "\n"),
	}

	var buf bytes.Buffer
	if err := showActiveRulesWith(&buf, dump.dump); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	mustContain := []string{
		"-A INPUT -p udp -m udp --dport 51820",
		"-A FORWARD -i wg0 -o eth0",
		"-A FORWARD -i wg01 -o wg01",
		"-A POSTROUTING -s 10.0.4.0/22 -o eth0 -j SNAT --to-source 63.143.33.107",
		"-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 74.63.223.157",
	}
	for _, want := range mustContain {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
	mustNotContain := []string{
		"-A INPUT -p tcp",
		"-A FORWARD -i br0",
		"-A POSTROUTING -s 192.168.99.0/24",
	}
	for _, banned := range mustNotContain {
		if strings.Contains(out, banned) {
			t.Errorf("unexpected %q in output:\n%s", banned, out)
		}
	}
}

func TestShowActiveRules_EmitsDrainCommands(t *testing.T) {

	dump := fakeDumper{
		"iptables|nat|POSTROUTING": "-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 1.2.3.4\n",
	}
	var buf bytes.Buffer
	if err := showActiveRulesWith(&buf, dump.dump); err != nil {
		t.Fatal(err)
	}
	want := "drain: iptables -t nat -D POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 1.2.3.4"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("missing drain command %q in:\n%s", want, buf.String())
	}
}

func TestShowActiveRules_NoMatchingRules(t *testing.T) {

	dump := fakeDumper{
		"iptables|filter|INPUT": "-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT\n",
	}
	var buf bytes.Buffer
	if err := showActiveRulesWith(&buf, dump.dump); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(no matching rules)") {
		t.Fatalf("expected '(no matching rules)' in:\n%s", buf.String())
	}
}

func TestShowActiveRules_PropagatesDumpError(t *testing.T) {
	failing := func(bin, table, chain string) (string, error) {
		return "", errors.New("iptables: command not found")
	}
	var buf bytes.Buffer
	err := showActiveRulesWith(&buf, failing)
	if err == nil || !strings.Contains(err.Error(), "command not found") {
		t.Fatalf("expected dump error to propagate, got: %v", err)
	}
}

func TestWGRuleHeuristic_PerChain(t *testing.T) {
	cases := []struct {
		name, rule, chain string
		wantMatch         bool
	}{
		{"wg iface input", "-A FORWARD -i wg0 -o eth0 -j ACCEPT", "FORWARD", true},
		{"wg iface output", "-A FORWARD -i eth0 -o wg7 -j ACCEPT", "FORWARD", true},
		{"non-wg forward", "-A FORWARD -i br0 -o br1 -j ACCEPT", "FORWARD", false},
		{"postrouting SNAT", "-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 1.2.3.4", "POSTROUTING", true},
		{"postrouting MASQUERADE", "-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j MASQUERADE", "POSTROUTING", true},
		{"postrouting RETURN unrelated", "-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j RETURN", "POSTROUTING", false},
		{"input udp accept", "-A INPUT -p udp -m udp --dport 51820 -j ACCEPT", "INPUT", true},
		{"input tcp accept", "-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT", "INPUT", false},
		{"input udp drop", "-A INPUT -p udp -m udp --dport 5353 -j DROP", "INPUT", false},
		{"input wg host drop", "-A INPUT -i wg0 -j DROP", "INPUT", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := wgRuleHeuristic(c.rule, c.chain) != ""
			if got != c.wantMatch {
				t.Errorf("wgRuleHeuristic(%q, %q) match=%v want=%v",
					c.rule, c.chain, got, c.wantMatch)
			}
		})
	}
}

func TestPreflight_IgnoresPolicyAndComments(t *testing.T) {

	dump := fakeDumper{
		"iptables|nat|POSTROUTING": strings.Join([]string{
			"-P POSTROUTING ACCEPT",
			"",
			"# stale comment",
		}, "\n"),
	}
	if err := preflightIPTablesWith(baseCfg(), dump.dump); err != nil {
		t.Fatalf("policy/comment lines must not trigger conflicts, got: %v", err)
	}
}
