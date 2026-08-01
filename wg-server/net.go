package wgserver

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

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

	flushWGRules(cfg)

	if err := allowWireGuardPort(cfg.WireGuardPort); err != nil {
		return fmt.Errorf("allow WireGuard INPUT port: %w", err)
	}

	if err := addForwardRules(cfg.WireGuardIface, cfg.InternetIface, cfg.WireGuardSubnet6 != ""); err != nil {
		return fmt.Errorf("add FORWARD rules: %w", err)
	}

	if err := addMasquerade(cfg.WireGuardSubnet, cfg.InternetIface); err != nil {
		return fmt.Errorf("add egress NAT: %w", err)
	}

	if cfg.WireGuardSubnet6 != "" {
		if err := addMasquerade6(cfg.WireGuardSubnet6, cfg.InternetIface); err != nil {
			return fmt.Errorf("add IPv6 MASQUERADE: %w", err)
		}
	} else {

		if err := addIPv6Drop(cfg.WireGuardIface); err != nil {
			return fmt.Errorf("add IPv6 DROP: %w", err)
		}
	}

	INFO("network setup complete, server IP=", serverIP.String(), "/", maskBits(ipNet.Mask))
	return nil
}

func cleanupNet(cfg *Config) {

	flushWGRules(cfg)
}

func flushWGRules(cfg *Config) {
	portStr := fmt.Sprintf("%d", cfg.WireGuardPort)
	wg := cfg.WireGuardIface
	net := cfg.InternetIface

	for _, bin := range []string{"iptables", "ip6tables"} {

		drainRule(bin, "-D", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT")

		drainRule(bin, "-D", "FORWARD", "-i", wg, "-o", wg, "-j", "ACCEPT")

		drainRule(bin, "-D", "FORWARD", "-i", wg, "-o", net, "-j", "ACCEPT")

		drainRule(bin, "-D", "FORWARD",
			"-i", net, "-o", wg,
			"-m", "state", "--state", "RELATED,ESTABLISHED",
			"-j", "ACCEPT")

		drainRule(bin, "-D", "FORWARD", "-i", net, "-o", wg, "-j", "DROP")
	}

	if cfg.WireGuardSubnet != "" {
		drainRule("iptables", masqueradeArgs("-D", cfg.WireGuardSubnet, net)...)
		if cfg.PublicIP != "" {
			drainRule("iptables", "-t", "nat", "-D", "POSTROUTING",
				"-s", cfg.WireGuardSubnet, "-o", net, "-j", "SNAT", "--to-source", cfg.PublicIP)
		}
	}

	if cfg.WireGuardSubnet6 != "" {
		drainRule("ip6tables",
			"-t", "nat", "-D", "POSTROUTING",
			"-s", cfg.WireGuardSubnet6, "-o", net, "-j", "MASQUERADE")
	}

	drainRule("ip6tables", "-D", "FORWARD", "-i", wg, "-j", "DROP")
	drainRule("ip6tables", "-D", "FORWARD", "-o", wg, "-j", "DROP")

	if cfg.WireGuardMeshPort > 0 {
		meshPort := fmt.Sprintf("%d", cfg.WireGuardMeshPort)
		mesh := meshIface(cfg)
		for _, bin := range []string{"iptables", "ip6tables"} {
			drainRule(bin, "-D", "INPUT", "-p", "udp", "--dport", meshPort, "-j", "ACCEPT")
		}
		drainRule("iptables", "-D", "FORWARD", "-i", mesh, "-o", wg, "-j", "ACCEPT")
		drainRule("iptables", "-D", "FORWARD", "-i", wg, "-o", mesh, "-j", "ACCEPT")
		drainRule("iptables", "-t", "mangle", "-D", "FORWARD", "-o", mesh,
			"-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")
	}
}

func addMeshRules(cfg *Config) error {
	if cfg.WireGuardMeshPort == 0 {
		return nil
	}
	mesh := meshIface(cfg)
	wg := cfg.WireGuardIface
	portStr := fmt.Sprintf("%d", cfg.WireGuardMeshPort)

	if err := execIPTables("-A", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := execIP6Tables("-A", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := execIPTables("-A", "FORWARD", "-i", mesh, "-o", wg, "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := execIPTables("-A", "FORWARD", "-i", wg, "-o", mesh, "-j", "ACCEPT"); err != nil {
		return err
	}
	return execIPTables("-t", "mangle", "-A", "FORWARD", "-o", mesh,
		"-p", "tcp", "--tcp-flags", "SYN,RST", "SYN", "-j", "TCPMSS", "--clamp-mss-to-pmtu")
}

func drainRule(bin string, args ...string) {
	for {
		if err := exec.Command(bin, args...).Run(); err != nil {
			return
		}
	}
}

func allowWireGuardPort(port int) error {
	portStr := fmt.Sprintf("%d", port)
	if err := execIPTables("-A", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT"); err != nil {
		return err
	}
	return execIP6Tables("-A", "INPUT", "-p", "udp", "--dport", portStr, "-j", "ACCEPT")
}

func addForwardRules(wgIface, netIface string, withIPv6 bool) error {

	bins := []func(...string) error{execIPTables}
	if withIPv6 {
		bins = append(bins, execIP6Tables)
	}
	for _, bin := range bins {
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

		if err := bin("-A", "FORWARD", "-i", netIface, "-o", wgIface, "-j", "DROP"); err != nil {
			return err
		}
	}
	return nil
}

func PreviewRules(cfg *Config) []string {
	var lines []string
	portStr := fmt.Sprintf("%d", cfg.WireGuardPort)
	wg := cfg.WireGuardIface
	net := cfg.InternetIface

	for _, bin := range []string{"iptables", "ip6tables"} {
		lines = append(lines,
			fmt.Sprintf("%s -A INPUT -p udp --dport %s -j ACCEPT", bin, portStr))
	}

	forwardBins := []string{"iptables"}
	if cfg.WireGuardSubnet6 != "" {
		forwardBins = append(forwardBins, "ip6tables")
	}
	for _, bin := range forwardBins {
		lines = append(lines,
			fmt.Sprintf("%s -A FORWARD -i %s -o %s -j ACCEPT", bin, wg, wg),
			fmt.Sprintf("%s -A FORWARD -i %s -o %s -j ACCEPT", bin, wg, net),
			fmt.Sprintf("%s -A FORWARD -i %s -o %s -m state --state RELATED,ESTABLISHED -j ACCEPT", bin, net, wg),
			fmt.Sprintf("%s -A FORWARD -i %s -o %s -j DROP", bin, net, wg),
		)
	}

	if cfg.WireGuardSubnet != "" {
		lines = append(lines, "iptables "+strings.Join(masqueradeArgs("-A", cfg.WireGuardSubnet, net), " "))
	}

	if cfg.WireGuardSubnet6 != "" {
		lines = append(lines,
			fmt.Sprintf("ip6tables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE", cfg.WireGuardSubnet6, net))
	} else {
		lines = append(lines,
			fmt.Sprintf("ip6tables -A FORWARD -i %s -j DROP", wg),
			fmt.Sprintf("ip6tables -A FORWARD -o %s -j DROP", wg),
		)
	}

	return lines
}

func addMasquerade(subnet, iface string) error {
	return execIPTables(masqueradeArgs("-A", subnet, iface)...)
}

func masqueradeArgs(action, subnet, iface string) []string {
	return []string{"-t", "nat", action, "POSTROUTING", "-s", subnet, "-o", iface, "-j", "MASQUERADE"}
}

func addMasquerade6(subnet6, iface string) error {
	return execIP6Tables("-t", "nat", "-A", "POSTROUTING", "-s", subnet6, "-o", iface, "-j", "MASQUERADE")
}

func addIPv6Drop(wgIface string) error {
	if err := execIP6Tables("-A", "FORWARD", "-i", wgIface, "-j", "DROP"); err != nil {
		return err
	}
	return execIP6Tables("-A", "FORWARD", "-o", wgIface, "-j", "DROP")
}

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
