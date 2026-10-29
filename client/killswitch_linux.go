//go:build linux

package client

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

var (
	killSwitchIPv4Active atomic.Bool
	killSwitchIPv6Active atomic.Bool

	demotedDefaultsMu sync.Mutex
	demotedDefaults   = map[string]int{}

	killSwitchRouteWatchOnce sync.Once
	killSwitchRoutePending   atomic.Bool
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

func isIPv6DefaultRoute(r *netlink.Route) bool {
	if r == nil {
		return false
	}
	if r.Dst == nil {
		return true
	}
	ones, bits := r.Dst.Mask.Size()
	return bits == 128 && ones == 0
}

func mainRouteTable(table int) bool {
	return table == 0 || table == unix.RT_TABLE_MAIN
}

func demoteKey(r netlink.Route) string {
	dst := ""
	if r.Dst != nil {
		dst = r.Dst.String()
	}
	gw := ""
	if r.Gw != nil {
		gw = r.Gw.String()
	}
	return fmt.Sprintf("%d|%d|%d|%s|%s", r.Family, r.LinkIndex, r.Table, dst, gw)
}

func demotePhysicalDefaults(family int) {
	routes, err := netlink.RouteList(nil, family)
	if err != nil {
		return
	}
	tunnels, tunnGws := tunnelLinkIndexes()
	demotedDefaultsMu.Lock()
	defer demotedDefaultsMu.Unlock()
	for i := range routes {
		r := routes[i]
		if !shouldDemoteLinuxRoute(&r, family, tunnels, tunnGws) {
			continue
		}
		key := demoteKey(r)
		if _, ok := demotedDefaults[key]; !ok {
			demotedDefaults[key] = r.Priority
		}
		originalLink := r.LinkIndex
		_ = netlink.RouteDel(&r)
		r.Priority = killSwitchDemotedMetric
		r.LinkIndex = originalLink
		_ = netlink.RouteAdd(&r)
	}
}

func shouldDemoteLinuxRoute(r *netlink.Route, family int, tunnels map[int]struct{}, tunnGws map[string]struct{}) bool {
	if r == nil || !mainRouteTable(r.Table) {
		return false
	}
	def := false
	switch family {
	case netlink.FAMILY_V4:
		def = isIPv4DefaultRoute(r)
	case netlink.FAMILY_V6:
		def = isIPv6DefaultRoute(r)
	}
	_, tunnelLink := tunnels[r.LinkIndex]
	tunnelGw := false
	if r.Gw != nil {
		_, tunnelGw = tunnGws[r.Gw.String()]
	}
	return demotePhysicalDefault(def, r.Type == unix.RTN_BLACKHOLE, tunnelLink, tunnelGw, r.Table == wgProtectTable, r.Priority)
}

func restoreDemotedDefaults(family int) {
	routes, err := netlink.RouteList(nil, family)
	if err != nil {
		return
	}
	demotedDefaultsMu.Lock()
	defer demotedDefaultsMu.Unlock()
	for i := range routes {
		r := routes[i]
		if r.Family != family && r.Family != 0 {
			continue
		}
		key := demoteKey(r)
		orig, ok := demotedDefaults[key]
		if !ok || r.Priority != killSwitchDemotedMetric {
			continue
		}
		_ = netlink.RouteDel(&r)
		r.Priority = orig
		_ = netlink.RouteAdd(&r)
		delete(demotedDefaults, key)
	}
	for key := range demotedDefaults {
		var fam int
		if _, err := fmt.Sscanf(key, "%d|", &fam); err == nil && fam == family {
			delete(demotedDefaults, key)
		}
	}
}

func enableKillSwitchIPv4() error {
	startLinuxKillSwitchRouteWatch()
	demotePhysicalDefaults(netlink.FAMILY_V4)
	if err := netlink.RouteReplace(blackholeRoute("0.0.0.0/0", netlink.FAMILY_V4)); err != nil {
		killSwitchIPv4Active.Store(false)
		return err
	}
	if killSwitchIPv4Active.CompareAndSwap(false, true) {
		INFO("IPv4 kill switch on (blackhole 0.0.0.0/0 metric ", killSwitchMetric, ")")
	}
	return nil
}

func disableKillSwitchIPv4() {
	restoreDemotedDefaults(netlink.FAMILY_V4)
	_ = netlink.RouteDel(blackholeRoute("0.0.0.0/0", netlink.FAMILY_V4))
	if killSwitchIPv4Active.CompareAndSwap(true, false) {
		INFO("IPv4 kill switch off")
	}
}

func enableKillSwitchIPv6() error {
	startLinuxKillSwitchRouteWatch()
	demotePhysicalDefaults(netlink.FAMILY_V6)
	if err := netlink.RouteReplace(blackholeRoute("::/0", netlink.FAMILY_V6)); err != nil {
		killSwitchIPv6Active.Store(false)
		DEBUG("IPv6 kill switch: ", err, " (IPv6 may be disabled)")
		return nil
	}
	if killSwitchIPv6Active.CompareAndSwap(false, true) {
		INFO("IPv6 kill switch on (blackhole ::/0 metric ", killSwitchMetric, ")")
	}
	return nil
}

func disableKillSwitchIPv6() {
	restoreDemotedDefaults(netlink.FAMILY_V6)
	_ = netlink.RouteDel(blackholeRoute("::/0", netlink.FAMILY_V6))
	if killSwitchIPv6Active.CompareAndSwap(true, false) {
		INFO("IPv6 kill switch off")
	}
}

func startLinuxKillSwitchRouteWatch() {
	killSwitchRouteWatchOnce.Do(func() {
		go func() {
			defer RecoverAndLog()
			updates := make(chan netlink.RouteUpdate, 32)
			done := make(chan struct{})
			if err := netlink.RouteSubscribe(updates, done); err != nil {
				DEBUG("kill switch route watch: ", err)
				return
			}
			for range updates {
				if !killSwitchRoutePending.CompareAndSwap(false, true) {
					continue
				}
				time.AfterFunc(200*time.Millisecond, func() {
					killSwitchRoutePending.Store(false)
					cfg := CONFIG.Load()
					if cfg == nil || (!cfg.KillSwitchIPv4 && !cfg.KillSwitchIPv6) {
						return
					}
					_ = syncKillSwitch(cfg.KillSwitchIPv4, cfg.KillSwitchIPv6)
				})
			}
		}()
	})
}
