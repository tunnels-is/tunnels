//go:build freebsd || linux || openbsd

package client

import (
	"net"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
)

// ipv4HostOnLink reports whether dest is already covered by a connected
// IPv4 prefix. A via-gateway /32 for an on-link peer (same podman/LAN
// subnet) breaks reachability: traffic hairpins through the default
// gateway instead of going L2.
func ipv4HostOnLink(network string) bool {
	if network == "" || network == "default" {
		return false
	}
	var host net.IP
	if _, dst, err := net.ParseCIDR(network); err == nil && dst != nil {
		ones, bits := dst.Mask.Size()
		if bits != 32 || ones == 0 {
			return false
		}
		host = dst.IP.To4()
	} else {
		host = net.ParseIP(network).To4()
	}
	if host == nil {
		return false
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok || n.IP.To4() == nil {
				continue
			}
			ones, bits := n.Mask.Size()
			if bits != 32 || ones == 0 || ones >= 32 {
				continue
			}
			if n.Contains(host) {
				return true
			}
		}
	}
	return false
}

func addIPv4Route(
	network string,
	ifName string,
	gateway string,
	metric string,
) (err error) {
	if ipv4HostOnLink(network) {
		DEBUG("skip route add, dest on-link: ", network)
		return nil
	}

	mInt, err := strconv.Atoi(metric)
	if err != nil {
		return err
	}

	r := new(netlink.Route)
	if network == "default" {
		_, r.Dst, _ = net.ParseCIDR("0.0.0.0/0")
	} else {
		_, r.Dst, err = net.ParseCIDR(network)
		if err != nil {
			return err
		}
	}

	r.Priority = mInt
	if gw := net.ParseIP(gateway); gw != nil {
		r.Gw = gw.To4()
	}

	if ifName != "" {
		link, lerr := netlink.LinkByName(ifName)
		if lerr != nil {
			return lerr
		}
		r.LinkIndex = link.Attrs().Index
		already, err := replaceConflictingIPv4Route(r, network, gateway, metric)
		if err != nil {
			return err
		}
		if already {
			return nil
		}
	} else {
		_ = delIPv4Route(network, gateway, metric)
	}

	err = netlink.RouteReplace(r)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			DEBUG("IPv4 route already exists")
			return nil
		}
		return err
	}

	DEBUG(
		"ip ",
		"route ",
		"add ",
		network,
		" via ",
		gateway,
		" dev ",
		ifName,
		" metric ",
		metric,
	)
	return
}

func replaceConflictingIPv4Route(r *netlink.Route, network, gateway, metric string) (alreadyPresent bool, err error) {
	if r.Dst == nil {
		return false, nil
	}
	ones, bits := r.Dst.Mask.Size()
	if ones == bits {
		existing, _ := netlink.RouteListFiltered(netlink.FAMILY_V4, &netlink.Route{Dst: r.Dst}, netlink.RT_FILTER_DST)
		if len(existing) == 1 && existing[0].LinkIndex == r.LinkIndex &&
			existing[0].Gw != nil && r.Gw != nil && existing[0].Gw.Equal(r.Gw) {
			return true, nil
		}
		for i := range existing {
			_ = netlink.RouteDel(&existing[i])
		}
		return false, nil
	}
	_ = delIPv4Route(network, gateway, metric)
	return false, nil
}

func addIPv6Route(
	network string,
	ifName string,
	gateway string,
	metric string,
) (err error) {
	_ = delIPv6Route(network, gateway, metric)

	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return err
	}

	mInt, err := strconv.Atoi(metric)
	if err != nil {
		return err
	}

	r := new(netlink.Route)
	r.LinkIndex = link.Attrs().Index
	r.Priority = mInt
	if network == "default" {
		_, r.Dst, _ = net.ParseCIDR("::/0")
	} else {
		_, r.Dst, err = net.ParseCIDR(network)
		if err != nil {
			return err
		}

	}

	err = netlink.RouteAdd(r)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			DEBUG("default ipv6 route already exists")
			return nil
		}
		if strings.Contains(err.Error(), "permission denied") {
			DEBUG("missing permission or ipv6 disabled when adding ipv6 route")
			return nil
		}
		return err
	}

	DEBUG(
		"ip ",
		"-6 ",
		"route ",
		"add ",
		network,
		" via ",
		gateway,
		" metric ",
		metric,
	)

	return
}

func delIPv4Route(network string, gateway string, metric string) (err error) {
	mInt, err := strconv.Atoi(metric)
	if err != nil {
		return err
	}

	r := new(netlink.Route)
	if network == "default" {
		_, r.Dst, _ = net.ParseCIDR("0.0.0.0/0")
	} else {
		_, r.Dst, err = net.ParseCIDR(network)
		if err != nil {
			return err
		}
	}

	r.Priority = mInt
	r.Gw = net.ParseIP(gateway).To4()

	DEBUG("DEL ROUTE: ", r)
	err = netlink.RouteDel(r)
	if err != nil {
		return err
	}

	return
}

func delIPv6Route(network string, gateway string, metric string) (err error) {
	mInt, err := strconv.Atoi(metric)
	if err != nil {
		return err
	}

	r := new(netlink.Route)
	if network == "default" {
		_, r.Dst, _ = net.ParseCIDR("::/0")
	} else {
		_, r.Dst, err = net.ParseCIDR(network)
		if err != nil {
			return err
		}
	}

	r.Priority = mInt
	r.Gw = net.ParseIP(gateway).To16()

	DEBUG("DEL IPv6 ROUTE: ", r)
	err = netlink.RouteDel(r)
	if err != nil {
		return err
	}

	return
}

func AdjustRoutersForTunneling() (err error) {
	defer RecoverAndLog()

	links, _ := netlink.LinkList()
	for _, v := range links {

		routes, _ := netlink.RouteList(v, netlink.FAMILY_V4)
		for i := range routes {
			r := routes[i]
			if r.Dst == nil && r.Priority < 2 {
				DEBUG("Adjusting Default Route: ", r)
				_ = netlink.RouteDel(&r)
				r.Priority = 100
				_ = netlink.RouteAdd(&r)
			}
		}
	}

	return
}
