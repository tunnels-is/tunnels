//go:build linux

package client

import (
	"net"
	"sync/atomic"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const killSwitchMetric = 50

var (
	killSwitchIPv4Active atomic.Bool
	killSwitchIPv6Active atomic.Bool
)

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

func demoteLowMetricDefaults(family int) {
	routes, err := netlink.RouteList(nil, family)
	if err != nil {
		return
	}
	for i := range routes {
		r := routes[i]
		if r.Type == unix.RTN_BLACKHOLE || r.Priority >= killSwitchMetric {
			continue
		}
		if family == netlink.FAMILY_V4 && r.Dst != nil {
			continue
		}
		if family == netlink.FAMILY_V6 && r.Dst != nil && r.Dst.String() != "::/0" {
			continue
		}
		_ = netlink.RouteDel(&r)
		r.Priority = 100
		_ = netlink.RouteAdd(&r)
	}
}

func enableKillSwitchIPv4() error {
	if !killSwitchIPv4Active.CompareAndSwap(false, true) {
		return nil
	}
	demoteLowMetricDefaults(netlink.FAMILY_V4)
	if err := netlink.RouteReplace(blackholeRoute("0.0.0.0/0", netlink.FAMILY_V4)); err != nil {
		killSwitchIPv4Active.Store(false)
		return err
	}
	INFO("IPv4 kill switch on (blackhole 0.0.0.0/0 metric ", killSwitchMetric, ")")
	return nil
}

func disableKillSwitchIPv4() {
	if !killSwitchIPv4Active.CompareAndSwap(true, false) {
		_ = netlink.RouteDel(blackholeRoute("0.0.0.0/0", netlink.FAMILY_V4))
		return
	}
	_ = netlink.RouteDel(blackholeRoute("0.0.0.0/0", netlink.FAMILY_V4))
	INFO("IPv4 kill switch off")
}

func enableKillSwitchIPv6() error {
	if !killSwitchIPv6Active.CompareAndSwap(false, true) {
		return nil
	}
	demoteLowMetricDefaults(netlink.FAMILY_V6)
	if err := netlink.RouteReplace(blackholeRoute("::/0", netlink.FAMILY_V6)); err != nil {
		killSwitchIPv6Active.Store(false)
		DEBUG("IPv6 kill switch: ", err, " (IPv6 may be disabled)")
		return nil
	}
	INFO("IPv6 kill switch on (blackhole ::/0 metric ", killSwitchMetric, ")")
	return nil
}

func disableKillSwitchIPv6() {
	if !killSwitchIPv6Active.CompareAndSwap(true, false) {
		_ = netlink.RouteDel(blackholeRoute("::/0", netlink.FAMILY_V6))
		return
	}
	_ = netlink.RouteDel(blackholeRoute("::/0", netlink.FAMILY_V6))
	INFO("IPv6 kill switch off")
}
