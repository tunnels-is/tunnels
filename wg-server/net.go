package main

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

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("LinkSetUp: %w", err)
	}

	if err := enableIPForward(); err != nil {
		return fmt.Errorf("enable ip_forward: %w", err)
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

	INFO("network setup complete, server IP=", serverIP.String(), "/", maskBits(ipNet.Mask))
	return nil
}

func cleanupNet(cfg *Config) {
	if err := denyWireGuardPort(cfg.WireGuardPort); err != nil {
		WARN("failed to remove WireGuard INPUT rule: ", err)
	}
	if err := removeForwardRules(cfg.WireGuardIface, cfg.InternetIface); err != nil {
		WARN("failed to remove FORWARD rules: ", err)
	}
	if err := removeMasquerade(cfg.WireGuardSubnet, cfg.InternetIface); err != nil {
		WARN("failed to remove MASQUERADE rule: ", err)
	}
}

func enableIPForward() error {
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
}

func allowWireGuardPort(port int) error {
	return execIPTables("-A", "INPUT", "-p", "udp", "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT")
}

func denyWireGuardPort(port int) error {
	return execIPTables("-D", "INPUT", "-p", "udp", "--dport", fmt.Sprintf("%d", port), "-j", "ACCEPT")
}

func addForwardRules(wgIface, netIface string) error {

	if err := execIPTables("-A", "FORWARD", "-i", wgIface, "-o", wgIface, "-j", "ACCEPT"); err != nil {
		return err
	}

	if err := execIPTables("-A", "FORWARD", "-i", wgIface, "-o", netIface, "-j", "ACCEPT"); err != nil {
		return err
	}

	return execIPTables(
		"-A", "FORWARD",
		"-i", netIface, "-o", wgIface,
		"-m", "state", "--state", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	)
}

func removeForwardRules(wgIface, netIface string) error {
	if err := execIPTables("-D", "FORWARD", "-i", wgIface, "-o", wgIface, "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := execIPTables("-D", "FORWARD", "-i", wgIface, "-o", netIface, "-j", "ACCEPT"); err != nil {
		return err
	}
	return execIPTables(
		"-D", "FORWARD",
		"-i", netIface, "-o", wgIface,
		"-m", "state", "--state", "RELATED,ESTABLISHED",
		"-j", "ACCEPT",
	)
}

func addMasquerade(subnet, iface string) error {
	return execIPTables("-t", "nat", "-A", "POSTROUTING", "-s", subnet, "-o", iface, "-j", "MASQUERADE")
}

func removeMasquerade(subnet, iface string) error {
	return execIPTables("-t", "nat", "-D", "POSTROUTING", "-s", subnet, "-o", iface, "-j", "MASQUERADE")
}

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

func execIPTables(args ...string) error {
	out, err := exec.Command("iptables", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("iptables %v: %w: %s", args, err, string(out))
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

func maskBits(mask net.IPMask) int {
	ones, _ := mask.Size()
	return ones
}
