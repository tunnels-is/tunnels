package wgserver

import (
	"errors"
	"strings"
	"testing"
)

// fakeDumper returns canned chain listings keyed by "bin|table|chain".
type fakeDumper map[string]string

func (f fakeDumper) dump(bin, table, chain string) (string, error) {
	if out, ok := f[bin+"|"+table+"|"+chain]; ok {
		return out, nil
	}
	return "", nil // empty chain
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
	// This is exactly the bug we saw in prod — a prior run with PublicIP
	// 63.143.33.106 left a SNAT rule behind. The new run with .157 must
	// refuse to start.
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

func TestPreflight_ConflictingPublicIP(t *testing.T) {
	// A SNAT rule with --to-source matching PublicIP but a *different* source
	// subnet is also a conflict, because it would cover/shadow any rule we'd
	// install that shares the destination IP.
	dump := fakeDumper{
		"iptables|nat|POSTROUTING": strings.Join([]string{
			"-A POSTROUTING -s 192.168.0.0/24 -o eth0 -j SNAT --to-source 74.63.223.157",
		}, "\n"),
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil || !strings.Contains(err.Error(), "PublicIP") {
		t.Fatalf("expected PublicIP conflict, got: %v", err)
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
	// A TCP rule on the same port number is not our concern.
	dump := fakeDumper{
		"iptables|filter|INPUT": "-A INPUT -p tcp -m tcp --dport 51820 -j ACCEPT\n",
	}
	if err := preflightIPTablesWith(baseCfg(), dump.dump); err != nil {
		t.Fatalf("tcp/51820 should not conflict with udp wg port, got: %v", err)
	}
}

func TestPreflight_NonWGForwardIsAllowed(t *testing.T) {
	// FORWARD rules using a different wg iface (e.g., a sibling wg-server
	// running on the same host with its own subnet) must not block this one.
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
	// Operator should see every offending rule, not just the first.
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
	// Substring-style false positive must not trigger.
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
	// The error must give an operator everything they need to find and drain
	// the offending rule without re-running iptables -L themselves:
	//   - bin/table/chain
	//   - the rule's position within the chain (1-based, like iptables -L)
	//   - the exact "flag value" tokens that triggered the match
	//   - a ready-to-paste `iptables -D ...` drain command
	dump := fakeDumper{
		"iptables|nat|POSTROUTING": strings.Join([]string{
			"-P POSTROUTING ACCEPT",
			"-A POSTROUTING -s 10.99.0.0/16 -o eth0 -j MASQUERADE",                              // rule #1 — no conflict
			"-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 63.143.33.106",           // rule #2 — conflict
		}, "\n"),
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()

	mustContain := []string{
		"iptables nat/POSTROUTING rule #2",        // bin + table/chain + position
		`WireGuardSubnet clash on "-s 10.0.0.0/22"`, // field name + exact matched tokens
		"rule:  -A POSTROUTING -s 10.0.0.0/22",    // full rule echoed
		"drain: iptables -t nat -D POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 63.143.33.106",
		"Config: subnet=10.0.0.0/22",              // current config summary
	}
	for _, want := range mustContain {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q\nfull message:\n%s", want, msg)
		}
	}
	// The non-conflicting rule on line #1 must NOT appear in the report.
	if strings.Contains(msg, "10.99.0.0/16") {
		t.Errorf("non-conflicting rule leaked into the report:\n%s", msg)
	}
}

func TestPreflight_PositionCountsOnlyAppendRules(t *testing.T) {
	// Position numbering must skip -P (policy) lines so it lines up with what
	// `iptables -L --line-numbers` would show.
	dump := fakeDumper{
		"iptables|filter|FORWARD": strings.Join([]string{
			"-P FORWARD ACCEPT",
			"-A FORWARD -i eth0 -o eth1 -j ACCEPT",      // rule #1 — no conflict
			"-A FORWARD -i wg0 -o eth0 -j ACCEPT",       // rule #2 — conflict
		}, "\n"),
	}
	err := preflightIPTablesWith(baseCfg(), dump.dump)
	if err == nil || !strings.Contains(err.Error(), "rule #2") {
		t.Fatalf("expected position #2, got: %v", err)
	}
}

func TestPreflight_IgnoresPolicyAndComments(t *testing.T) {
	// -P (policy) and blank lines must not be parsed as rules.
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
