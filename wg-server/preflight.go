package wgserver

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type ruleDumper func(bin, table, chain string) (string, error)

func defaultRuleDumper(bin, table, chain string) (string, error) {
	args := []string{"-t", table, "-S", chain}
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %v: %w: %s", bin, args, err, string(out))
	}
	return string(out), nil
}

type ruleConflict struct {
	bin   string
	table string
	chain string
	pos   int
	field string
	match string
	rule  string
}

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
			pos++
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

func hasFlagValue(rule, flag, value string) bool {
	fields := strings.Fields(rule)
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == flag && fields[i+1] == value {
			return true
		}
	}
	return false
}

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

		if hasFlagValue(rule, "-p", "udp") && strings.HasSuffix(rule, "-j ACCEPT") {
			return "UDP ACCEPT in INPUT"
		}
	}
	return ""
}

func interfaceTokenStartsWith(rule, prefix string) bool {
	fields := strings.Fields(rule)
	for i := 0; i < len(fields)-1; i++ {
		if (fields[i] == "-i" || fields[i] == "-o") && strings.HasPrefix(fields[i+1], prefix) {
			return true
		}
	}
	return false
}
