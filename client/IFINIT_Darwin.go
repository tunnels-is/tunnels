//go:build darwin

package client

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type TInterface struct {
	tunnel atomic.Pointer[*TUN]

	Name        string
	SystemName  string
	IPv4Address string
	IPv6Address string
	NetMask     string
	TxQueuelen  int32
	MTU         int32
	Persistent  bool
	Gateway     string

	// linux ?
	Multiqueue bool
	User       uint
	Group      uint
	TunnelFile string
	RWC        io.ReadWriteCloser
	FD         uintptr
}

func (t *TInterface) Close() error {
	if t.RWC != nil {
		return t.RWC.Close()
	}
	return nil
}

const (
	appleUTUNCtl     = "com.apple.net.utun_control"
	appleCTLIOCGINFO = (0x40000000 | 0x80000000) | ((100 & 0x1fff) << 16) | uint32(byte('N'))<<8 | 3
	appleTUNSIFMODE  = (0x80000000) | ((4 & 0x1fff) << 16) | uint32(byte('t'))<<8 | 94
)

type sockaddrCtl struct {
	scLen      uint8
	scFamily   uint8
	ssSysaddr  uint16
	scID       uint32
	scUnit     uint32
	scReserved [5]uint32
}

var sockaddrCtlSize uintptr = 32

func (t *TInterface) Create() (err error) {
	ifIndex := -1

	var fd int
	if fd, err = syscall.Socket(
		syscall.AF_SYSTEM,
		syscall.SOCK_DGRAM,
		2,
	); err != nil {
		return fmt.Errorf("error in syscall.Socket: %v", err)
	}

	ctlInfo := &struct {
		ctlID   uint32
		ctlName [96]byte
	}{}
	copy(ctlInfo.ctlName[:], []byte(appleUTUNCtl))

	if _, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(appleCTLIOCGINFO),
		uintptr(unsafe.Pointer(ctlInfo)),
	); errno != 0 {
		err = errno
		return fmt.Errorf("error in syscall.Syscall(syscall.SYS_IOCTL, ...): %v", err)
	}

	addrP := unsafe.Pointer(&sockaddrCtl{
		scLen:     uint8(sockaddrCtlSize),
		scFamily:  syscall.AF_SYSTEM,
		ssSysaddr: 2,
		scID:      ctlInfo.ctlID,
		scUnit:    uint32(ifIndex) + 1,
	})

	if _, _, errno := syscall.RawSyscall(
		syscall.SYS_CONNECT,
		uintptr(fd),
		uintptr(addrP),
		uintptr(sockaddrCtlSize),
	); errno != 0 {
		err = errno
		return fmt.Errorf("error in syscall.RawSyscall(syscall.SYS_CONNECT, ...): %v", err)
	}

	var ifName struct {
		name [16]byte
	}
	ifNameSize := uintptr(16)

	_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, uintptr(fd),
		2,
		2,
		uintptr(unsafe.Pointer(&ifName)),
		uintptr(unsafe.Pointer(&ifNameSize)), 0)
	if errno != 0 {
		err = errno
		return fmt.Errorf("error in syscall.Syscall6(syscall.SYS_GETSOCKOPT, ...): %v", err)
	}

	err = syscall.SetNonblock(fd, true)
	if err != nil {
		return fmt.Errorf("setting non-blocking error")
	}

	t.SystemName = string(bytes.Replace(ifName.name[:], []byte{0}, []byte{}, -1))
	t.RWC = os.NewFile(uintptr(fd), t.SystemName)
	return nil
}

func (t *TInterface) sysName() string {
	if t.SystemName != "" {
		return t.SystemName
	}
	return t.Name
}

func (t *TInterface) Up() (err error) {
	name := t.sysName()
	DEBUG("ifconfig", name, t.IPv4Address, t.Gateway, "up")

	out, err := exec.Command("ifconfig", name, t.IPv4Address, t.Gateway, "up").CombinedOutput()
	if err != nil {
		ERROR("unable to bring up tunnel adapter: ", string(out), " err: ", err)
		return err
	}

	return
}

func (t *TInterface) Down() (err error) {
	name := t.sysName()
	DEBUG("ifconfig", name, "down")

	out, err := exec.Command("ifconfig", name, "down").CombinedOutput()
	if err != nil {
		ERROR("unable to bring down tunnel adapter: ", string(out), " err: ", err)
		return err
	}

	return
}

func (t *TInterface) SetMTU() (err error) {
	name := t.sysName()
	DEBUG("ifconfig", name, "mtu", strconv.FormatInt(int64(t.MTU), 10))
	out, err := exec.Command("ifconfig", name, "mtu", strconv.FormatInt(int64(t.MTU), 10)).CombinedOutput()
	if err != nil {
		ERROR("Unable to change mtu out: ", string(out), " err: ", err)
		return err
	}
	return
}

func (t *TInterface) Netmask() (err error) {
	return nil
}

func (t *TInterface) SetTXQueueLen() (err error) {

	return nil
}

func (t *TInterface) AddrV6() (err error) {

	if t.IPv6Address == "" {
		return nil
	}

	name := t.sysName()
	ipv6Addr := t.IPv6Address
	if !strings.Contains(ipv6Addr, "/") {
		ipv6Addr = ipv6Addr + "/64"
	}

	DEBUG("ifconfig", name, "inet6", ipv6Addr)
	out, err := exec.Command("ifconfig", name, "inet6", ipv6Addr).CombinedOutput()
	if err != nil {

		if strings.Contains(string(out), "File exists") || strings.Contains(err.Error(), "exists") {
			DEBUG("IPv6 address already exists on interface: ", name)
			return nil
		}
		ERROR("IPv6 address configuration failed: ", err, " out: ", string(out))
		return err
	}

	DEBUG("Added IPv6 address ", t.IPv6Address, " to interface ", name)
	return nil
}

func (t *TInterface) PrepareForSwitch() {

}

func (t *TInterface) Connect(tun *TUN) (err error) {
	err = t.Up()
	if err != nil {
		return
	}
	err = t.SetMTU()
	if err != nil {
		return
	}
	err = t.SetTXQueueLen()
	if err != nil {
		return
	}

	if t.IPv6Address != "" {
		err = t.AddrV6()
		if err != nil {
			DEBUG("Unable to add IPv6 address, maybe IPv6 is turned off ?, err : ", err)
		}
	}

	meta := tun.meta.Load()
	if meta.EnableDefaultRoute {
		_ = IP_DelDefaultRoute()
		err = IP_AddDefaultRoute(t.IPv4Address)
		if err != nil {
			return
		}

		if t.IPv6Address != "" {
			iperr := IP_AddRouteV6("default", t.SystemName, t.IPv6Address, "0")
			if iperr != nil {
				DEBUG("Unable to add IPv6 route, maybe IPv6 is turned off ?, err : ", iperr)
			}
		}
	}

	if sub := tun.ServerResponse.WireGuardSubnet; sub != "" {
		err = IP_AddRoute(sub, "", t.IPv4Address, "0")
		if err != nil {
			return err
		}
	}
	if sub6 := tun.ServerResponse.WireGuardSubnet6; sub6 != "" && t.IPv6Address != "" {
		iperr := IP_AddRouteV6(sub6, t.SystemName, t.IPv6Address, "0")
		if iperr != nil {
			DEBUG("Unable to add IPv6 WireGuard subnet route, err : ", iperr)
		}
	}

	for _, n := range tun.ServerResponse.Networks {
		if n.Nat != "" {
			err = IP_AddRoute(n.Nat, "", t.IPv4Address, "0")
			if err != nil {
				return err
			}
		}
	}

	for _, v := range tun.ServerResponse.Routes {
		err = IP_AddRoute(v.Address, "", t.IPv4Address, v.Metric)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *TInterface) Disconnect(tun *TUN) (err error) {
	defer RecoverAndLog()
	if tun.wgDevice != nil {
		tun.wgDevice.Close()
	}

	err = t.Close()
	if err != nil {
		ERROR("unable to close the interface", err)
	}

	meta := tun.meta.Load()
	if IsDefaultConnection(meta.IFName) || meta.EnableDefaultRoute {
		err = IP_DelRoute("default", t.IPv4Address, "0")

		gateway := STATE.Load().DefaultGateway.Load()
		if gateway != nil {
			_ = IP_AddDefaultRoute(gateway.To4().String())
		} else {
			ERROR("default gateway not found in STATE")
		}

		if t.IPv6Address != "" {
			iperr := IP_DelRouteV6("default", t.IPv6Address, "0")
			if iperr != nil {
				DEBUG("Unable to delete IPv6 default route, err : ", iperr)
			}
		}
	}

	if sub := tun.ServerResponse.WireGuardSubnet; sub != "" {
		if delErr := IP_DelRoute(sub, t.IPv4Address, "0"); delErr != nil {
			DEBUG("Unable to delete WireGuard subnet route, err : ", delErr)
		}
	}
	if sub6 := tun.ServerResponse.WireGuardSubnet6; sub6 != "" && t.IPv6Address != "" {
		if delErr := IP_DelRouteV6(sub6, t.IPv6Address, "0"); delErr != nil {
			DEBUG("Unable to delete IPv6 WireGuard subnet route, err : ", delErr)
		}
	}

	for _, n := range tun.ServerResponse.Networks {
		if n.Nat != "" {
			err = IP_DelRoute(n.Nat, t.IPv4Address, "0")
			if err != nil {
				return err
			}
		}
	}

	for _, r := range tun.ServerResponse.Routes {
		err = IP_DelRoute(r.Address, t.IPv4Address, r.Metric)
		if err != nil {
			return err
		}
	}

	return nil
}

func IP_AddDefaultRoute(gateway string) (err error) {
	DEBUG("route", "add", "default", gateway)

	out, err := exec.Command("route", "add", "default", gateway).CombinedOutput()
	if err != nil {
		ERROR("Unable to add route: ", string(out), " err: ", err)
		return err
	}
	return
}

func IP_DelDefaultRoute() (err error) {
	DEBUG("route", "delete", "default")

	out, err := exec.Command("route", "delete", "default").CombinedOutput()
	if err != nil {
		ERROR("Unable to delete route: ", string(out), " err: ", err)
		return err
	}
	return
}

func IP_AddRoute(
	network string,
	_ string,
	gateway string,
	metric string,
) (err error) {
	_ = IP_DelRoute(network, "", metric)

	DEBUG("route", "-n", "add", "-net", network, gateway)

	out, err := exec.Command("route", "-n", "add", "-net", network, gateway).CombinedOutput()
	if err != nil {
		ERROR("Unable to add route: ", string(out), " err: ", err)
		return err
	}

	return
}

func IP_DelRoute(network string, gateway string, metric string) (err error) {
	DEBUG("route", "-n", "delete", "-net", network)

	out, err := exec.Command("route", "-n", "delete", "-net", network).CombinedOutput()
	if err != nil {
		ERROR("Unable to delete route: ", string(out), " err: ", err)
		return err
	}

	return
}

func IP_AddRouteV6(
	network string,
	ifName string,
	gateway string,
	metric string,
) (err error) {
	_ = IP_DelRouteV6(network, gateway, metric)

	var cmd *exec.Cmd
	if network == "default" {

		DEBUG("route", "-n", "add", "-inet6", "default", "-interface", ifName)
		cmd = exec.Command("route", "-n", "add", "-inet6", "default", "-interface", ifName)
	} else {

		DEBUG("route", "-n", "add", "-inet6", "-net", network, "-interface", ifName)
		cmd = exec.Command("route", "-n", "add", "-inet6", "-net", network, "-interface", ifName)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {

		if strings.Contains(string(out), "File exists") || strings.Contains(err.Error(), "exists") {
			DEBUG("IPv6 route already exists: ", network)
			return nil
		}
		ERROR("Unable to add IPv6 route: ", err, " out: ", string(out))
		return err
	}

	return
}

func IP_DelRouteV6(network string, _ string, _ string) (err error) {
	var cmd *exec.Cmd
	if network == "default" {
		DEBUG("route", "-n", "delete", "-inet6", "default")
		cmd = exec.Command("route", "-n", "delete", "-inet6", "default")
	} else {
		DEBUG("route", "-n", "delete", "-inet6", "-net", network)
		cmd = exec.Command("route", "-n", "delete", "-inet6", "-net", network)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {

		if strings.Contains(string(out), "not in table") || strings.Contains(string(out), "No such process") {
			DEBUG("IPv6 route doesn't exist (already deleted): ", network)
			return nil
		}
		ERROR("Unable to delete IPv6 route: ", err, " out: ", string(out))
		return err
	}

	return
}

func RestoreDNSOnClose() {

}

func RestoreSaneDNSDefaults() {

}

func GetDNSServers(id string) error {
	DEFAULT_DNS_SERVERS = nil
	return nil
}

func AdjustRoutersForTunneling() (err error) {

	return nil
}
