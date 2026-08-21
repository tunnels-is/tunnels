//go:build linux

package client

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"github.com/vishvananda/netlink"
	wgconn "golang.zx2c4.com/wireguard/conn"
)

var endpointProtectInstalled atomic.Bool

func bindProtectToInterface(_ wgconn.Bind, _ uint32) error {
	return nil
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
