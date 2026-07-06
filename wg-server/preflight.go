package wgserver

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ruleDumper returns the output of `bin -t table -S chain`. Indirected so
// tests can feed canned chain listings.
type ruleDumper func(bin, table, chain string) (string, error)

func defaultRuleDumper(bin, table, chain string) (string, error) {
	args := []string{"-t", table, "-S", chain}
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %v: %w: %s", bin, args, err, string(out))
	}
	return string(out), nil
}

// ruleConflict is one offending iptables rule discovered by the preflight
// check. It carries enough context for the operator to locate and drain the
// rule by hand without re-running iptables -L themselves.
type ruleConflict struct {
	bin   string // "iptables" / "ip6tables"
	table string // "filter" / "nat"
	chain string // "INPUT" / "FORWARD" / "POSTROUTING"
	pos   int    // 1-based position within the chain, matching `iptables -L --line-numbers`
	field string // which config field clashed: "WireGuardSubnet" / "WireGuardIface" / "WireGuardPort"
	match string // the exact "flag value" tokens that matched, e.g. `-s 10.0.0.0/22`
	rule  string // the full -A line as printed by `iptables -S`
}

// drainCommand returns a ready-to-paste shell command that removes this rule.
// It simply rewrites the leading `-A <chain>` to `-D <chain>`, which is
// exactly what `iptables -S` is meant to round-trip into.
func (c ruleConflict) drainCommand() string {
	delForm := strings.Replace(c.rule, "-A ", "-D ", 1)
	return fmt.Sprintf("%s -t %s %s", c.bin, c.table, delForm)
}

func (c ruleConflict) String() string {
	return fmt.Sprintf(
		"%s %s/%s rule #%d — %s clash on %q\n"+
			"      rule:  %s\n"+
			"      drain: %s",
		c.bin, c.table, c.chain, c.pos, c.field, c.match, c.rule, c.drainCommand(),
	)
}

// preflightIPTables refuses to start the wg-server if any existing
// iptables/ip6tables rule references this config's WireGuardSubnet,
// WireGuardSubnet6, WireGuardIface, or WireGuardPort.
//
// The check is intentionally strict: the legacy on-shutdown cleanup is
// config-pinned and leaks rules whenever the operator changes subnet or iface
// between runs. Rather than try to auto-clean state we no longer recognise
// (and risk silently shadowing it), we exit and ask the operator to drain the
// table by hand. This also enforces the invariant that only one wg-server ever
// owns a given subnet on a host.
func preflightIPTables(cfg *Config) error {
	return preflightIPTablesWith(cfg, defaultRuleDumper)
}

func preflightIPTablesWith(cfg *Config, dump ruleDumper) error {
	chains := []struct {
		bin, table, chain string
		ipv6              bool
	}{
		{"iptables", "filter", "INPUT", false},
		{"iptables", "filter", "FORWARD", false},
		{"iptables", "nat", "POSTROUTING", false},
		{"ip6tables", "filter", "INPUT", true},
		{"ip6tables", "filter", "FORWARD", true},
		{"ip6tables", "nat", "POSTROUTING", true},
	}

	var conflicts []ruleConflict
	for _, c := range chains {
		out, err := dump(c.bin, c.table, c.chain)
		if err != nil {
			return fmt.Errorf("dump %s -t %s %s: %w", c.bin, c.table, c.chain, err)
		}

		pos := 0
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "-A ") {
				continue
			}
			pos++ // counts only -A rules, mirroring how iptables numbers them
			field, match := ruleConflictsWithCfg(line, cfg, c.ipv6)
			if field == "" {
				continue
			}
			conflicts = append(conflicts, ruleConflict{
				bin: c.bin, table: c.table, chain: c.chain, pos: pos,
				field: field, match: match, rule: line,
			})
		}
	}

	if len(conflicts) == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "iptables preflight: refusing to start — %d existing rule(s) conflict with this config.\n",
		len(conflicts))
	fmt.Fprintf(&b, "Config: subnet=%s subnet6=%s iface=%s port=%d publicIP=%s\n",
		cfgOrDash(cfg.WireGuardSubnet),
		cfgOrDash(cfg.WireGuardSubnet6),
		cfgOrDash(cfg.WireGuardIface),
		cfg.WireGuardPort,
		cfgOrDash(cfg.PublicIP),
	)
	fmt.Fprintln(&b, "Drain each rule below (paste the `drain:` line), then restart:")
	for i, c := range conflicts {
		fmt.Fprintf(&b, "  [%d] %s\n", i+1, c.String())
	}
	return fmt.Errorf("%s", strings.TrimRight(b.String(), "\n"))
}

func cfgOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ruleConflictsWithCfg inspects a single `iptables -S` line and reports
// whether it touches anything this wg-server owns under cfg. The returned
// field is the offending config name ("WireGuardSubnet", "WireGuardIface", or
// "WireGuardPort") and match is the exact "flag value" pair that triggered the
// hit (e.g. `-s 10.0.0.0/22`). Both are empty when the rule does not conflict.
//
// The ipv6 flag selects IPv4 vs IPv6 subnet identity.
func ruleConflictsWithCfg(rule string, cfg *Config, ipv6 bool) (field, match string) {
	subnet := cfg.WireGuardSubnet
	if ipv6 {
		subnet = cfg.WireGuardSubnet6
	}
	if subnet != "" {
		if hasFlagValue(rule, "-s", subnet) {
			return "WireGuardSubnet", "-s " + subnet
		}
		if hasFlagValue(rule, "-d", subnet) {
			return "WireGuardSubnet", "-d " + subnet
		}
	}

	if cfg.WireGuardIface != "" {
		if hasFlagValue(rule, "-i", cfg.WireGuardIface) {
			return "WireGuardIface", "-i " + cfg.WireGuardIface
		}
		if hasFlagValue(rule, "-o", cfg.WireGuardIface) {
			return "WireGuardIface", "-o " + cfg.WireGuardIface
		}
	}

	if cfg.WireGuardPort > 0 {
		portStr := fmt.Sprintf("%d", cfg.WireGuardPort)
		if hasFlagValue(rule, "--dport", portStr) && hasFlagValue(rule, "-p", "udp") {
			return "WireGuardPort", "-p udp --dport " + portStr
		}
	}

	return "", ""
}

// hasFlagValue reports whether rule (a single `iptables -S` line, whitespace-
// separated) contains the pair `flag value` as adjacent tokens. Token-level
// matching avoids substring false positives — e.g. WireGuardSubnet=10.0.0.0/2
// must not match a rule that carries -s 10.0.0.0/22.
func hasFlagValue(rule, flag, value string) bool {
	fields := strings.Fields(rule)
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == flag && fields[i+1] == value {
			return true
		}
	}
	return false
}

// ShowActiveRules writes every currently-installed iptables/ip6tables rule
// across the chains wg-server cares about that matches a config-agnostic
// "looks like wg-server" heuristic. Unlike preflightIPTables, this does not
// take a Config — it surfaces rules left over from any prior run regardless
// of which subnet / iface / PublicIP they were installed under.
//
// Useful for operators who hit a preflight conflict and want to see what
// drain commands to run, without restarting against the offending config.
func ShowActiveRules() error {
	return showActiveRulesWith(os.Stdout, defaultRuleDumper)
}

func showActiveRulesWith(w io.Writer, dump ruleDumper) error {
	chains := []struct {
		bin, table, chain string
	}{
		{"iptables", "filter", "INPUT"},
		{"iptables", "filter", "FORWARD"},
		{"iptables", "nat", "POSTROUTING"},
		{"ip6tables", "filter", "INPUT"},
		{"ip6tables", "filter", "FORWARD"},
		{"ip6tables", "nat", "POSTROUTING"},
	}

	fmt.Fprintln(w, "=== wg-server --showActiveRules: rules matching wg-server shape ===")
	fmt.Fprintln(w, "Heuristic (config-agnostic):")
	fmt.Fprintln(w, "  - FORWARD: any rule with -i or -o starting with \"wg\"")
	fmt.Fprintln(w, "  - POSTROUTING (nat): any rule with -j SNAT or -j MASQUERADE")
	fmt.Fprintln(w, "  - INPUT: any rule with -p udp and -j ACCEPT")

	total := 0
	for _, c := range chains {
		out, err := dump(c.bin, c.table, c.chain)
		if err != nil {
			return fmt.Errorf("dump %s -t %s %s: %w", c.bin, c.table, c.chain, err)
		}
		type hit struct {
			pos    int
			rule   string
			reason string
		}
		var matched []hit
		pos := 0
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "-A ") {
				continue
			}
			pos++
			if reason := wgRuleHeuristic(line, c.chain); reason != "" {
				matched = append(matched, hit{pos: pos, rule: line, reason: reason})
			}
		}
		if len(matched) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s %s/%s:\n", c.bin, c.table, c.chain)
		for _, m := range matched {
			drain := fmt.Sprintf("%s -t %s %s", c.bin, c.table, strings.Replace(m.rule, "-A ", "-D ", 1))
			fmt.Fprintf(w, "  #%d (%s)\n      rule:  %s\n      drain: %s\n", m.pos, m.reason, m.rule, drain)
			total++
		}
	}
	if total == 0 {
		fmt.Fprintln(w, "\n(no matching rules)")
	} else {
		fmt.Fprintf(w, "\nTotal: %d rule(s) matched.\n", total)
	}
	return nil
}

// wgRuleHeuristic returns a non-empty reason if rule (a single `iptables -S`
// line in `chain`) looks like something wg-server might own under SOME
// configuration. Config-agnostic on purpose: callers use it to find stale
// rules from prior runs whose subnet/iface/PublicIP no longer match what
// the current config knows about.
func wgRuleHeuristic(rule, chain string) string {
	if interfaceTokenStartsWith(rule, "wg") {
		return "references wg-prefixed iface"
	}
	if chain == "POSTROUTING" {
		if strings.Contains(rule, "-j SNAT") || strings.Contains(rule, "-j MASQUERADE") {
			return "SNAT/MASQUERADE in POSTROUTING"
		}
	}
	if chain == "INPUT" {
		// UDP ACCEPT in INPUT is uncommon outside VPN/DHCP-server-style
		// setups, so worth surfacing for operator review.
		if hasFlagValue(rule, "-p", "udp") && strings.HasSuffix(rule, "-j ACCEPT") {
			return "UDP ACCEPT in INPUT"
		}
	}
	return ""
}

// interfaceTokenStartsWith reports whether rule (an `iptables -S` line)
// contains an -i or -o flag whose value starts with prefix. Token-level so
// "wgateway0" doesn't false-match a "wg" prefix… wait, actually it does — but
// for this heuristic that's acceptable: a sysadmin who names an unrelated
// iface "wg…" is choosing into the false positive.
func interfaceTokenStartsWith(rule, prefix string) bool {
	fields := strings.Fields(rule)
	for i := 0; i < len(fields)-1; i++ {
		if (fields[i] == "-i" || fields[i] == "-o") && strings.HasPrefix(fields[i+1], prefix) {
			return true
		}
	}
	return false
}
