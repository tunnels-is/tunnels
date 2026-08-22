package types

import (
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strings"
)

var ifaceNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,14}$`)

// ValidIfaceName is the iptables-safe interface name we accept from
// controller records and local config (no trailing +, max 15 chars).
func ValidIfaceName(name string) bool {
	if name == "" || strings.HasSuffix(name, "+") {
		return false
	}
	return ifaceNameRe.MatchString(name)
}

// IPv4Gateway returns the first host address in n (network + 1), using the
// masked prefix so a subnet written as 10.0.0.5/22 still yields 10.0.0.1.
func IPv4Gateway(n *net.IPNet) net.IP {
	if n == nil {
		return nil
	}
	base := n.IP.To4()
	if base == nil {
		return nil
	}
	ip := make(net.IP, 4)
	copy(ip, base.Mask(n.Mask))
	for i := 3; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
	return ip
}

// IPv4Broadcast returns the directed broadcast address of n.
func IPv4Broadcast(n *net.IPNet) net.IP {
	if n == nil {
		return nil
	}
	ip := n.IP.To4()
	if ip == nil {
		return nil
	}
	mask := net.IP(n.Mask).To4()
	if mask == nil {
		return nil
	}
	out := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		out[i] = ip[i] | ^mask[i]
	}
	return out
}

// IsReservedWireGuardIPv4 reports whether ip must not be assigned to a peer.
// Network, gateway (first host), directed broadcast, and the .0/.255 address
// in each /24 inside a larger prefix are reserved.
func IsReservedWireGuardIPv4(ipNet *net.IPNet, ip net.IP) bool {
	if ipNet == nil {
		return true
	}
	ip = ip.To4()
	if ip == nil {
		return true
	}
	if !ipNet.Contains(ip) {
		return true
	}
	network := ipNet.IP.To4().Mask(ipNet.Mask)
	if ip.Equal(network) {
		return true
	}
	if gw := IPv4Gateway(ipNet); gw != nil && ip.Equal(gw) {
		return true
	}
	if bcast := IPv4Broadcast(ipNet); bcast != nil && ip.Equal(bcast) {
		return true
	}
	if ip[3] == 0 || ip[3] == 255 {
		return true
	}
	return false
}

// IsReservedWireGuardIPv6 reports whether addr must not be assigned to a peer
// (network and network+1 / server address).
func IsReservedWireGuardIPv6(prefix netip.Prefix, addr netip.Addr) bool {
	if !prefix.IsValid() || !addr.IsValid() || !addr.Is6() {
		return true
	}
	prefix = prefix.Masked()
	if !prefix.Contains(addr) {
		return true
	}
	netAddr := prefix.Addr()
	return addr == netAddr || addr == netAddr.Next()
}

// ValidateDeviceWireGuardAddrs rejects empty-ok addresses that are reserved
// or outside the server prefix.
func ValidateDeviceWireGuardAddrs(subnet, subnet6, ip, ipv6 string) error {
	if ip != "" {
		if subnet == "" {
			return fmt.Errorf("WireGuard IP set without a server subnet")
		}
		_, ipNet, err := net.ParseCIDR(subnet)
		if err != nil {
			return fmt.Errorf("invalid server subnet %q", subnet)
		}
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			return fmt.Errorf("invalid WireGuard IP %q", ip)
		}
		if IsReservedWireGuardIPv4(ipNet, parsed) {
			return fmt.Errorf("WireGuard IP %s is reserved", ip)
		}
	}
	if ipv6 != "" {
		if subnet6 == "" {
			return fmt.Errorf("WireGuard IPv6 set without a server subnet")
		}
		prefix, err := netip.ParsePrefix(subnet6)
		if err != nil {
			return fmt.Errorf("invalid server IPv6 subnet %q", subnet6)
		}
		addr, err := netip.ParseAddr(ipv6)
		if err != nil || !addr.Is6() {
			return fmt.Errorf("invalid WireGuard IPv6 %q", ipv6)
		}
		if IsReservedWireGuardIPv6(prefix, addr) {
			return fmt.Errorf("WireGuard IPv6 %s is reserved", ipv6)
		}
	}
	return nil
}
