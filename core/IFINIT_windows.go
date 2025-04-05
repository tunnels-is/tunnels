//go:build windows

package core

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type TInterface struct {
	tunnel        atomic.Pointer[*TUN]
	shouldRestart bool
	shouldExit    bool
	exitChannel   chan byte

	Name        string
	IPv4Address string
	IPv6Address string
	NetMask     string
	TxQueuelen  int32
	MTU         int32
	Persistent  bool
	Gateway     string

	// DLL
	WDLL *DLL

	// Windows specific
	GUID          windows.GUID
	NamePtr       *uint16
	TypePtr       *uint16
	UNamePtr      uintptr
	UTypePtr      uintptr
	UGUIDPtr      uintptr
	Handle        uintptr
	SessionHandle uintptr
	RingCap       uint32
	GatewayMetric string
	IFIndex       int
}

func (t *TInterface) VerifyOrLoadPointer(method string) {
	// if atomic.LoadPointer(unsafe.Pointer(&t.WDLL.module)) == nil {
	// WINTUN IS GONE
	// }
}

func CreateNewTunnelInterface(
	meta *TunnelMETA,
) (IF *TInterface, err error) {
	defer RecoverAndLogToFile()

	var GUID windows.GUID
	if meta.WindowsGUID != "" {
		GUID, err = windows.GUIDFromString(meta.WindowsGUID)
		if err != nil {
			ERROR("Unable to create Windows UID from string: ", meta.WindowsGUID)
			return
		}
	} else {
		GUID = *new(windows.GUID)
	}

	IF = &TInterface{
		Name:        meta.IFName,
		IPv4Address: meta.IPv4Address,
		NetMask:     meta.NetMask,
		TxQueuelen:  meta.TxQueueLen,
		MTU:         meta.MTU,

		// hardcoded for now
		IPv6Address:   "fe80::1",
		GatewayMetric: "2000",
		// Gateway:       DEFAULT_GATEWAY.To4().String(),
		// Gateway: "127.0.0.1",
		Gateway: meta.IPv4Address,
		GUID:    GUID,
		RingCap: 0x4000000,
		// RingCap: 0x8000000,
	}
	DEBUG(fmt.Sprintf("New tunnel interface/adapter: %v", IF))

	IF.WDLL = new(DLL)
	_ = IF.WDLL.Init("./wintun.dll")
	return IF, err
}

func (t *TInterface) CreateOrOpen() (err error) {
	t.NamePtr, err = windows.UTF16PtrFromString(t.Name)
	if err != nil {
		DEBUG(fmt.Sprintf("Adapter creation error (%s) err: %s", t.Name, err))
		return
	}
	t.UNamePtr = uintptr(unsafe.Pointer(t.NamePtr))

	t.TypePtr, err = windows.UTF16PtrFromString("tunnels")
	if err != nil {
		DEBUG(fmt.Sprintf("Adapter creation error (%s) err: %s", t.Name, err))
		return
	}
	t.UTypePtr = uintptr(unsafe.Pointer(t.TypePtr))

	// https://github.com/microsoft/go-winio/blob/main/pkg/guid/guid.go
	// https://github.com/WireGuard/wintun/pull/7
	// https://github.com/WireGuard/wintun/blob/master/README.md#wintuncreateadapter
	t.UGUIDPtr = uintptr(unsafe.Pointer(&t.GUID))

	var msg error
	add, err := t.WDLL.GetAddr(0)
	if err != nil {
		return
	}
	t.Handle, _, msg = syscall.SyscallN(
		add.UPTR,
		// t.WDLL.PTR_OpenAdapter,
		t.UNamePtr,
	)

	DEBUG(fmt.Sprintf("Opened adapter (%s) err: %s", t.Name, msg))

	if t.Handle == 0 {
		add, err = t.WDLL.GetAddr(2)
		if err != nil {
			return
		}
		t.Handle, _, msg = syscall.SyscallN(
			// WCreateAdapter.Addr(),
			// t.WDLL.PTR_CreateAdapter,
			add.UPTR,
			t.UNamePtr,
			t.UTypePtr,
			t.UGUIDPtr,
		)
		if t.Handle == 0 {
			err = msg
			ERROR(fmt.Sprintf("Created adapter (%s) err: %s", t.Name, msg))
			return
		}
		DEBUG(fmt.Sprintf("Created adapter (%s)", t.Name))
	}

	// runtime.SetFinalizer(IF.Handle, AdapterCleanup)
	return
}

func (t *TInterface) Up() (err error) {
	var msg error
	add, err := t.WDLL.GetAddr(3)
	if err != nil {
		return
	}
	t.SessionHandle, _, msg = syscall.SyscallN(
		// WStartSession.Addr(),
		// t.WDLL.PTR_StartSession,
		add.UPTR,
		t.Handle,
		uintptr(t.RingCap))
	if t.SessionHandle == 0 {
		err = msg
		ERROR(fmt.Sprintf("Interface/Adapter (%s) state (up) error(%s)", t.Name, err))
	} else {
		DEBUG(fmt.Sprintf("Interface/Adapter (%s) state (up)", t.Name))
	}
	return
}

func (t *TInterface) Down() (err error) {
	// cmd := exec.Command(
	// 	"netsh",
	// 	"interface",
	// 	"ipv4",
	// 	"delete",
	// 	"address",
	// 	`name="`+t.Name+`"`,
	// 	"addr=",
	// 	t.IPv4Address,
	// 	"gateway=",
	// 	"All",
	// )
	//
	// DEBUG(
	// 	"netsh",
	// 	"interface",
	// 	"ipv4",
	// 	"delete",
	// 	"address",
	// 	`name="`+t.Name+`"`,
	// 	"addr=",
	// 	t.IPv4Address,
	// 	"gateway=",
	// 	"All",
	// )
	//
	// cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	// ob, cerr := cmd.Output()
	// if cerr != nil {
	// 	ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
	// 	return cerr
	// }

	// r1, _, msg := syscall.SyscallN(
	// 	t.WDLL.PTR_EndSession,
	// 	t.Handle)
	// if r1 == 0 {
	// 	err = msg
	// 	ERROR(fmt.Sprintf("Interface/Adapter (%s) state (close) error(%s)", t.Name, err))
	// } else {
	// 	DEBUG(fmt.Sprintf("Interface/Adapter (%s) state (close)", t.Name))
	// }

	return nil
}

func (t *TInterface) Addr() (err error) {
	cmd := exec.Command(
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
		"store=presistent",
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	ob, err := cmd.Output()
	if err != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, err))
		return err
	}

	return nil
}

func (t *TInterface) Close() (err error) {
	return t.Delete()
}

func (t *TInterface) Delete() (err error) {
	add, err := t.WDLL.GetAddr(1)
	if err != nil {
		return
	}
	r1, _, msg := syscall.SyscallN(
		// WCloseAdapter.Addr(),
		// t.WDLL.PTR_CloseAdapter,
		add.UPTR,
		t.Handle)
	if r1 == 0 {
		err = msg
		ERROR(fmt.Sprintf("Interface/Adapter (%s) state (delete) error(%s)", t.Name, err))
	} else {
		DEBUG(fmt.Sprintf("Interface/Adapter (%s) state (delete)", t.Name))
	}

	add, err = t.WDLL.GetAddr(4)
	if err != nil {
		return
	}
	r1, _, msg = syscall.SyscallN(
		// WEndSession.Addr(),
		// t.WDLL.PTR_EndSession,
		add.UPTR,
		t.SessionHandle)
	if r1 == 0 {
		err = msg
		ERROR(fmt.Sprintf("Interface/Adapter (%s) state (close) error(%s)", t.Name, err))
	} else {
		DEBUG(fmt.Sprintf("Interface/Adapter (%s) state (close)", t.Name))
	}

	return
}

func IP_RouteMetric(network string, ifname string, metric string) (err error) {
	if metric == "0" {
		metric = "1"
	}

	cmd := exec.Command(
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

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
	}

	return
}

func IP_AddRoute(
	network string,
	ifName string,
	gateway string,
	metric string,
) (err error) {
	if metric == "0" {
		metric = "1"
	}

	_ = IP_DelRoute(network, gateway, metric)

	cmd := exec.Command(
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

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	ob, cerr := cmd.Output()

	if cerr != nil {
		return fmt.Errorf("%s - out: %s", cerr, ob)
	}

	return
}

func IP_DelRoute(network string, _ string, _ string) (err error) {
	// if IsActiveRouterIP(network) {
	// 	return
	// }

	cmd := exec.Command(
		"route",
		"DELETE",
		network,
	)

	DEBUG(
		"route",
		"DELETE",
		network,
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
	}

	return
}

func DNS_Del(IFNameOrIndex string) (err error) {
	cmd := exec.Command(
		"netsh",
		"interface",
		"ipv4",
		"delete",
		"dnsservers",
		`name=`+IFNameOrIndex,
		"all",
	)
	DEBUG(
		"netsh",
		"interface",
		"ipv4",
		"delete",
		"dnsservers",
		`name=`+IFNameOrIndex,
		"all",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
	}

	return nil
}

func DNS_Set(IFNameOrIndex, DNSIP, Index string) (err error) {
	cmd := exec.Command(
		"netsh",
		"interface",
		"ipv4",
		"add",
		"dnsservers",
		`name=`+IFNameOrIndex,
		"address="+DNSIP,
		"index="+Index,
	)
	DEBUG(
		"netsh",
		"interface",
		"ipv4",
		"add",
		"dnsservers",
		`name=`+IFNameOrIndex,
		"address="+DNSIP,
		"index="+Index,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
	}

	return nil
}

func (t *TInterface) SetMTU() error {
	cmd := exec.Command(
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

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	ob, cerr := cmd.Output()
	if cerr != nil {
		ERROR(fmt.Sprintf("%s - out: %s ", ob, cerr))
		return cerr
	}
	return nil
}

func (t *TInterface) addRoutes(n *ServerNetwork) (err error) {
	if n.Nat != "" {
		err = IP_AddRoute(n.Nat, t.Name, t.IPv4Address, "0")
		if err != nil {
			return err
		}
	}

	for _, v := range n.Routes {
		// default routes are not allowed on windows
		if strings.ToLower(v.Address) == "default" || strings.HasPrefix(v.Address, "0.0.0.0") {
			continue
		}

		err = IP_AddRoute(v.Address, t.Name, t.IPv4Address, v.Metric)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TInterface) deleteRoutes(n *ServerNetwork) (err error) {
	err = IP_DelRoute(n.Nat, t.IPv4Address, "0")
	if err != nil {
		return err
	}

	for _, v := range n.Routes {
		if strings.ToLower(v.Address) == "default" || strings.Contains(v.Address, "0.0.0.0") {
			continue
		}

		err = IP_DelRoute(v.Address, t.IPv4Address, v.Metric)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *TInterface) ApplyRoutes(V *TUN) (err error) {

	meta := V.meta.Load()
	if IsDefaultConnection(meta.IFName) || meta.EnableDefaultRoute {
		t.GatewayMetric = "1"
		err = IP_RouteMetric("0.0.0.0/0", t.Name, "1")
		if err != nil {
			return
		}
	}

	for _, n := range V.CRResponse.Networks {
		t.addRoutes(n)
	}

	if V.CRResponse.VPLNetwork != nil {
		t.addRoutes(V.CRResponse.VPLNetwork)
	}

	return
}

func (t *TInterface) RemoveRoutes(V *Tunnel, preserve bool) (err error) {
	defer RecoverAndLogToFile()

	for _, n := range V.CRRespose.Networks {
		t.deleteRoutes(n)
	}

	if V.CRRespose.VPLNetwork != nil {
		t.deleteRoutes(V.CRRespose.VPLNetwork)
	}
	if !preserve {
		if IsDefaultConnection(V.Meta.IFName) || V.Meta.EnableDefaultRoute {
			t.GatewayMetric = "2000"
			err = IP_RouteMetric("0.0.0.0/0", t.Name, "2000")
			if err != nil {
				return
			}
		}
	}

	return
}

func (t *TInterface) Connect(tun *TUN) (err error) {
	err = t.CreateOrOpen()
	if err != nil {
		return
	}

	t.GatewayMetric = "2000"
	if err = t.Addr(); err != nil {
		return
	}
	if err = t.Up(); err != nil {
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
	t.exitChannel = make(chan byte, 10)
	meta := tun.meta.Load()

	if IsDefaultConnection(meta.IFName) || meta.EnableDefaultRoute {
		// Gateway metric is what determines default
		// routing on windows. The interface will always
		// have a default router on creation.
		t.GatewayMetric = "1"
		err = IP_RouteMetric("0.0.0.0/0", t.Name, "1")
		if err != nil {
			return
		}
	}

	// _ = DNS_Del(strconv.Itoa(DEFAULT_INTERFACE_ID))
	// err = DNS_Set(strconv.Itoa(DEFAULT_INTERFACE_ID), "127.0.0.1", "1")

	if tun.CRResponse.VPLNetwork != nil {
		t.addRoutes(tun.CRResponse.VPLNetwork)
	}

	for _, n := range tun.CRResponse.Networks {
		t.addRoutes(n)
	}

	return
}

func (t *TInterface) CloseReadAndWriteLoop() {
	exitTimeout := time.NewTicker(10 * time.Second)
	exitCount := 0
exitLoop:
	for {
		select {
		case _ = <-t.exitChannel:
			exitCount++
			if exitCount >= 2 {
				break exitLoop
			}
		case _ = <-exitTimeout.C:
			ERROR("timed out waiting for reader and writer to exit")
			return
		}
	}
	return
}

func (t *TInterface) Disconnect(tun *TUN) (err error) {
	defer RecoverAndLogToFile()

	t.shouldRestart = false
	t.shouldExit = true

	for _, n := range tun.CRResponse.Networks {
		t.deleteRoutes(n)
	}

	if tun.CRResponse.VPLNetwork != nil {
		t.deleteRoutes(tun.CRResponse.VPLNetwork)
	}

	if tun.connection != nil {
		tun.connection.Close()
	}

	t.CloseReadAndWriteLoop()

	err = t.Close()
	if err != nil {
		ERROR("unable to close the interface", err)
	}

	err = t.Delete()
	if err != nil {
		ERROR("unable to delete the interface", err)
	}

	return
}

var (
	GUID *windows.GUID

	// WINDOWS DLL
	IPHLPApi = syscall.NewLazyDLL("iphlpapi.dll")
	GetTCP   = IPHLPApi.NewProc("GetExtendedTcpTable")
	GetUDP   = IPHLPApi.NewProc("GetExtendedUdpTable")
	SetTCP   = IPHLPApi.NewProc("SetTcpEntry")
)

const (
	PacketSizeMax                       = 0xffff    // Maximum packet size
	RingCapacityMin                     = 0x20000   // Minimum ring capacity (128 kiB)
	RingCapacityMax                     = 0x4000000 // Maximum ring capacity (64 MiB)
	AdapterNameMax                      = 128
	LOAD_LIBRARY_SEARCH_APPLICATION_DIR = 0x00000200
	LOAD_LIBRARY_SEARCH_SYSTEM32        = 0x00000800

	// experimental
	MIB_TCP_TABLE_OWNER_PID_ALL = 5
	MIB_TCP_STATE_DELETE_TCB    = 12
)

func logMessage(_ int, timestamp uint64, msg *uint16) int {
	DEBUG(timestamp, " > ", windows.UTF16PtrToString(msg))
	return 0
}

func (t *TInterface) ReceivePacket() (packet []byte, size uint16, err error) {
	add, err := t.WDLL.GetAddr(5)
	if err != nil {
		return
	}
	r0, _, msg := syscall.SyscallN(
		add.UPTR,
		// t.WDLL.PTR_ReceivePacket,
		t.SessionHandle,
		uintptr(unsafe.Pointer(&size)),
	)

	if r0 == 0 {
		err = msg
		return
	}

	packet = unsafe.Slice((*byte)(unsafe.Pointer(r0)), size)

	return
}

func (t *TInterface) ReleaseReceivePacket(packet []byte) (err error) {
	if packet == nil {
		return
	}
	add, err := t.WDLL.GetAddr(7)
	if err != nil {
		return
	}
	r0, _, msg := syscall.SyscallN(
		add.UPTR,
		// t.WDLL.PTR_ReleaseReceivedPacket,
		t.SessionHandle,
		uintptr(unsafe.Pointer(&packet[0])),
	)
	if r0 == 0 {
		err = msg
		return
	}

	return
}

func (t *TInterface) AllocateSendPacket(packetSize int) (packet []byte, err error) {
	add, err := t.WDLL.GetAddr(6)
	if err != nil {
		return
	}
	r0, _, msg := syscall.SyscallN(
		add.UPTR,
		t.SessionHandle,
		uintptr(packetSize),
	)

	if r0 == 0 {
		err = msg
		return
	}

	packet = unsafe.Slice((*byte)(unsafe.Pointer(r0)), packetSize)
	return
}

func (t *TInterface) SendPacket(packet []byte) (err error) {
	add, err := t.WDLL.GetAddr(8)
	if err != nil {
		return
	}
	_, _, _ = syscall.SyscallN(
		// t.WDLL.PTR_SendPacket,
		add.UPTR,
		t.SessionHandle,
		uintptr(unsafe.Pointer(&packet[0])),
	)
	return
}

type DLL struct {
	module          uintptr
	moduleHandle    windows.Handle
	moduleUnsafePTR *unsafe.Pointer

	// PTR_OpenAdapter           uintptr
	// PTR_CloseAdapter          uintptr
	// PTR_CreateAdapter         uintptr
	// PTR_StartSession          uintptr
	// PTR_EndSession            uintptr
	// PTR_ReceivePacket         uintptr
	// PTR_AllocateSendPacket    uintptr
	// PTR_ReleaseReceivedPacket uintptr
	// PTR_SendPacket            uintptr
	// PTR_SetLogger uintptr

	// NEW
	AddressMap [100]*DLLAddress
}
type DLLAddress struct {
	Name string
	PTR  *unsafe.Pointer
	UPTR uintptr
}

func (d *DLL) GetAddr(index int) (addr *DLLAddress, err error) {
	addr = d.AddressMap[index]
	if addr.PTR != nil {
		if atomic.LoadPointer(addr.PTR) != nil {
			return
		}
	}

	err = d.LazyLoadLibrary("./wintun.dll")
	if err != nil {
		return
	}

	addr.UPTR, err = windows.GetProcAddress(d.moduleHandle, addr.Name)
	if err != nil {
		ERROR("unable to load proc address: ", err)
		return
	}

	atomic.StorePointer(
		(*unsafe.Pointer)(unsafe.Pointer(&addr.PTR)),
		unsafe.Pointer(addr.UPTR),
	)

	return
}

func (d *DLL) LazyLoadLibrary(name string) (err error) {
	if d.moduleUnsafePTR != nil {
		if atomic.LoadPointer(d.moduleUnsafePTR) != nil {
			return
		}
	}

	d.moduleHandle, err = windows.LoadLibraryEx(
		name,
		0,
		LOAD_LIBRARY_SEARCH_APPLICATION_DIR|LOAD_LIBRARY_SEARCH_SYSTEM32,
	)
	if err != nil {
		ERROR("Unable to load DLL: ", name, " ERR: ", err)
		return err
	}
	atomic.StoreUintptr(
		&d.module,
		uintptr(unsafe.Pointer(d.moduleHandle)),
	)

	atomic.StorePointer(
		(*unsafe.Pointer)(unsafe.Pointer(&d.moduleUnsafePTR)),
		unsafe.Pointer(d.moduleHandle),
	)
	return
}

func (d *DLL) Init(name string) (err error) {
	err = d.LazyLoadLibrary("./wintun.dll")
	if err != nil {
		return
	}

	d.AddressMap[0] = &DLLAddress{Name: "WintunOpenAdapter"}
	d.AddressMap[1] = &DLLAddress{Name: "WintunCloseAdapter"}
	d.AddressMap[2] = &DLLAddress{Name: "WintunCreateAdapter"}
	d.AddressMap[3] = &DLLAddress{Name: "WintunStartSession"}
	d.AddressMap[4] = &DLLAddress{Name: "WintunEndSession"}
	d.AddressMap[5] = &DLLAddress{Name: "WintunReceivePacket"}
	d.AddressMap[6] = &DLLAddress{Name: "WintunAllocateSendPacket"}
	d.AddressMap[7] = &DLLAddress{Name: "WintunReleaseReceivePacket"}
	d.AddressMap[8] = &DLLAddress{Name: "WintunSendPacket"}
	d.AddressMap[9] = &DLLAddress{Name: "WintunSetLogger"}

	d.AddressMap[9].UPTR, err = windows.GetProcAddress(d.moduleHandle, "WintunSetLogger")
	if err != nil {
		ERROR("unable to load proc address: ", err)
		return
	}

	r1, r2, err := syscall.SyscallN(
		d.AddressMap[9].UPTR,
		windows.NewCallback(logMessage),
	)
	DEBUG("Adapter logger created: ", r1, r2, err)

	return
}
