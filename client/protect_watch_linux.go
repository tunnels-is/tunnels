//go:build linux

package client

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func runProtectWatcher() {
	protectTickerLoop()
}

// hostProtectRouteRequired reports whether a protect host still needs a
// via-gateway /32. On-link destinations are skipped by IP_AddRoute; the
// 5s watcher must treat those as already satisfied or it will reinstall
// every tick.
func hostProtectRouteRequired(host string) bool {
	if host == "" {
		return false
	}
	return !ipv4HostOnLink(host + "/32")
}

func isIPv4DefaultRoute(r *netlink.Route) bool {
	if r == nil {
		return false
	}
	if r.Dst == nil {
		return true
	}
	ones, bits := r.Dst.Mask.Size()
	return bits == 32 && ones == 0
}

func pickPhysicalIPv4Default(routes []netlink.Route, tunnelIdxs map[int]struct{}, tunnelGws map[string]struct{}) *netlink.Route {
	var best *netlink.Route
	for i := range routes {
		r := &routes[i]
		if !isIPv4DefaultRoute(r) {
			continue
		}
		if r.Type == unix.RTN_BLACKHOLE {
			continue
		}
		if _, ok := tunnelIdxs[r.LinkIndex]; ok {
			continue
		}
		if r.Gw != nil {
			if _, ok := tunnelGws[r.Gw.String()]; ok {
				continue
			}
		}
		if r.LinkIndex == 0 {
			continue
		}
		if best == nil || r.Priority < best.Priority {
			best = r
		}
	}
	return best
}

func tunnelLinkIndexes() (idxs map[int]struct{}, gws map[string]struct{}) {
	idxs = make(map[int]struct{})
	gws = make(map[string]struct{})
	tunnelMapRange(func(tun *TUN) bool {
		t := tun.tunnel.Load()
		if t == nil {
			return true
		}
		if t.IPv4Address != "" {
			gws[t.IPv4Address] = struct{}{}
		}
		if t.Name == "" {
			return true
		}
		link, err := netlink.LinkByName(t.Name)
		if err != nil {
			return true
		}
		idxs[link.Attrs().Index] = struct{}{}
		return true
	})
	return idxs, gws
}

func ipv4RouteMatches(r *netlink.Route, ifIndex int, gw net.IP) bool {
	if r == nil || r.LinkIndex != ifIndex || r.Gw == nil || gw == nil {
		return false
	}
	return r.Gw.Equal(gw)
}

func protectRoutesPresent(ifName string, gw net.IP, ifIndex int, hosts []string) bool {
	gw4 := gw.To4()
	if ifName == "" || gw4 == nil || ifIndex == 0 {
		return false
	}
	link, err := netlink.LinkByName(ifName)
	if err != nil || link.Attrs().Index != ifIndex {
		return false
	}

	protectRoutes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{
		Table:  wgProtectTable,
		Family: netlink.FAMILY_V4,
	}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return false
	}
	haveProtect := false
	for i := range protectRoutes {
		r := &protectRoutes[i]
		if isIPv4DefaultRoute(r) && ipv4RouteMatches(r, ifIndex, gw4) {
			haveProtect = true
			break
		}
	}
	if !haveProtect {
		return false
	}

	for _, host := range hosts {
		if !hostProtectRouteRequired(host) {
			continue
		}
		_, dst, err := net.ParseCIDR(host + "/32")
		if err != nil {
			return false
		}
		existing, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: dst}, netlink.RT_FILTER_DST)
		if err != nil {
			return false
		}
		ok := false
		for i := range existing {
			if ipv4RouteMatches(&existing[i], ifIndex, gw4) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func physicalIPv4Default() (ifName string, gw net.IP, ifIndex int, err error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", nil, 0, err
	}
	idxs, gws := tunnelLinkIndexes()
	best := pickPhysicalIPv4Default(routes, idxs, gws)
	if best == nil {
		return "", nil, 0, fmt.Errorf("no physical IPv4 default route")
	}
	link, err := netlink.LinkByIndex(best.LinkIndex)
	if err != nil {
		return "", nil, 0, err
	}
	gw4 := best.Gw.To4()
	if gw4 == nil {
		return "", nil, 0, fmt.Errorf("physical default has no IPv4 gateway")
	}
	return link.Attrs().Name, append(net.IP(nil), gw4...), link.Attrs().Index, nil
}
