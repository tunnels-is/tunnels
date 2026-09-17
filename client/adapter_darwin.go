//go:build darwin

package client

import (
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
)

type adapter struct {
	tunnel atomic.Pointer[*TUN]

	Name        string
	IPv4Address string
	IPv6Address string
	NetMask     string
	TxQueuelen  int32
	MTU         int32
	Gateway     string
}

func (t *adapter) Up() (err error) {
	DEBUG("ifconfig", t.Name, t.IPv4Address, t.Gateway, "up")

	out, err := exec.Command("ifconfig", t.Name, t.IPv4Address, t.Gateway, "up").CombinedOutput()
	if err != nil {
		ERROR("unable to bring up tunnel adapter: ", string(out), " err: ", err)
		return err
	}

	return
}

func (t *adapter) SetMTU() (err error) {
	DEBUG("ifconfig", t.Name, "mtu", strconv.FormatInt(int64(t.MTU), 10))
	out, err := exec.Command("ifconfig", t.Name, "mtu", strconv.FormatInt(int64(t.MTU), 10)).CombinedOutput()
	if err != nil {
		ERROR("Unable to change mtu out: ", string(out), " err: ", err)
		return err
	}
	return
}

func (t *adapter) AddrV6() (err error) {
	if t.IPv6Address == "" {
		return nil
	}

	ipv6Addr := t.IPv6Address
	if !strings.Contains(ipv6Addr, "/") {
		ipv6Addr = ipv6Addr + "/64"
	}

	DEBUG("ifconfig", t.Name, "inet6", ipv6Addr)
	out, err := exec.Command("ifconfig", t.Name, "inet6", ipv6Addr).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "File exists") || strings.Contains(err.Error(), "exists") {
			DEBUG("IPv6 address already exists on interface: ", t.Name)
			return nil
		}
		ERROR("IPv6 address configuration failed: ", err, " out: ", string(out))
		return err
	}

	DEBUG("Added IPv6 address ", t.IPv6Address, " to interface ", t.Name)
	return nil
}

func (t *adapter) configureAdapter() (err error) {
	err = t.Up()
	if err != nil {
		return
	}
	err = t.SetMTU()
	if err != nil {
		return
	}

	if t.IPv6Address != "" {
		err = t.AddrV6()
		if err != nil {
			DEBUG("Unable to add IPv6 address, maybe IPv6 is turned off ?, err : ", err)
		}
	}

	return nil
}

func (t *adapter) applyTunnelRoutes(tun *TUN) (err error) {
	meta := tun.meta.Load()
	if meta.EnableDefaultRoute {
		_ = delDefaultIPv4Route()
		err = addDefaultIPv4Route(t.IPv4Address)
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
		err = addIPv4Route(sub, "", t.IPv4Address, "0")
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
			err = addIPv4Route(wan, "", t.IPv4Address, "0")
			if err != nil {
				return err
			}
		}
	}

	for _, n := range tun.ServerResponse.Networks {
		if n.Nat != "" {
			err = addIPv4Route(n.Nat, "", t.IPv4Address, "0")
			if err != nil {
				return err
			}
		}
	}

	for _, v := range tun.ServerResponse.Routes {
		err = addIPv4Route(v.Address, "", t.IPv4Address, v.Metric)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *adapter) Connect(tun *TUN) (err error) {
	err = t.configureAdapter()
	if err != nil {
		return
	}

	return t.applyTunnelRoutes(tun)
}

func (t *adapter) Delete() error { return nil }

func (t *adapter) Disconnect(tun *TUN) (err error) {
	defer RecoverAndLog()
	if tun.wgDevice != nil {
		tun.wgDevice.Close()
	}

	meta := tun.meta.Load()
	if isDefaultTunnelName(meta.IFName) || meta.EnableDefaultRoute {
		err = delIPv4Route("default", t.IPv4Address, "0")

		gateway := STATE.Load().DefaultGateway.Load()
		if gateway != nil {
			_ = addDefaultIPv4Route(gateway.To4().String())
		} else {
			ERROR("default gateway not found in STATE")
		}

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

	return nil
}
