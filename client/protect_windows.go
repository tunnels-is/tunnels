//go:build windows

package client

import (
	"fmt"
	"net"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	wgconn "golang.zx2c4.com/wireguard/conn"
)

func applyEndpointProtect(string, net.IP) error { return nil }

func removeEndpointProtect() {}

func runProtectWatcher() { protectTickerLoop() }

func protectRoutesPresent(ifName string, gw net.IP, ifIndex int, hosts []string) bool {
	gw4 := gw.To4()
	if ifName == "" || gw4 == nil {
		return false
	}
	iface, err := net.InterfaceByName(ifName)
	if err != nil || iface.Flags&net.FlagUp == 0 {
		return false
	}
	if ifIndex != 0 && iface.Index != ifIndex {
		return false
	}
	gws, err := windowsIPv4Gateways()
	if err != nil {
		return false
	}
	agw, ok := gws[uint32(iface.Index)]
	if !ok || agw == nil || !agw.Equal(gw4) {
		return false
	}
	out, err := hiddenCommand("route", "print", "-4").CombinedOutput()
	if err != nil {
		return false
	}
	table := string(out)
	for _, host := range hosts {
		if !strings.Contains(table, host) {
			return false
		}
	}
	return true
}

func ipFromSocketAddress(sa windows.SocketAddress) net.IP {
	if sa.Sockaddr == nil || sa.SockaddrLength < 8 {
		return nil
	}
	switch sa.Sockaddr.Addr.Family {
	case windows.AF_INET:
		raw := (*windows.RawSockaddrInet4)(unsafe.Pointer(sa.Sockaddr))
		ip := make(net.IP, 4)
		copy(ip, raw.Addr[:])
		return ip
	default:
		return nil
	}
}

func windowsIPv4Gateways() (map[uint32]net.IP, error) {
	var size uint32 = 15000
	flags := uint32(windows.GAA_FLAG_INCLUDE_GATEWAYS | windows.GAA_FLAG_SKIP_ANYCAST |
		windows.GAA_FLAG_SKIP_MULTICAST | windows.GAA_FLAG_SKIP_DNS_SERVER)
	for {
		buf := make([]byte, size)
		aa := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
		err := windows.GetAdaptersAddresses(windows.AF_INET, flags, 0, aa, &size)
		if err == windows.ERROR_BUFFER_OVERFLOW {
			continue
		}
		if err != nil {
			return nil, err
		}
		out := make(map[uint32]net.IP)
		for ; aa != nil; aa = aa.Next {
			if aa.OperStatus != windows.IfOperStatusUp {
				continue
			}
			if aa.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK || aa.IfType == windows.IF_TYPE_TUNNEL {
				continue
			}
			if aa.FirstGatewayAddress == nil {
				continue
			}
			ip := ipFromSocketAddress(aa.FirstGatewayAddress.Address)
			if ip4 := ip.To4(); ip4 != nil {
				out[aa.IfIndex] = append(net.IP(nil), ip4...)
			}
		}
		return out, nil
	}
}

func physicalIPv4Default() (ifName string, gw net.IP, ifIndex int, err error) {
	gws, err := windowsIPv4Gateways()
	if err != nil {
		return "", nil, 0, err
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", nil, 0, err
	}
	tunnels := tunnelNameSet()
	tunnGws := tunnelGatewaySet()
	preferred := ""
	if n := STATE.Load().DefaultInterfaceName.Load(); n != nil {
		preferred = *n
	}
	pick := func(iface net.Interface) (string, net.IP, int, bool) {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			return "", nil, 0, false
		}
		if _, ok := tunnels[iface.Name]; ok {
			return "", nil, 0, false
		}
		g, ok := gws[uint32(iface.Index)]
		if !ok || g == nil {
			return "", nil, 0, false
		}
		if _, isTun := tunnGws[g.String()]; isTun {
			return "", nil, 0, false
		}
		return iface.Name, g, iface.Index, true
	}
	if preferred != "" {
		for _, iface := range ifaces {
			if iface.Name != preferred {
				continue
			}
			if n, g, idx, ok := pick(iface); ok {
				return n, g, idx, nil
			}
		}
	}
	for _, iface := range ifaces {
		if n, g, idx, ok := pick(iface); ok {
			return n, g, idx, nil
		}
	}
	return "", nil, 0, fmt.Errorf("no physical IPv4 default route")
}

func bindProtectToInterface(b wgconn.Bind, ifIndex uint32) error {
	if ifIndex == 0 {
		return fmt.Errorf("interface index is 0")
	}
	bsi, ok := b.(wgconn.BindSocketToInterface)
	if !ok {
		return fmt.Errorf("WireGuard bind does not support interface binding")
	}
	if err := bsi.BindSocketToInterface4(ifIndex, false); err != nil {
		return fmt.Errorf("IPv4 IP_UNICAST_IF: %w", err)
	}
	if err := bsi.BindSocketToInterface6(ifIndex, false); err != nil {
		DEBUG("IPv6 IP_UNICAST_IF skipped: ", err)
	}
	return nil
}
