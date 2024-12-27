//go:build freebsd || linux || openbsd

package core

import (
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/vishvananda/netlink"
)

type TunnelInterface struct {
	tunnel        atomic.Pointer[*Tunnel]
	shouldRestart bool

	Name        string
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

func (t *TunnelInterface) Close() error {
	if t.RWC != nil {
		return t.RWC.Close()
	}
	return nil
}

func CreateNewTunnelInterface(
	VC *Tunnel,
) (IF *TunnelInterface, err error) {
	defer RecoverAndLogToFile()

	// Some kernels seem to not be compiled with
	// /dev/net/tun ... we need to create it.
	// Even after creating the user might need to reboot.
	_, err = os.Stat("/dev/net/tun")
	if err != nil {
		createDevNetTun()
	}

	IF = &TunnelInterface{
		Name:          VC.Meta.IFName,
		IPv4Address:   VC.Meta.IPv4Address,
		NetMask:       VC.Meta.NetMask,
		TxQueuelen:    VC.Meta.TxQueueLen,
		MTU:           VC.Meta.MTU,
		Persistent:    VC.Meta.Persistent,
		shouldRestart: true,
		// IPv6Address: "fe80::1",
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

func (t *TunnelInterface) Create() (err error) {
	if t.TunnelFile == "" {
		t.TunnelFile = "/dev/net/tun"
	}

	INFO("about to open device: ", t.TunnelFile)
	fd, err := syscall.Open(t.TunnelFile, os.O_RDWR|syscall.O_NONBLOCK, 0)
	// fd, err := syscall.Open(t.TunnelFile, os.O_RDWR, 0)
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

	if t.Persistent {
		if err = tunnelCtl(t.FD, syscall.TUNSETPERSIST, uintptr(1)); err != nil {
			return err
		}
	}

	t.RWC = os.NewFile(t.FD, "tun_"+t.Name)
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

func (t *TunnelInterface) Addr() (err error) {
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

func (t *TunnelInterface) Up() (err error) {
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

func (t *TunnelInterface) Down() (err error) {
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

func (t *TunnelInterface) SetMTU() (err error) {
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

func (t *TunnelInterface) SetTXQueueLen() (err error) {
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

func (t *TunnelInterface) Netmask() (err error) {
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

func (t *TunnelInterface) Delete() (err error) {
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

func (t *TunnelInterface) ApplyRoutes(V *Tunnel) (err error) {
	if IsDefaultConnection(V.Meta.IFName) || V.Meta.EnableDefaultRoute {
		err = IP_AddRoute("default", "", t.IPv4Address, "0")
		if err != nil {
			return
		}
	}

	for _, n := range V.CRR.Networks {
		t.addRoutes(V, n)
	}

	if V.CRR.VPLNetwork != nil {
		t.addRoutes(V, V.CRR.VPLNetwork)
	}

	return
}

func (t *TunnelInterface) RemoveRoutes(V *Tunnel, preserve bool) (err error) {
	defer RecoverAndLogToFile()

	for _, n := range V.CRR.Networks {
		t.deleteRoutes(V, n)
	}

	if V.CRR.VPLNetwork != nil {
		t.deleteRoutes(V, V.CRR.VPLNetwork)
	}
	if !preserve {
		if IsDefaultConnection(V.Meta.IFName) || V.Meta.EnableDefaultRoute {
			err = IP_DelRoute("default", t.IPv4Address, "0")
		}
	}

	return
}

func (t *TunnelInterface) Connect(V *Tunnel) (err error) {
	// if !t.Persistent {
	err = t.Addr()
	if err != nil {
		return
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

	if IsDefaultConnection(V.Meta.IFName) || V.Meta.EnableDefaultRoute {
		err = IP_AddRoute("default", "", t.IPv4Address, "0")
		if err != nil {
			return
		}
	}

	for _, n := range V.CRR.Networks {
		t.addRoutes(V, n)
	}

	if V.CRR.VPLNetwork != nil {
		t.addRoutes(V, V.CRR.VPLNetwork)
	}

	return
}

func (t *TunnelInterface) addRoutes(_ *Tunnel, n *ServerNetwork) (err error) {
	if n.Nat != "" {
		err = IP_AddRoute(n.Nat, "", t.IPv4Address, "0")
		if err != nil {
			return
		}
	}

	for _, v := range n.Routes {
		if strings.ToLower(v.Address) == "default" || strings.HasPrefix(v.Address, "0.0.0.0") {
			continue
		}

		err = IP_AddRoute(v.Address, "", t.IPv4Address, v.Metric)
		if err != nil {
			return
		}
	}
	return nil
}

func (t *TunnelInterface) deleteRoutes(_ *Tunnel, n *ServerNetwork) (err error) {
	if n.Nat != "" {
		err = IP_DelRoute(n.Nat, t.IPv4Address, "0")
		if err != nil {
			return
		}
	}

	for _, v := range n.Routes {
		if strings.ToLower(v.Address) == "default" || strings.Contains(v.Address, "0.0.0.0") {
			continue
		}

		err = IP_DelRoute(v.Address, t.IPv4Address, v.Metric)
		if err != nil {
			return
		}
	}

	return nil
}

func (t *TunnelInterface) Disconnect(V *Tunnel) (err error) {
	defer RecoverAndLogToFile()

	for _, n := range V.CRR.Networks {
		t.deleteRoutes(V, n)
	}

	if V.CRR.VPLNetwork != nil {
		t.deleteRoutes(V, V.CRR.VPLNetwork)
	}

	if V.Con != nil {
		V.Con.Close()
	}

	t.shouldRestart = false

	if IsDefaultConnection(V.Meta.IFName) || V.Meta.EnableDefaultRoute {
		err = IP_DelRoute("default", t.IPv4Address, "0")
	}

	err = t.Close()
	if err != nil {
		ERROR("unable to close the interface", err)
	}

	err = t.Delete()
	if err != nil {
		ERROR("unable to delete the interface", err)
	}

	RemoveTunnelInterfaceFromList(t)

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

func StartDefaultInterface() (err error) {
	CON := new(Tunnel)
	CON.Meta = findDefaultTunnelMeta()

	DEFAULT_TUNNEL, err = CreateNewTunnelInterface(CON)
	if err != nil {
		return
	}

	CON.Interface = DEFAULT_TUNNEL

	err = DEFAULT_TUNNEL.Addr()
	if err != nil {
		return
	}
	err = DEFAULT_TUNNEL.SetMTU()
	if err != nil {
		return
	}
	err = DEFAULT_TUNNEL.SetTXQueueLen()
	if err != nil {
		return
	}
	err = DEFAULT_TUNNEL.Up()
	if err != nil {
		return
	}

	GLOBAL_STATE.DefaultInterfaceOnline = true

	return
}

func tunnelCtl(fd uintptr, request uintptr, argp uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, uintptr(request), argp)
	if errno != 0 {
		return os.NewSyscallError("ioctl", errno)
	}
	return nil
}

func socketCtlv6(request uintptr, argp uintptr) error {
	fd, err := syscall.Socket(
		syscall.AF_INET6,
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
		_, r.Dst, err = net.ParseCIDR("0.0.0.0/0")
	} else {
		_, r.Dst, err = net.ParseCIDR(network)
	}

	r.Priority = mInt
	r.Gw = net.ParseIP(gateway).To4()

	DEBUG("NEW ROUTE: ", r)
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

func IP_DelRouteNoGW(network string, metric int) (err error) {
	// if IsActiveRouterIP(network) {
	// 	return
	// }
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
	// if IsActiveRouterIP(network) {
	// 	return
	// }

	mInt, err := strconv.Atoi(metric)
	if err != nil {
		return err
	}

	r := new(netlink.Route)
	if network == "default" {
		_, r.Dst, err = net.ParseCIDR("0.0.0.0/0")
	} else {
		_, r.Dst, err = net.ParseCIDR(network)
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

func RestoreDNSOnClose() {
	// not implemented for unix
}

func RestoreSaneDNSDefaults() {
	// not implemented for unix
}

func GetDNSServers(id string) error {
	// not implemented for unix
	DEFAULT_DNS_SERVERS = nil
	return nil
}

func IPv6Enabled() bool {
	out, err := exec.Command("bash", "-c", "cat /proc/sys/net/ipv6/conf/"+DEFAULT_INTERFACE_NAME+"/disable_ipv6").CombinedOutput()
	if err != nil {
		ERROR("Error getting ipv6 settings for interface: ", DEFAULT_INTERFACE_NAME, " || msg: ", err, " || output: ", string(out))
		return true
	}

	outString := string(out)
	outString = strings.TrimSpace(outString)

	return outString == "0"
}

func AdjustRoutersForTunneling() (err error) {
	defer func() {
		r := recover()
		if r != nil {
			ERROR(r, string(debug.Stack()))
		}
	}()
	DEBUG("Adjusting route metrics")

	links, _ := netlink.LinkList()
	for _, v := range links {
		routes, _ := netlink.RouteList(v, 4)
		for _, r := range routes {
			if r.Dst == nil && r.Priority < 2 {
				DEBUG("ADJUST DEFAULT ROUTE: ", r)
				_ = netlink.RouteDel(&r)
				r.Priority = 100
				_ = netlink.RouteDel(&r)
				return
			}
		}
	}

	return
}

func PrintInterfaces() (error, []byte) {
	out, err := exec.Command("bash", "-c", "ip a").Output()
	if err != nil {
		return err, nil
	}
	return nil, out
}

func PrintRoutes() (error, []byte) {
	out, err := exec.Command("bash", "-c", "ip route").Output()
	if err != nil {
		return err, nil
	}
	return nil, out
}

func PrintDNS() (error, []byte) {
	return nil, []byte(".. DNS already printed via resolv.conf and NetworkManager settings")
}
