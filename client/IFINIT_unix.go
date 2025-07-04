//go:build freebsd || linux || openbsd

package client

import (
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/vishvananda/netlink"
)

type TInterface struct {
	tunnel atomic.Pointer[*TUN]

	Name        string
	IPv4Address string
	IPv6Address string
	NetMask     string
	TxQueuelen  int32
	MTU         int32
	Gateway     string

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

func CreateNewTunnelInterface(
	meta *TunnelMETA,
) (IF *TInterface, err error) {
	defer RecoverAndLogToFile()

	// Some kernels seem to not be compiled with
	// /dev/net/tun ... we need to create it.
	// Even after creating the user might need to reboot.
	_, err = os.Stat("/dev/net/tun")
	if err != nil {
		createDevNetTun()
	}

	IF = &TInterface{
		Name:        meta.IFName,
		IPv4Address: meta.IPv4Address,
		IPv6Address: meta.IPv6Address,
		NetMask:     meta.NetMask,
		Gateway:     meta.IPv4Address,
		TxQueuelen:  meta.TxQueueLen,
		MTU:         meta.MTU,
	}

	err = IF.Create()
	if err != nil {
		return IF, err
	}

	if IF.RWC == nil {
		return IF, errors.New("unable to create tunnel read write closer")
	}

	return IF, err
}

type syscallCreateIF struct {
	Name  [0x10]byte
	Flags uint16
	pad   [0x28 - 0x10 - 2]byte
}

func (t *TInterface) Create() (err error) {
	if t.TunnelFile == "" {
		t.TunnelFile = "/dev/net/tun"
	}

	INFO("about to open device: ", t.TunnelFile)
	fd, err := syscall.Open(t.TunnelFile, os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		ERROR("erro opening device: ", t.TunnelFile, " || err: ", err)
		return err
	}

	t.FD = uintptr(fd)

	var flags uint16 = 0x1000
	flags |= 0x0001
	if t.Multiqueue {
		flags |= 0x0100 // MULTIQUEUE FLAG
	}

	var req syscallCreateIF
	req.Flags = flags
	copy(req.Name[:], []byte(t.Name))

	if err = tunnelCtl(t.FD, syscall.TUNSETIFF, uintptr(unsafe.Pointer(&req))); err != nil {
		return err
	}

	if t.User != 0 {
		if err = tunnelCtl(t.FD, syscall.TUNSETOWNER, uintptr(t.User)); err != nil {
			return err
		}
	}

	if t.Group != 0 {
		if err = tunnelCtl(t.FD, syscall.TUNSETGROUP, uintptr(t.Group)); err != nil {
			return err
		}
	}

	t.RWC = os.NewFile(t.FD, "tun_"+t.Name)
	// if t.Persistent {
	// 	if err = tunnelCtl(t.FD, syscall.TUNSETPERSIST, uintptr(1)); err != nil {
	// 		return err
	// 	}
	// }

	return
}

type syscallSetFlags struct {
	Name  [16]byte
	Flags int16
}

type syscallAddAddrV4 struct {
	Name [16]byte
	syscall.RawSockaddrInet4
}

func (t *TInterface) Addr() (err error) {
	var ifr syscallAddAddrV4
	ifr.Port = 0
	ifr.Family = syscall.AF_INET

	copy(ifr.Name[:], []byte(t.Name))
	copy(ifr.Addr[:], net.ParseIP(t.IPv4Address).To4())

	if err = socketCtl(
		syscall.SIOCSIFADDR,
		uintptr(unsafe.Pointer(&ifr)),
	); err != nil {
		return
	}

	return
}

func (t *TInterface) GetLink() (netlink.Link, error) {
	link, err := netlink.LinkByName(t.Name)
	if err != nil {
		return nil, err
	}
	return link, nil
}

func (t *TInterface) AddrV6() (err error) {
	link, err := netlink.LinkByName(t.Name)
	if err != nil {
		return err
	}

	ipv6, ipv6Net, err := net.ParseCIDR(t.IPv6Address + "/64")
	if err != nil {
		ipv6 = net.ParseIP(t.IPv6Address)
		if ipv6 == nil {
			return errors.New("invalid IPv6 address")
		}
		_, ipv6Net, _ = net.ParseCIDR(t.IPv6Address + "/64")
	}

	addr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ipv6,
			Mask: ipv6Net.Mask,
		},
	}

	err = netlink.AddrAdd(link, addr)
	if err != nil && !strings.Contains(err.Error(), "exists") && !strings.Contains(err.Error(), "permission denied") {
		return err
	}

	DEBUG("Added IPv6 address ", t.IPv6Address, " to interface ", t.Name)

	return nil
}

func (t *TInterface) Up() (err error) {
	var ifr syscallSetFlags

	copy(ifr.Name[:], []byte(t.Name))
	ifr.Flags |= 0x1

	if err = socketCtl(
		syscall.SIOCSIFFLAGS,
		uintptr(unsafe.Pointer(&ifr)),
	); err != nil {
		return
	}

	return
}

func (t *TInterface) Down() (err error) {
	var ifr syscallSetFlags

	copy(ifr.Name[:], []byte(t.Name))
	ifr.Flags |= 0x0

	if err = socketCtl(
		syscall.SIOCSIFFLAGS,
		uintptr(unsafe.Pointer(&ifr)),
	); err != nil {
		return
	}

	return
}

type syscallChangeMTU struct {
	Name [16]byte
	MTU  int32
}

func (t *TInterface) SetMTU() (err error) {
	var ifr syscallChangeMTU
	copy(ifr.Name[:], []byte(t.Name))
	ifr.MTU = t.MTU

	if err = socketCtl(
		syscall.SIOCSIFMTU,
		uintptr(unsafe.Pointer(&ifr)),
	); err != nil {
		return
	}

	return
}

type syscallChangeTXQueueLen struct {
	Name       [16]byte
	TxQueueLen int32
}

func (t *TInterface) SetTXQueueLen() (err error) {
	var ifr syscallChangeTXQueueLen
	copy(ifr.Name[:], []byte(t.Name))
	ifr.TxQueueLen = t.TxQueuelen

	if err = socketCtl(
		syscall.SIOCSIFTXQLEN,
		uintptr(unsafe.Pointer(&ifr)),
	); err != nil {
		return
	}

	return
}

func (t *TInterface) Netmask() (err error) {
	var ifr syscallAddAddrV4
	ifr.Port = 0
	ifr.Family = syscall.AF_INET

	copy(ifr.Name[:], []byte(t.Name))
	copy(ifr.Addr[:], net.ParseIP(t.NetMask).To4())

	if err = socketCtl(
		syscall.SIOCSIFNETMASK,
		uintptr(unsafe.Pointer(&ifr)),
	); err != nil {
		return
	}

	return
}

func (t *TInterface) Delete() (err error) {
	var ifr syscallSetFlags
	DOR := 1 << 17

	copy(ifr.Name[:], []byte(t.Name))
	ifr.Flags |= 0x0
	ifr.Flags = int16(DOR)

	if err = socketCtl(
		syscall.SIOCSIFFLAGS,
		uintptr(unsafe.Pointer(&ifr)),
	); err != nil {
		return
	}

	_ = exec.Command("ip", "link", "delete", t.Name).Run()

	return
}

func (t *TInterface) Connect(tun *TUN) (err error) {
	err = t.Addr()
	if err != nil {
		return
	}

	if t.IPv6Address != "" {
		err = t.AddrV6()
		if err != nil {
			DEBUG("Unable to add IPv6 address, maybe IPv6 is turned off ?, err : ", err)
			return
		}
	}

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

	meta := tun.meta.Load()
	if IsDefaultConnection(meta.IFName) || meta.EnableDefaultRoute {
		err = IP_AddRoute("default", "", t.IPv4Address, "0")
		if err != nil {
			return err
		}

		// Add default IPv6 route if IPv6 address is configured
		if t.IPv6Address != "" {
			iperr := IP_AddRouteV6("default", t.Name, t.IPv6Address, "0")
			if iperr != nil {
				DEBUG("Unable to add IPv6 route, maybe IPv6 is turned off ?, err : ", err)
			}
		}
	}

	if tun.ServerResponse.LAN != nil && tun.ServerResponse.LAN.Nat != "" {
		err = IP_AddRoute(tun.ServerResponse.LAN.Nat, "", t.IPv4Address, "0")
		if err != nil {
			return err
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

	return
}

func (t *TInterface) Disconnect(tun *TUN) (err error) {
	defer RecoverAndLogToFile()
	if tun.connection != nil {
		_ = tun.connection.Close()
	}

	err = t.Close()
	if err != nil {
		ERROR("unable to close the interface", err)
	}

	_ = t.Delete()

	return
}

func createDevNetTun() {
	out, err := exec.Command("mkdir", "-p", "/dev/net").CombinedOutput()
	if err != nil {
		ERROR("TUN CREATE:", err, string(out))
		return
	}
	out, err = exec.Command("mknod", "/dev/net/tun", "c", "10", "200").CombinedOutput()
	if err != nil {
		ERROR("TUN CREATE:", err, string(out))
		return
	}
	out, err = exec.Command("chmod", "600", "/dev/net/tun").CombinedOutput()
	if err != nil {
		ERROR("TUN CREATE:", err, string(out))
		return
	}
}

func tunnelCtl(fd uintptr, request uintptr, argp uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(request), argp)
	if errno != 0 {
		return os.NewSyscallError("ioctl", errno)
	}
	return nil
}

func socketCtl(request uintptr, argp uintptr) error {
	fd, err := syscall.Socket(
		syscall.AF_INET,
		syscall.SOCK_DGRAM,
		syscall.IPPROTO_IP,
	)
	defer syscall.Close(fd)
	if err != nil {
		return err
	}

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(request), argp)
	if errno != 0 {
		return os.NewSyscallError("ioctl", errno)
	}
	return nil
}

func IP_AddRoute(
	network string,
	_ string,
	gateway string,
	metric string,
) (err error) {
	_ = IP_DelRoute(network, gateway, metric)

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

	err = netlink.RouteAdd(r)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
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
		" metric ",
		metric,
	)
	return
}

func IP_AddRouteV6(
	network string,
	ifName string,
	gateway string,
	metric string,
) (err error) {
	_ = IP_DelRouteV6(network, gateway, metric)

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
		// r.Gw = net.ParseIP(gateway).To16()
	}

	err = netlink.RouteAdd(r)
	if err != nil {
		if strings.Contains(err.Error(), "exists") {
			return nil
		}
		if strings.Contains(err.Error(), "permission denied") {
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

func IP_DelRouteNoGW(network string, metric int) (err error) {
	r := new(netlink.Route)
	r.Dst = new(net.IPNet)
	r.Dst.IP = net.ParseIP(network).To4()
	r.Priority = metric
	DEBUG("DEL ROUTE: ", r)
	err = netlink.RouteDel(r)
	if err != nil {
		return err
	}

	return
}

func IP_DelRoute(network string, gateway string, metric string) (err error) {
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

func IP_DelRouteV6(network string, gateway string, metric string) (err error) {
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

func RestoreDNSOnClose() {
	// not implemented for unix
}

func RestoreSaneDNSDefaults() {
	// not implemented for unix
}

func IPv6Enabled() bool {
	s := STATE.Load()
	defIntName := s.DefaultInterfaceName.Load()
	if defIntName == nil {
		return false
	}

	out, err := exec.Command("bash", "-c", "cat /proc/sys/net/ipv6/conf/"+*defIntName+"/disable_ipv6").CombinedOutput()
	if err != nil {
		ERROR("Error getting ipv6 settings for interface: ", s.DefaultInterfaceName.Load(), " || msg: ", err, " || output: ", string(out))
		return true
	}

	outString := string(out)
	outString = strings.TrimSpace(outString)

	return outString == "0"
}

func AdjustRoutersForTunneling() (err error) {
	defer RecoverAndLogToFile()

	links, _ := netlink.LinkList()
	for _, v := range links {
		routes, _ := netlink.RouteList(v, 4)
		for _, r := range routes {
			if r.Dst == nil && r.Priority < 2 {
				DEBUG("Adjusting Default Route: ", r)
				_ = netlink.RouteDel(&r)
				r.Priority = 100
				_ = netlink.RouteAdd(&r)
				return
			}
		}
	}

	return
}
