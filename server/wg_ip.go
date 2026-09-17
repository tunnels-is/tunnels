package main

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func assignNextWireGuardIP(serverID uuid.UUID) (string, error) {
	server, err := findServerByID(serverID)
	if err != nil || server == nil {
		return "", fmt.Errorf("server not found")
	}
	if server.WireGuardSubnet == "" {
		return "", fmt.Errorf("server has no WireGuard subnet configured")
	}
	_, ipNet, err := net.ParseCIDR(server.WireGuardSubnet)
	if err != nil {
		return "", fmt.Errorf("invalid subnet %q: %w", server.WireGuardSubnet, err)
	}

	devices, err := getAllDevices()
	if err != nil {
		return "", fmt.Errorf("list devices: %w", err)
	}

	used := make(map[uint32]bool)
	for _, d := range devices {
		if d.WireGuardIP == "" {
			continue
		}
		ip4 := net.ParseIP(d.WireGuardIP).To4()
		if ip4 != nil && ipNet.Contains(ip4) {
			used[wgIPToU32(ip4)] = true
		}
	}

	base := wgIPToU32(ipNet.IP.To4()) + 2
	for {
		next := wgU32ToIP(base)
		if !ipNet.Contains(next) {
			return "", fmt.Errorf("WireGuard subnet %s is exhausted", server.WireGuardSubnet)
		}
		if types.IsReservedWireGuardIPv4(ipNet, next) {
			base++
			continue
		}
		if !used[base] {
			return next.String(), nil
		}
		base++
	}
}

func wgIPToU32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func wgU32ToIP(n uint32) net.IP {
	return net.IP{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}

func assignNextWireGuardIPv6(serverID uuid.UUID) (string, error) {
	server, err := findServerByID(serverID)
	if err != nil || server == nil {
		return "", fmt.Errorf("server not found")
	}
	if server.WireGuardSubnet6 == "" {
		return "", nil
	}
	prefix, err := netip.ParsePrefix(server.WireGuardSubnet6)
	if err != nil {
		return "", fmt.Errorf("invalid IPv6 subnet %q: %w", server.WireGuardSubnet6, err)
	}

	prefix = prefix.Masked()

	devices, err := getAllDevices()
	if err != nil {
		return "", fmt.Errorf("list devices: %w", err)
	}

	used := make(map[netip.Addr]bool)
	for _, d := range devices {
		if d.WireGuardIPv6 == "" {
			continue
		}
		if addr, parseErr := netip.ParseAddr(d.WireGuardIPv6); parseErr == nil && prefix.Contains(addr) {
			used[addr] = true
		}
	}

	candidate := prefix.Addr().Next().Next()
	for prefix.Contains(candidate) {
		if types.IsReservedWireGuardIPv6(prefix, candidate) {
			candidate = candidate.Next()
			continue
		}
		if !used[candidate] {
			return candidate.String(), nil
		}
		candidate = candidate.Next()
	}
	return "", fmt.Errorf("WireGuard IPv6 subnet %s is exhausted", server.WireGuardSubnet6)
}
