//go:build linux

package client

import (
	"net"
	"sync/atomic"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Route-based kill switch (Linux).
//
// When a default-route tunnel drops, the kernel would otherwise fall back to
// the physical default gateway and leak traffic in the clear. We install a
// blackhole default route at metric killSwitchMetric, which sits *between*:
//   - the /32 host routes to the control/VPN endpoints (metric 0, pinned via
//     the physical gateway at connect time and surviving tunnel teardown), so a
//     reconnect handshake can still reach those endpoints; and
//   - the physical default route (bumped to metric 100 by
//     AdjustRoutersForTunneling), so every *other* destination is dropped.
//
// The blackhole is removed once the tunnel reconnects (its own metric-0 default
// route then carries traffic) or on an explicit disconnect.
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

// demoteLowMetricDefaults raises any non-blackhole IPv4 default route whose
// metric would otherwise beat the kill-switch blackhole (metric < killSwitchMetric)
// up to 100, so the blackhole reliably wins while the tunnel is down. Handles the
// case where a post-startup event (DHCP renew, link change) reinstalled a
// low-metric physical default after AdjustRoutersForTunneling ran. Idempotent.
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
	// Ensure no physical default route can out-prioritize the blackhole.
	demoteLowMetricDefaults()
	if err := netlink.RouteReplace(blackholeRoute("0.0.0.0/0", netlink.FAMILY_V4)); err != nil {
		killSwitchActive.Store(false)
		return err
	}
	// IPv6 may be disabled on the host — best effort.
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
