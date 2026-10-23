//go:build linux

package client

import (
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	wgconn "golang.zx2c4.com/wireguard/conn"
)

var endpointProtectInstalled atomic.Bool

func protectBindWraps() bool { return false }

// bindProtectToInterface forces the WireGuard UDP sockets out the physical
// NIC. IP_UNICAST_IF sets sk_bound_dev_if without CAP_NET_RAW. The ifindex
// for IPv4 is passed in network byte order.
func bindProtectToInterface(b wgconn.Bind, ifIndex uint32) error {
	if ifIndex == 0 {
		return fmt.Errorf("interface index is 0")
	}
	v4, v6 := udpConnsFromBind(b)
	if v4 == nil && v6 == nil {
		return fmt.Errorf("no UDP sockets on WireGuard bind")
	}
	if v4 != nil {
		if err := setUnicastIF(v4, ifIndex, false); err != nil {
			return fmt.Errorf("IPv4 IP_UNICAST_IF: %w", err)
		}
	}
	if v6 != nil {
		if err := setUnicastIF(v6, ifIndex, true); err != nil {
			DEBUG("IPv6 IPV6_UNICAST_IF skipped: ", err)
		}
	}
	return nil
}

func setUnicastIF(c *net.UDPConn, ifIndex uint32, v6 bool) error {
	sc, err := c.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	if err := sc.Control(func(fd uintptr) {
		if v6 {
			sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_UNICAST_IF, int(ifIndex))
			return
		}
		sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_UNICAST_IF, ipv4UnicastIfIndex(ifIndex))
	}); err != nil {
		return err
	}
	return sockErr
}

func ipv4UnicastIfIndex(ifIndex uint32) int {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], ifIndex)
	return int(binary.NativeEndian.Uint32(b[:]))
}

func udpConnsFromBind(b wgconn.Bind) (v4, v6 *net.UDPConn) {
	v := reflect.ValueOf(b)
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}
	return reflectedUDPConn(v, "ipv4"), reflectedUDPConn(v, "ipv6")
}

func reflectedUDPConn(v reflect.Value, field string) *net.UDPConn {
	f := v.FieldByName(field)
	if !f.IsValid() || f.Kind() != reflect.Ptr || f.IsNil() {
		return nil
	}
	ptr := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
	c, _ := ptr.Interface().(*net.UDPConn)
	return c
}

func applyEndpointProtect(ifName string, gw net.IP) error {
	if ifName == "" {
		return fmt.Errorf("missing default interface for WireGuard socket protect")
	}
	gw4 := gw.To4()
	if gw4 == nil {
		return fmt.Errorf("WireGuard socket protect requires an IPv4 gateway")
	}
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("protect link %s: %w", ifName, err)
	}

	_, dst, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		return err
	}
	rt := &netlink.Route{
		Table:     wgProtectTable,
		LinkIndex: link.Attrs().Index,
		Gw:        gw4,
		Dst:       dst,
		Family:    netlink.FAMILY_V4,
	}
	if err := netlink.RouteReplace(rt); err != nil {
		return fmt.Errorf("protect route: %w", err)
	}

	mask := uint32(0xffffffff)
	rule := netlink.NewRule()
	rule.Family = netlink.FAMILY_V4
	rule.Table = wgProtectTable
	rule.Mark = uint32(wgProtectMark)
	rule.Mask = &mask
	rule.Priority = wgProtectPrio
	if err := netlink.RuleAdd(rule); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "exist") {
			return fmt.Errorf("protect rule: %w", err)
		}
	}

	endpointProtectInstalled.Store(true)
	startProtectWatcher()
	DEBUG("installed WireGuard socket protect: fwmark ", wgProtectMark,
		" table ", wgProtectTable, " via ", gw4, " dev ", ifName)
	return nil
}

func removeEndpointProtect() {
	remaining := false
	tunnelMapRange(func(_ *TUN) bool {
		remaining = true
		return false
	})
	if remaining {
		return
	}
	if !endpointProtectInstalled.CompareAndSwap(true, false) {
		return
	}

	mask := uint32(0xffffffff)
	rule := netlink.NewRule()
	rule.Family = netlink.FAMILY_V4
	rule.Table = wgProtectTable
	rule.Mark = uint32(wgProtectMark)
	rule.Mask = &mask
	rule.Priority = wgProtectPrio
	if err := netlink.RuleDel(rule); err != nil {
		DEBUG("remove protect rule: ", err)
	}

	_, dst, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		return
	}
	rt := &netlink.Route{
		Table:  wgProtectTable,
		Dst:    dst,
		Family: netlink.FAMILY_V4,
	}
	if err := netlink.RouteDel(rt); err != nil {
		DEBUG("remove protect route: ", err)
	}
}
