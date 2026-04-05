package wgserver

import (
	"fmt"
	"net"
	"os"
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

	if err := enableIPForward(); err != nil {
		return fmt.Errorf("enable ip_forward: %w", err)
	}

	if cfg.WireGuardSubnet6 != "" {
		if err := enableIPv6Forward(); err != nil {
			return fmt.Errorf("enable ipv6 forwarding: %w", err)
		}
	}

	if err := allowWireGuardPort(cfg.WireGuardPort); err != nil {
		return fmt.Errorf("allow WireGuard INPUT port: %w", err)
	}

	if err := addForwardRules(cfg.WireGuardIface, cfg.InternetIface); err != nil {
		return fmt.Errorf("add FORWARD rules: %w", err)
	}

	if err := addMasquerade(cfg.WireGuardSubnet, cfg.InternetIface); err != nil {
		return fmt.Errorf("add MASQUERADE: %w", err)
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
	if !ipForwardWasEnabled {
		if err := disableIPForward(); err != nil {
			WARN("failed to disable ip_forward: ", err)
		}
	}
	if cfg.WireGuardSubnet6 != "" && !ipv6ForwardWasEnabled {
		if err := disableIPv6Forward(); err != nil {
			WARN("failed to disable ipv6 forwarding: ", err)
		}
	}

	if err := denyWireGuardPort(cfg.WireGuardPort); err != nil {
		WARN("failed to remove WireGuard INPUT rule: ", err)
	}
	if err := removeForwardRules(cfg.WireGuardIface, cfg.InternetIface); err != nil {
		WARN("failed to remove FORWARD rules: ", err)
	}
	if err := removeMasquerade(cfg.WireGuardSubnet, cfg.InternetIface); err != nil {
		WARN("failed to remove MASQUERADE rule: ", err)
	}

	if cfg.WireGuardSubnet6 != "" {
		if err := removeMasquerade6(cfg.WireGuardSubnet6, cfg.InternetIface); err != nil {
			WARN("failed to remove IPv6 MASQUERADE rule: ", err)
		}
	} else {
		if err := removeIPv6Drop(cfg.WireGuardIface); err != nil {
			WARN("failed to remove IPv6 DROP rules: ", err)
		}
	}
}

// ---------------------------------------------------------------------------
// IP forwarding
// ---------------------------------------------------------------------------

var (
	ipForwardWasEnabled   bool
	ipv6ForwardWasEnabled bool
)

func enableIPForward() error {
	cur, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err == nil && len(cur) > 0 && cur[0] == '1' {
		ipForwardWasEnabled = true
	}
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
}

func disableIPForward() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("0"), 0644)
}

func enableIPv6Forward() error {
	cur, err := os.ReadFile("/proc/sys/net/ipv6/conf/all/forwarding")
	if err == nil && len(cur) > 0 && cur[0] == '1' {
		ipv6ForwardWasEnabled = true
	}
	return os.WriteFile("/proc/sys/net/ipv6/conf/all/forwarding", []byte("1"), 0644)
}

func disableIPv6Forward() error {
	return os.WriteFile("/proc/sys/net/ipv6/conf/all/forwarding", []byte("0"), 0644)
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

func denyWireGuardPort(port int) error {
	portStr := fmt.Sprintf("%d", port)
	if err := execIPTables("-D", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT"); err != nil {
		return err
	}
	return execIP6Tables("-D", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT")
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
	}
	return nil
}

func removeForwardRules(wgIface, netIface string) error {
	for _, bin := range []func(...string) error{execIPTables, execIP6Tables} {
		if err := bin("-D", "FORWARD", "-i", wgIface, "-o", wgIface, "-j", "ACCEPT"); err != nil {
			return err
		}
		if err := bin("-D", "FORWARD", "-i", wgIface, "-o", netIface, "-j", "ACCEPT"); err != nil {
			return err
		}
		if err := bin(
			"-D", "FORWARD",
			"-i", netIface, "-o", wgIface,
			"-m", "state", "--state", "RELATED,ESTABLISHED",
			"-j", "ACCEPT",
		); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// MASQUERADE (IPv4 uses iptables, IPv6 uses ip6tables)
// ---------------------------------------------------------------------------

func addMasquerade(subnet, iface string) error {
	return execIPTables("-t", "nat", "-A", "POSTROUTING", "-s", subnet, "-o", iface, "-j", "MASQUERADE")
}

func removeMasquerade(subnet, iface string) error {
	return execIPTables("-t", "nat", "-D", "POSTROUTING", "-s", subnet, "-o", iface, "-j", "MASQUERADE")
}

func addMasquerade6(subnet6, iface string) error {
	return execIP6Tables("-t", "nat", "-A", "POSTROUTING", "-s", subnet6, "-o", iface, "-j", "MASQUERADE")
}

func removeMasquerade6(subnet6, iface string) error {
	return execIP6Tables("-t", "nat", "-D", "POSTROUTING", "-s", subnet6, "-o", iface, "-j", "MASQUERADE")
}

// ---------------------------------------------------------------------------
// Cross-server masquerade exclusion (dual-stack)
// ---------------------------------------------------------------------------

func addCrossServerMasqueradeExclusion(peerSubnet, iface string) error {
	args := []string{"-t", "nat", "-C", "POSTROUTING",
		"-s", peerSubnet, "-o", iface, "-j", "RETURN"}
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
	args := []string{"-t", "nat", "-C", "POSTROUTING",
		"-s", peerSubnet6, "-o", iface, "-j", "RETURN"}
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

func removeIPv6Drop(wgIface string) error {
	if err := execIP6Tables("-D", "FORWARD", "-i", wgIface, "-j", "DROP"); err != nil {
		return err
	}
	return execIP6Tables("-D", "FORWARD", "-o", wgIface, "-j", "DROP")
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
