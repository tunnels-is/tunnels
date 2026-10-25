//go:build linux

package client

import (
	"net"

	"github.com/vishvananda/netlink"
)

// addPeerMTURoute installs a /32 on the tunnel with its own MTU.
// It does not go through addIPv4Route: that helper skips /32s that sit on a
// connected prefix, and it has no MTU field.
func addPeerMTURoute(ifName, ip string, mtu int) error {
	link, dst, err := peerRoute(ifName, ip)
	if err != nil {
		return err
	}
	r := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Scope:     netlink.SCOPE_LINK,
		MTU:       mtu,
	}
	return netlink.RouteReplace(r)
}

func delPeerMTURoute(ifName, ip string) error {
	link, dst, err := peerRoute(ifName, ip)
	if err != nil {
		return err
	}
	existing, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: dst}, netlink.RT_FILTER_DST)
	if err != nil {
		return err
	}
	var first error
	for i := range existing {
		if existing[i].LinkIndex != link.Attrs().Index {
			continue
		}
		if err := netlink.RouteDel(&existing[i]); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func peerRoute(ifName, ip string) (netlink.Link, *net.IPNet, error) {
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return nil, nil, err
	}
	_, dst, err := net.ParseCIDR(ip + "/32")
	if err != nil {
		return nil, nil, err
	}
	return link, dst, nil
}

func egressMTU(ip net.IP) int {
	if ip == nil {
		return 0
	}
	routes, err := netlink.RouteGet(ip)
	if err != nil || len(routes) == 0 {
		return 0
	}
	link, err := netlink.LinkByIndex(routes[0].LinkIndex)
	if err != nil || link == nil || link.Attrs() == nil {
		return 0
	}
	return link.Attrs().MTU
}
