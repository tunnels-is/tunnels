//go:build darwin

package core

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type TunnelInterface struct {
	tunnel        atomic.Pointer[*Tunnel]
	shouldRestart bool

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

func CreateNewTunnelInterface(
	VC *Tunnel,
) (IF *TunnelInterface, err error) {
	defer RecoverAndLogToFile()

	IF = &TunnelInterface{
		Name:        VC.Meta.IFName,
		IPv4Address: VC.Meta.IPv4Address,
		Gateway:     VC.Meta.IPv4Address,
		NetMask:     VC.Meta.NetMask,
		TxQueuelen:  VC.Meta.TxQueueLen,
		MTU:         VC.Meta.MTU,
		Persistent:  VC.Meta.Persistent,
		// IPv6Address: "fe80::1",
	}

	err = IF.Create()
	if err != nil {
		return
	}

	return
}

func StartDefaultInterface() (err error) {
	CON := new(Tunnel)
	CON.Meta = findDefaultTunnelMeta()

	DEFAULT_TUNNEL, err = CreateNewTunnelInterface(CON)
	if err != nil {
		return
	}

	CON.Interface = DEFAULT_TUNNEL

	err = DEFAULT_TUNNEL.Up()
	if err != nil {
		return
	}

	err = DEFAULT_TUNNEL.SetMTU()
	if err != nil {
		return
	}

	// err = DEFAULT_TUNNEL.SetTXQueueLen()
	// if err != nil {
	// 	return
	// }

	GLOBAL_STATE.DefaultInterfaceOnline = true

	return
}

func (t *TunnelInterface) Close() error {
	if t.RWC != nil {
		return t.RWC.Close()
	}
	return nil
}

const (
	appleUTUNCtl = "com.apple.net.utun_control"
	/*
	 * From ioctl.h:
	 * #define	IOCPARM_MASK	0x1fff		// parameter length, at most 13 bits
	 * ...
	 * #define	IOC_OUT		0x40000000	// copy out parameters
	 * #define	IOC_IN		0x80000000	// copy in parameters
	 * #define	IOC_INOUT	(IOC_IN|IOC_OUT)
	 * ...
	 * #define _IOC(inout,group,num,len) \
	 * 	(inout | ((len & IOCPARM_MASK) << 16) | ((group) << 8) | (num))
	 * ...
	 * #define	_IOWR(g,n,t)	_IOC(IOC_INOUT,	(g), (n), sizeof(t))
	 *
	 * From kern_control.h:
	 * #define CTLIOCGINFO     _IOWR('N', 3, struct ctl_info)	// get id from name
	 *
	 */
	appleCTLIOCGINFO = (0x40000000 | 0x80000000) | ((100 & 0x1fff) << 16) | uint32(byte('N'))<<8 | 3
	/*
	 * #define _IOW(g,n,t) _IOC(IOC_IN, (g), (n), sizeof(t))
	 * #define TUNSIFMODE _IOW('t', 94, int)
	 */
	appleTUNSIFMODE = (0x80000000) | ((4 & 0x1fff) << 16) | uint32(byte('t'))<<8 | 94
)

/*
 * struct sockaddr_ctl {
 *     u_char sc_len; // depends on size of bundle ID string
 *     u_char sc_family; // AF_SYSTEM
 *     u_int16_t ss_sysaddr; // AF_SYS_KERNCONTROL
 *     u_int32_t sc_id; // Controller unique identifier
 *     u_int32_t sc_unit; // Developer private unit number
 *     u_int32_t sc_reserved[5];
 * };
 */
type sockaddrCtl struct {
	scLen      uint8
	scFamily   uint8
	ssSysaddr  uint16
	scID       uint32
	scUnit     uint32
	scReserved [5]uint32
}

var sockaddrCtlSize uintptr = 32

func (t *TunnelInterface) Create() (err error) {
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
		2, /* #define SYSPROTO_CONTROL 2 */
		2, /* #define UTUN_OPT_IFNAME 2 */
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

func (t *TunnelInterface) Up() (err error) {
	DEBUG("ifconfig", t.SystemName, t.IPv4Address, t.Gateway, "up")

	out, err := exec.Command("ifconfig", t.SystemName, t.IPv4Address, t.Gateway, "up").CombinedOutput()
	if err != nil {
		ERROR("unable to bring up tunnel adapter: ", string(out), " err: ", err)
		return err
	}

	return
}

func (t *TunnelInterface) Down() (err error) {
	DEBUG("ifconfig", t.SystemName, "down")

	out, err := exec.Command("ifconfig", t.SystemName, "down").CombinedOutput()
	if err != nil {
		ERROR("unable to bring down tunnel adapter: ", string(out), " err: ", err)
		return err
	}

	return
}

func (t *TunnelInterface) Addr() (err error) {
	// not needed on macos
	return nil
}

func (t *TunnelInterface) SetMTU() (err error) {
	DEBUG("ifconfig", t.SystemName, "mtu", strconv.FormatInt(int64(t.MTU), 10))

	out, err := exec.Command("ifconfig", t.SystemName, "mtu", strconv.FormatInt(int64(t.MTU), 10)).CombinedOutput()
	if err != nil {
		ERROR("Unable to change mtu out: ", string(out), " err: ", err)
		return err
	}
	return
}

func (t *TunnelInterface) Netmask() (err error) {
	return nil
}

func (t *TunnelInterface) Delete() (err error) {
	return nil
}

func (t *TunnelInterface) SetTXQueueLen() (err error) {
	// DEBUG("ifconfig", t.SystemName, "txqueuelen", strconv.FormatInt(int64(t.TxQueuelen), 10))
	//
	// out, err := exec.Command("ifconfig", t.SystemName, "txqueuelen", strconv.FormatInt(int64(t.TxQueuelen), 10)).CombinedOutput()
	// if err != nil {
	// 	ERROR("Unable to change txqueuelen out: ", string(out), " err: ", err)
	// 	return err
	// }
	return
}

func (t *TunnelInterface) addRoutes(n *ServerNetwork) (err error) {
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

func (t *TunnelInterface) deleteRoutes(V *Tunnel, n *ServerNetwork) (err error) {
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

func (t *TunnelInterface) ApplyRoutes(V *Tunnel) (err error) {
	if IsDefaultConnection(V.Meta.IFName) || V.Meta.EnableDefaultRoute {
		// _ = IP_DelDefaultRoute()
		err = IP_AddDefaultRoute(t.IPv4Address)
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
			_ = IP_DelDefaultRoute()
			err = IP_AddDefaultRoute(DEFAULT_GATEWAY.To4().String())
			if err != nil {
				ERROR("unable to restore default route", err)
			}

		}
	}

	return
}

func (t *TunnelInterface) Connect(V *TUN) (err error) {
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
	// err = t.SetTXQueueLen()
	// if err != nil {
	// 	return
	// }
	// }

	if IsDefaultConnection(V.Meta.IFName) || V.Meta.EnableDefaultRoute {
		_ = IP_DelDefaultRoute()
		err = IP_AddDefaultRoute(t.IPv4Address)
		if err != nil {
			return
		}
	}

	if V.CRR.VPLNetwork != nil {
		t.addRoutes(V, V.CRR.VPLNetwork)
	}

	for _, n := range V.CRR.Networks {
		t.addRoutes(V, n)
	}

	return nil
}

func (t *TunnelInterface) Disconnect(V *Tunnel) (err error) {
	defer RecoverAndLogToFile()

	t.shouldRestart = false
	if V.Con != nil {
		V.Con.Close()
	}

	for _, n := range V.CRR.Networks {
		t.deleteRoutes(V, n)
	}

	if V.CRR.VPLNetwork != nil {
		t.deleteRoutes(V, V.CRR.VPLNetwork)
	}

	if IsDefaultConnection(V.Meta.IFName) || V.Meta.EnableDefaultRoute {
		_ = IP_DelDefaultRoute()
		err = IP_AddDefaultRoute(DEFAULT_GATEWAY.To4().String())
		if err != nil {
			ERROR("unable to restore default route", err)
		}
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
	_ = IP_DelRoute(network, "", "")

	DEBUG("route", "-n", "add", "-net", network, gateway)

	out, err := exec.Command("route", "-n", "add", "-net", network, gateway).CombinedOutput()
	if err != nil {
		ERROR("Unable to add route: ", string(out), " err: ", err)
		return err
	}

	return
}

func IP_DelRoute(network string, gateway string, metric string) (err error) {
	// if IsActiveRouterIP(network) {
	// 	return
	// }

	DEBUG("route", "-n", "delete", "-net", network)

	out, err := exec.Command("route", "-n", "delete", "-net", network).CombinedOutput()
	if err != nil {
		ERROR("Unable to delete route: ", string(out), " err: ", err)
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
	if DEFAULT_INTERFACE_NAME == "" {
		DEBUG("no default interface name found, assuming ipv6 is enabled")
		return true
	}

	iface, err := net.InterfaceByName(DEFAULT_INTERFACE_NAME)
	if err != nil {
		ERROR("Error retrieving interface: ", err)
		return false
	}

	addrs, err := iface.Addrs()
	if err != nil {
		ERROR("Error retrieving addresses: ", err)
		return false
	}

	for _, addr := range addrs {
		// Check for IPv6 address
		if strings.Contains(addr.String(), ":") {
			DEBUG("ipv6 is enabled on the default interface")
			return true
		}
	}

	DEBUG("ipv6 is not enabled on the default interface")
	return false
}
