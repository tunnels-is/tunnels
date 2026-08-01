//go:build linux

package client

import (
	"net"
	"sync/atomic"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const killSwitchMetric = 50

var killSwitchActive atomic.Bool

func killSwitchSupported() bool { return true }

func blackholeRoute(cidr string, family int) *netlink.Route {
	_, dst, _ := net.ParseCIDR(cidr)
	return &netlink.Route{
		Type:     unix.RTN_BLACKHOLE,
		Dst:      dst,
		Priority: killSwitchMetric,
		Family:   family,
	}
}

func demoteLowMetricDefaults() {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return
	}
	for i := range routes {
		r := routes[i]
		if r.Dst != nil || r.Type == unix.RTN_BLACKHOLE || r.Priority >= killSwitchMetric {
			continue
		}
		_ = netlink.RouteDel(&r)
		r.Priority = 100
		_ = netlink.RouteAdd(&r)
	}
}

func enableKillSwitch() error {
	if !killSwitchActive.CompareAndSwap(false, true) {
		return nil
	}

	demoteLowMetricDefaults()
	if err := netlink.RouteReplace(blackholeRoute("0.0.0.0/0", netlink.FAMILY_V4)); err != nil {
		killSwitchActive.Store(false)
		return err
	}

	_ = netlink.RouteReplace(blackholeRoute("::/0", netlink.FAMILY_V6))
	return nil
}

func disableKillSwitch() {
	if !killSwitchActive.CompareAndSwap(true, false) {
		return
	}
	_ = netlink.RouteDel(blackholeRoute("0.0.0.0/0", netlink.FAMILY_V4))
	_ = netlink.RouteDel(blackholeRoute("::/0", netlink.FAMILY_V6))
}
