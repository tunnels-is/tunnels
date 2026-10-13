//go:build windows

package client

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func setIPv4RouteMetric(network string, ifname string, metric string) (err error) {
	if err = validateRouteArgs(network, "", metric); err != nil {
		return err
	}
	if metric == "0" {
		metric = "1"
	}

	cmd := hiddenCommand(
		"netsh",
		"interface",
		"ipv4",
		"set",
		"route",
		network,
		ifname,
		"metric="+metric,
		"store=active",
	)
	DEBUG(
		"netsh",
		"interface",
		"ipv4",
		"set",
		"route",
		network,
		ifname,
		"metric="+metric,
		"store=active",
	)

	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
	}

	return
}

func addIPv4Route(
	network string,
	ifName string,
	gateway string,
	metric string,
) (err error) {
	if err = validateRouteArgs(network, gateway, metric); err != nil {
		return err
	}
	if ipv4HostOnLink(network) {
		DEBUG("skip route add, dest on-link: ", network)
		return nil
	}
	if metric == "0" {
		metric = "1"
	}

	if ifName != "" {
		if replaceConflictingIPv4Route(network, ifName, gateway, metric) {
			return nil
		}
	} else {
		_ = delIPv4Route(network, gateway, metric)
	}

	cmd := hiddenCommand(
		"netsh",
		"interface",
		"ipv4",
		"add",
		"route",
		network,
		ifName,
		gateway,
		metric,
		"store=active",
	)

	DEBUG(
		"netsh",
		"interface",
		"ipv4",
		"add",
		"route",
		network,
		ifName,
		gateway,
		metric,
		"store=active",
	)

	ob, cerr := cmd.Output()

	if cerr != nil {
		return fmt.Errorf("%s - out: %s", cerr, ob)
	}

	return
}

func replaceConflictingIPv4Route(network, ifName, gateway, metric string) (alreadyPresent bool) {
	host := ipv4HostFromNetwork(network)
	if host != nil && ipv4HostRoutePresent(host, ifName, net.ParseIP(gateway)) {
		return true
	}
	_ = delIPv4Route(network, gateway, metric)
	return false
}

func ipv4HostRoutePresent(host net.IP, ifName string, gw net.IP) bool {
	gw4 := gw.To4()
	if host == nil || gw4 == nil {
		return false
	}
	iface, err := net.InterfaceByName(ifName)
	if err != nil {
		return false
	}
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_INET, &table); err != nil {
		return false
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	var matches int
	present := false
	for _, r := range table.Rows() {
		if r.DestinationPrefix.PrefixLength != 32 {
			continue
		}
		dst := (*windows.RawSockaddrInet4)(unsafe.Pointer(&r.DestinationPrefix.Prefix))
		if !net.IP(dst.Addr[:]).Equal(host) {
			continue
		}
		matches++
		nh := (*windows.RawSockaddrInet4)(unsafe.Pointer(&r.NextHop))
		if r.InterfaceIndex == uint32(iface.Index) && net.IP(nh.Addr[:]).Equal(gw4) {
			present = true
		}
	}
	return present && matches == 1
}

func delIPv4Route(network string, _ string, _ string) (err error) {
	cmd := hiddenCommand("route", "DELETE", network)

	DEBUG("route", "DELETE", network)

	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
	}

	return
}

func addIPv6Route(
	network string,
	ifName string,
	gateway string,
	metric string,
) (err error) {
	if err = validateRouteArgs(network, "", metric); err != nil {
		return err
	}
	if metric == "0" {
		metric = "1"
	}

	_ = delIPv6Route(network, gateway, metric)

	dest := network
	if network == "default" {
		dest = "::/0"
	}
	cmd := netshAddIPv6RouteCmd(dest, ifName, metric)
	DEBUG(
		"netsh",
		"interface",
		"ipv6",
		"add",
		"route",
		dest,
		`interface="`+ifName+`"`,
		"metric="+metric,
		"store=active",
	)

	ob, cerr := cmd.Output()

	if cerr != nil {
		if strings.Contains(string(ob), "already exists") || strings.Contains(cerr.Error(), "already exists") {
			DEBUG("IPv6 route already exists: ", network)
			return nil
		}
		return fmt.Errorf("IPv6 route add failed: %s - out: %s", cerr, ob)
	}

	return
}

func netshAddIPv6RouteCmd(dest, ifName, metric string) *exec.Cmd {
	return hiddenCommand(
		"netsh",
		"interface",
		"ipv6",
		"add",
		"route",
		dest,
		`interface="`+ifName+`"`,
		"metric="+metric,
		"store=active",
	)
}

func delIPv6Route(network string, _ string, _ string) (err error) {
	var cmd *exec.Cmd
	if network == "default" {
		cmd = hiddenCommand(
			"netsh",
			"interface",
			"ipv6",
			"delete",
			"route",
			"::/0",
		)
		DEBUG(
			"netsh",
			"interface",
			"ipv6",
			"delete",
			"route",
			"::/0",
		)
	} else {
		cmd = hiddenCommand(
			"netsh",
			"interface",
			"ipv6",
			"delete",
			"route",
			network,
		)
		DEBUG(
			"netsh",
			"interface",
			"ipv6",
			"delete",
			"route",
			network,
		)
	}

	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("IPv6 route delete failed: %s - out: %s", cerr, ob))
		return cerr
	}

	return
}
