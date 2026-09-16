//go:build windows

package client

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
)

type adapter struct {
	tunnel atomic.Pointer[*TUN]

	Name          string
	IPv4Address   string
	IPv6Address   string
	NetMask       string
	TxQueuelen    int32
	MTU           int32
	Gateway       string
	GatewayMetric string
}

func (t *adapter) Addr() (err error) {
	cmd := hiddenCommand(
		"netsh",
		"interface",
		"ipv4",
		"set",
		"address",
		`name="`+t.Name+`"`,
		"static",
		t.IPv4Address,
		t.NetMask,
		t.Gateway,
		"gwmetric="+t.GatewayMetric,
		"store=persistent",
	)

	DEBUG(
		"netsh",
		"interface",
		"ipv4",
		"set",
		"address",
		`name="`+t.Name+`"`,
		"static",
		t.IPv4Address,
		t.NetMask,
		t.Gateway,
		"gwmetric="+t.GatewayMetric,
		"store=persistent",
	)

	ob, err := cmd.Output()
	if err != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, err))
		return err
	}

	return nil
}

func (t *adapter) AddrV6() (err error) {
	ipv6Addr := t.IPv6Address
	if ipv6Addr == "" {
		return nil
	}

	if !strings.Contains(ipv6Addr, "/") {
		ipv6Addr = ipv6Addr + "/64"
	}

	cmd := hiddenCommand(
		"netsh",
		"interface",
		"ipv6",
		"add",
		"address",
		`interface="`+t.Name+`"`,
		"address="+ipv6Addr,
		"store=persistent",
	)

	DEBUG(
		"netsh",
		"interface",
		"ipv6",
		"add",
		"address",
		`interface="`+t.Name+`"`,
		"address="+ipv6Addr,
		"store=persistent",
	)

	ob, err := cmd.Output()
	if err != nil {
		if strings.Contains(string(ob), "already exists") || strings.Contains(err.Error(), "already exists") {
			DEBUG("IPv6 address already exists on interface: ", t.Name)
			return nil
		}
		ERROR(fmt.Sprintf("IPv6 address configuration failed: %s - out: %s", err, ob))
		return err
	}

	DEBUG("Added IPv6 address ", t.IPv6Address, " to interface ", t.Name)
	return nil
}

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
	if metric == "0" {
		metric = "1"
	}

	_ = delIPv4Route(network, gateway, metric)

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

	var cmd *exec.Cmd
	if network == "default" {
		cmd = hiddenCommand(
			"netsh",
			"interface",
			"ipv6",
			"add",
			"route",
			"::/0",
			`interface="`+ifName+`"`,
			"metric="+metric,
			"store=active",
		)
		DEBUG(
			"netsh",
			"interface",
			"ipv6",
			"add",
			"route",
			"::/0",
			`interface="`+ifName+`"`,
			"metric="+metric,
			"store=active",
		)
	} else {
		cmd = hiddenCommand(
			"netsh",
			"interface",
			"ipv6",
			"add",
			"route",
			network,
			`interface="`+ifName+`"`,
			"metric="+metric,
			"store=active",
		)
		DEBUG(
			"netsh",
			"interface",
			"ipv6",
			"add",
			"route",
			network,
			`interface="`+ifName+`"`,
			"metric="+metric,
			"store=active",
		)
	}

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

func (t *adapter) SetMTU() error {
	cmd := hiddenCommand(
		"netsh",
		"interface",
		"ipv4",
		"set",
		"subinterface",
		t.Name,
		"mtu="+strconv.FormatInt(int64(t.MTU), 10),
	)

	DEBUG(
		"netsh ",
		"interface ",
		"ipv4 ",
		"set ",
		"subinterface ",
		t.Name,
		"mtu="+strconv.FormatInt(int64(t.MTU), 10),
	)

	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
	}
	return nil
}

func (t *adapter) configureAdapter() (err error) {
	t.GatewayMetric = "2000"
	if err = t.Addr(); err != nil {
		return
	}

	if t.IPv6Address != "" {
		err = t.AddrV6()
		if err != nil {
			DEBUG("Unable to add IPv6 address, maybe IPv6 is turned off ?, err : ", err)
		}
	}

	err = t.SetMTU()
	if err != nil {
		return
	}

	closeAllOpenTCPConnections()
	return
}

func (t *adapter) applyTunnelRoutes(tun *TUN) (err error) {
	meta := tun.meta.Load()

	if meta.EnableDefaultRoute {
		t.GatewayMetric = "1"
		err = setIPv4RouteMetric("0.0.0.0/0", t.Name, "1")
		if err != nil {
			return
		}

		if t.IPv6Address != "" {
			iperr := addIPv6Route("default", t.Name, t.IPv6Address, "0")
			if iperr != nil {
				DEBUG("Unable to add IPv6 route, maybe IPv6 is turned off ?, err : ", iperr)
			}
		}
	}

	if sub := tun.ServerResponse.WireGuardSubnet; sub != "" {
		err = addIPv4Route(sub, meta.IFName, t.IPv4Address, "0")
		if err != nil {
			return err
		}
	}
	if sub6 := tun.ServerResponse.WireGuardSubnet6; sub6 != "" && t.IPv6Address != "" {
		iperr := addIPv6Route(sub6, t.Name, t.IPv6Address, "0")
		if iperr != nil {
			DEBUG("Unable to add IPv6 WireGuard subnet route, err : ", iperr)
		}
	}

	if meta.EnableWAN {
		if wan := tun.ServerResponse.WANCIDR; wan != "" {
			err = addIPv4Route(wan, meta.IFName, t.IPv4Address, "0")
			if err != nil {
				return err
			}
		}
	}

	for _, n := range tun.ServerResponse.Networks {
		if n.Nat != "" {
			err = addIPv4Route(n.Nat, meta.IFName, t.IPv4Address, "0")
			if err != nil {
				return err
			}
		}
	}

	for _, v := range tun.ServerResponse.Routes {
		err = addIPv4Route(v.Address, meta.IFName, t.IPv4Address, v.Metric)
		if err != nil {
			return err
		}
	}

	return
}

func (t *adapter) Connect(tun *TUN) (err error) {
	err = t.configureAdapter()
	if err != nil {
		return
	}

	err = t.applyTunnelRoutes(tun)
	if err != nil {
		return
	}

	closeAllOpenTCPConnections()
	return
}

func (t *adapter) Delete() error { return nil }

func (t *adapter) Disconnect(tun *TUN) (err error) {
	defer RecoverAndLog()

	if tun.wgDevice != nil {
		tun.wgDevice.Close()
	}

	meta := tun.meta.Load()
	if IsDefaultConnection(meta.IFName) || meta.EnableDefaultRoute {
		err = delIPv4Route("default", t.IPv4Address, "0")

		if t.IPv6Address != "" {
			iperr := delIPv6Route("default", t.IPv6Address, "0")
			if iperr != nil {
				DEBUG("Unable to delete IPv6 default route, err : ", iperr)
			}
		}
	}

	if sub := tun.ServerResponse.WireGuardSubnet; sub != "" {
		if delErr := delIPv4Route(sub, t.IPv4Address, "0"); delErr != nil {
			DEBUG("Unable to delete WireGuard subnet route, err : ", delErr)
		}
	}
	if sub6 := tun.ServerResponse.WireGuardSubnet6; sub6 != "" && t.IPv6Address != "" {
		if delErr := delIPv6Route(sub6, t.IPv6Address, "0"); delErr != nil {
			DEBUG("Unable to delete IPv6 WireGuard subnet route, err : ", delErr)
		}
	}

	for _, n := range tun.ServerResponse.Networks {
		if n.Nat != "" {
			err = delIPv4Route(n.Nat, t.IPv4Address, "0")
			if err != nil {
				return err
			}
		}
	}
	for _, r := range tun.ServerResponse.Routes {
		err = delIPv4Route(r.Address, t.IPv4Address, r.Metric)
		if err != nil {
			return err
		}
	}
	return
}
