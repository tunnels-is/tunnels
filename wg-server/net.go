package wgserver

import (
	"fmt"
	"net"
	"os/exec"

	"github.com/vishvananda/netlink"
)

func setupNet(cfg *Config) error {
	link, err := netlink.LinkByName(cfg.WireGuardIface)
	if err != nil {
		return fmt.Errorf("interface %q not found: %w", cfg.WireGuardIface, err)
	}

	_, ipNet, err := net.ParseCIDR(cfg.WireGuardSubnet)
	if err != nil {
		return fmt.Errorf("invalid WireGuardSubnet %q: %w", cfg.WireGuardSubnet, err)
	}

	serverIP := firstHost(ipNet)
	addr := &netlink.Addr{
		IPNet: &net.IPNet{IP: serverIP, Mask: ipNet.Mask},
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("AddrAdd %s: %w", serverIP, err)
	}

	// If an IPv6 subnet is configured, assign the first host address to the interface.
	if cfg.WireGuardSubnet6 != "" {
		_, ipNet6, err := net.ParseCIDR(cfg.WireGuardSubnet6)
		if err != nil {
			return fmt.Errorf("invalid WireGuardSubnet6 %q: %w", cfg.WireGuardSubnet6, err)
		}
		serverIPv6 := firstHost6(ipNet6)
		addr6 := &netlink.Addr{
			IPNet: &net.IPNet{IP: serverIPv6, Mask: ipNet6.Mask},
		}
		if err := netlink.AddrAdd(link, addr6); err != nil {
			return fmt.Errorf("AddrAdd IPv6 %s: %w", serverIPv6, err)
		}
		INFO("IPv6 address assigned: ", serverIPv6.String(), "/", maskBits(ipNet6.Mask))
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("LinkSetUp: %w", err)
	}

	// ip_forward / ipv6 forwarding is the operator's responsibility — must be
	// enabled before wg-server starts (e.g. via sysctl.d). The wg-server no
	// longer flips these sysctls, since the shared-state bookkeeping breaks
	// down when multiple instances run on one host.

	// Phase 1: drain any rules left over from a previous (possibly unclean) run.
	// Phase 2 (below) then installs a single fresh copy of each. This guarantees
	// exactly one instance of every rule regardless of prior state.
	flushWGRules(cfg)

	if err := allowWireGuardPort(cfg.WireGuardPort); err != nil {
		return fmt.Errorf("allow WireGuard INPUT port: %w", err)
	}

	if err := addForwardRules(cfg.WireGuardIface, cfg.InternetIface); err != nil {
		return fmt.Errorf("add FORWARD rules: %w", err)
	}

	if err := addMasquerade(cfg.WireGuardSubnet, cfg.InternetIface, cfg.PublicIP); err != nil {
		return fmt.Errorf("add egress NAT: %w", err)
	}
	if cfg.PublicIP != "" {
		INFO("egress SNAT pinned to ", cfg.PublicIP, " for ", cfg.WireGuardSubnet)
	}

	// IPv6-specific rules.
	if cfg.WireGuardSubnet6 != "" {
		if err := addMasquerade6(cfg.WireGuardSubnet6, cfg.InternetIface); err != nil {
			return fmt.Errorf("add IPv6 MASQUERADE: %w", err)
		}
	} else {
		// No IPv6 subnet — drop IPv6 forwarding through the WireGuard interface.
		if err := addIPv6Drop(cfg.WireGuardIface); err != nil {
			return fmt.Errorf("add IPv6 DROP: %w", err)
		}
	}

	INFO("network setup complete, server IP=", serverIP.String(), "/", maskBits(ipNet.Mask))
	return nil
}

func cleanupNet(cfg *Config) {
	// Drain every rule wg-server installs. Safe if rules are already absent.
	// We deliberately leave ip_forward and ipv6 forwarding alone — they're
	// managed by the operator (sysctl.d or similar).
	flushWGRules(cfg)
}

// flushWGRules drain-deletes every iptables/ip6tables rule that wg-server
// owns, repeating -D until no matching rule remains in each chain. Used both
// at startup (to clear any leftover state from a crash or ungraceful restart)
// and at shutdown (to leave the host with a clean table).
//
// Errors are intentionally ignored: iptables returns non-zero when no matching
// rule is found, which is the expected stop condition for the drain loop and
// the normal case when the rule was never installed.
func flushWGRules(cfg *Config) {
	portStr := fmt.Sprintf("%d", cfg.WireGuardPort)
	wg := cfg.WireGuardIface
	net := cfg.InternetIface

	for _, bin := range []string{"iptables", "ip6tables"} {
		// INPUT udp:<port> ACCEPT
		drainRule(bin, "-D", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT")
		// FORWARD wg -> wg
		drainRule(bin, "-D", "FORWARD", "-i", wg, "-o", wg, "-j", "ACCEPT")
		// FORWARD wg -> net
		drainRule(bin, "-D", "FORWARD", "-i", wg, "-o", net, "-j", "ACCEPT")
		// FORWARD net -> wg (RELATED,ESTABLISHED)
		drainRule(bin, "-D", "FORWARD",
			"-i", net, "-o", wg,
			"-m", "state", "--state", "RELATED,ESTABLISHED",
			"-j", "ACCEPT")
		// FORWARD net -> wg DROP (unsolicited inbound)
		drainRule(bin, "-D", "FORWARD", "-i", net, "-o", wg, "-j", "DROP")
	}

	// IPv4 egress NAT — drain both shapes (MASQUERADE and the pinned SNAT)
	// so a config change in PublicIP cleans up across restarts.
	if cfg.WireGuardSubnet != "" {
		drainRule("iptables", masqueradeArgs("-D", cfg.WireGuardSubnet, net, "")...)
		if cfg.PublicIP != "" {
			drainRule("iptables", masqueradeArgs("-D", cfg.WireGuardSubnet, net, cfg.PublicIP)...)
		}
	}
	// IPv6 MASQUERADE
	if cfg.WireGuardSubnet6 != "" {
		drainRule("ip6tables",
			"-t", "nat", "-D", "POSTROUTING",
			"-s", cfg.WireGuardSubnet6, "-o", net, "-j", "MASQUERADE")
	}
	// IPv6 DROP (installed when no v6 subnet is configured)
	drainRule("ip6tables", "-D", "FORWARD", "-i", wg, "-j", "DROP")
	drainRule("ip6tables", "-D", "FORWARD", "-o", wg, "-j", "DROP")
}

// drainRule runs `bin args...` (with args[0] = "-D ...") repeatedly until it
// fails. iptables returns a non-zero exit when no matching rule is found, so
// this cleanly removes 0..N duplicates in a single call.
func drainRule(bin string, args ...string) {
	for {
		if err := exec.Command(bin, args...).Run(); err != nil {
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Port rules (applied to both iptables and ip6tables)
// ---------------------------------------------------------------------------

func allowWireGuardPort(port int) error {
	portStr := fmt.Sprintf("%d", port)
	if err := execIPTables("-A", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT"); err != nil {
		return err
	}
	return execIP6Tables("-A", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT")
}

// ---------------------------------------------------------------------------
// FORWARD rules (applied to both iptables and ip6tables)
// ---------------------------------------------------------------------------

func addForwardRules(wgIface, netIface string) error {
	for _, bin := range []func(...string) error{execIPTables, execIP6Tables} {
		if err := bin("-A", "FORWARD", "-i", wgIface, "-o", wgIface, "-j", "ACCEPT"); err != nil {
			return err
		}
		if err := bin("-A", "FORWARD", "-i", wgIface, "-o", netIface, "-j", "ACCEPT"); err != nil {
			return err
		}
		if err := bin(
			"-A", "FORWARD",
			"-i", netIface, "-o", wgIface,
			"-m", "state", "--state", "RELATED,ESTABLISHED",
			"-j", "ACCEPT",
		); err != nil {
			return err
		}
		// Anything from the internet to a WG peer that didn't match the
		// ESTABLISHED/RELATED rule above is unsolicited inbound — drop it.
		// Must be appended after the stateful ACCEPT so legitimate replies
		// match first.
		if err := bin("-A", "FORWARD", "-i", netIface, "-o", wgIface, "-j", "DROP"); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// MASQUERADE (IPv4 uses iptables, IPv6 uses ip6tables)
// ---------------------------------------------------------------------------

// addMasquerade installs the POSTROUTING NAT rule for WG egress.
// When publicIP is set, the rule pins the SNAT source to that address. This
// matters on multi-homed hosts where MASQUERADE would otherwise auto-select
// the interface's primary IP — which is rarely the desired public IP.
// When publicIP is empty, fall back to MASQUERADE for single-homed setups.
func addMasquerade(subnet, iface, publicIP string) error {
	return execIPTables(masqueradeArgs("-A", subnet, iface, publicIP)...)
}

// masqueradeArgs builds the iptables argument list for the POSTROUTING NAT
// rule. Action is "-A" to install or "-D" to drain.
func masqueradeArgs(action, subnet, iface, publicIP string) []string {
	args := []string{"-t", "nat", action, "POSTROUTING", "-s", subnet, "-o", iface, "-j"}
	if publicIP != "" {
		return append(args, "SNAT", "--to-source", publicIP)
	}
	return append(args, "MASQUERADE")
}

func addMasquerade6(subnet6, iface string) error {
	return execIP6Tables("-t", "nat", "-A", "POSTROUTING", "-s", subnet6, "-o", iface, "-j", "MASQUERADE")
}

// ---------------------------------------------------------------------------
// Cross-server masquerade exclusion (dual-stack)
// ---------------------------------------------------------------------------

func addCrossServerMasqueradeExclusion(peerSubnet, iface string) error {
	args := []string{
		"-t", "nat", "-C", "POSTROUTING",
		"-s", peerSubnet, "-o", iface, "-j", "RETURN",
	}
	out, err := exec.Command("iptables", args...).CombinedOutput()
	if err == nil {
		return nil
	}
	_ = out
	return execIPTables("-t", "nat", "-I", "POSTROUTING", "1",
		"-s", peerSubnet, "-o", iface, "-j", "RETURN")
}

func removeCrossServerMasqueradeExclusion(peerSubnet, iface string) error {
	return execIPTables("-t", "nat", "-D", "POSTROUTING",
		"-s", peerSubnet, "-o", iface, "-j", "RETURN")
}

func addCrossServerMasqueradeExclusion6(peerSubnet6, iface string) error {
	args := []string{
		"-t", "nat", "-C", "POSTROUTING",
		"-s", peerSubnet6, "-o", iface, "-j", "RETURN",
	}
	out, err := exec.Command("ip6tables", args...).CombinedOutput()
	if err == nil {
		return nil
	}
	_ = out
	return execIP6Tables("-t", "nat", "-I", "POSTROUTING", "1",
		"-s", peerSubnet6, "-o", iface, "-j", "RETURN")
}

func removeCrossServerMasqueradeExclusion6(peerSubnet6, iface string) error {
	return execIP6Tables("-t", "nat", "-D", "POSTROUTING",
		"-s", peerSubnet6, "-o", iface, "-j", "RETURN")
}

// ---------------------------------------------------------------------------
// IPv6 DROP (when no IPv6 subnet is configured)
// ---------------------------------------------------------------------------

func addIPv6Drop(wgIface string) error {
	if err := execIP6Tables("-A", "FORWARD", "-i", wgIface, "-j", "DROP"); err != nil {
		return err
	}
	return execIP6Tables("-A", "FORWARD", "-o", wgIface, "-j", "DROP")
}

// ---------------------------------------------------------------------------
// Exec helpers
// ---------------------------------------------------------------------------

func execIPTables(args ...string) error {
	out, err := exec.Command("iptables", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables %v: %w: %s", args, err, string(out))
	}
	return nil
}

func execIP6Tables(args ...string) error {
	out, err := exec.Command("ip6tables", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip6tables %v: %w: %s", args, err, string(out))
	}
	return nil
}

// ---------------------------------------------------------------------------
// IP helpers
// ---------------------------------------------------------------------------

func firstHost(n *net.IPNet) net.IP {
	ip := make(net.IP, 4)
	base := n.IP.To4()
	copy(ip, base)
	ip[3]++
	return ip
}

func firstHost6(n *net.IPNet) net.IP {
	ip := make(net.IP, net.IPv6len)
	copy(ip, n.IP.To16())
	// Increment the 128-bit address by 1, carrying from the low byte.
	for i := net.IPv6len - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
	return ip
}

func maskBits(mask net.IPMask) int {
	ones, _ := mask.Size()
	return ones
}
