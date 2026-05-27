package wgserver

import (
	"bytes"
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

func TestShowActiveRules_CatchesAllShapes(t *testing.T) {
	// The heuristic is config-agnostic — it should pick up wg-server-style
	// rules regardless of which subnet/iface/PublicIP they were installed
	// with, and skip unrelated rules in the same chains.
	dump := fakeDumper{
		// INPUT: only -p udp ACCEPT counts.
		"iptables|filter|INPUT": strings.Join([]string{
			"-A INPUT -p udp -m udp --dport 51820 -j ACCEPT", // wg-shape
			"-A INPUT -p tcp -m tcp --dport 22 -j ACCEPT",    // unrelated
		}, "\n"),
		// FORWARD: anything touching wg* counts.
		"iptables|filter|FORWARD": strings.Join([]string{
			"-A FORWARD -i wg0 -o eth0 -j ACCEPT", // wg-shape
			"-A FORWARD -i wg01 -o wg01 -j ACCEPT", // wg-shape (stale iface name)
			"-A FORWARD -i br0 -o br1 -j ACCEPT",   // unrelated bridge
		}, "\n"),
		// POSTROUTING: any SNAT or MASQUERADE counts.
		"iptables|nat|POSTROUTING": strings.Join([]string{
			"-A POSTROUTING -s 10.0.4.0/22 -o eth0 -j SNAT --to-source 63.143.33.107", // stale wg subnet
			"-A POSTROUTING -s 10.0.0.0/22 -o eth0 -j SNAT --to-source 74.63.223.157", // current wg subnet
			"-A POSTROUTING -s 192.168.99.0/24 -o eth0 -j RETURN",                     // unrelated
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
	// Each matched rule must come with a ready-to-paste drain command that
	// rewrites -A → -D and prefixes the bin + table.
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
	// Chains are empty / contain only unrelated rules. Output should still
	// be produced (so the operator knows the check ran) and should clearly
	// say nothing matched.
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
