//go:build darwin

package client

import (
	"fmt"
	"net"
	"os/exec"
	"reflect"
	"unsafe"

	"golang.org/x/sys/unix"
	wgconn "golang.zx2c4.com/wireguard/conn"
)

func applyEndpointProtect(string, net.IP) error { return nil }

func removeEndpointProtect() {}

func runProtectWatcher() { protectTickerLoop() }

func protectRoutesPresent(ifName string, gw net.IP, ifIndex int, hosts []string) bool {
	gw4 := gw.To4()
	if ifName == "" || gw4 == nil {
		return false
	}
	iface, err := net.InterfaceByName(ifName)
	if err != nil || iface.Flags&net.FlagUp == 0 {
		return false
	}
	if ifIndex != 0 && iface.Index != ifIndex {
		return false
	}
	for _, host := range hosts {
		out, err := exec.Command("route", "-n", "get", "-ifscope", ifName, host).CombinedOutput()
		if err != nil {
			return false
		}
		got := parseRouteGetGateway(string(out))
		if got == nil || !got.Equal(gw4) {
			return false
		}
	}
	return true
}

func physicalIPv4Default() (ifName string, gw net.IP, ifIndex int, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", nil, 0, err
	}
	tunnels := tunnelNameSet()
	tunnGws := tunnelGatewaySet()
	preferred := ""
	if n := STATE.Load().DefaultInterfaceName.Load(); n != nil {
		preferred = *n
	}
	try := func(iface net.Interface) (string, net.IP, int, bool) {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			return "", nil, 0, false
		}
		if _, ok := tunnels[iface.Name]; ok {
			return "", nil, 0, false
		}
		out, err := exec.Command("route", "-n", "get", "-ifscope", iface.Name, "default").CombinedOutput()
		if err != nil {
			return "", nil, 0, false
		}
		g := parseRouteGetGateway(string(out))
		if g == nil {
			return "", nil, 0, false
		}
		if _, ok := tunnGws[g.String()]; ok {
			return "", nil, 0, false
		}
		return iface.Name, g, iface.Index, true
	}
	if preferred != "" {
		for _, iface := range ifaces {
			if iface.Name != preferred {
				continue
			}
			if n, g, idx, ok := try(iface); ok {
				return n, g, idx, nil
			}
		}
	}
	for _, iface := range ifaces {
		if n, g, idx, ok := try(iface); ok {
			return n, g, idx, nil
		}
	}
	return "", nil, 0, fmt.Errorf("no physical IPv4 default route")
}

func bindProtectToInterface(b wgconn.Bind, ifIndex uint32) error {
	if ifIndex == 0 {
		return fmt.Errorf("interface index is 0")
	}
	v4, v6 := udpConnsFromBind(b)
	if v4 == nil && v6 == nil {
		return fmt.Errorf("no UDP sockets on WireGuard bind")
	}
	if v4 != nil {
		if err := setBoundIF(v4, ifIndex, false); err != nil {
			return fmt.Errorf("IPv4 IP_BOUND_IF: %w", err)
		}
	}
	if v6 != nil {
		if err := setBoundIF(v6, ifIndex, true); err != nil {
			DEBUG("IPv6 IPV6_BOUND_IF skipped: ", err)
		}
	}
	return nil
}

func setBoundIF(c *net.UDPConn, ifIndex uint32, v6 bool) error {
	sc, err := c.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	if err := sc.Control(func(fd uintptr) {
		if v6 {
			sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, int(ifIndex))
		} else {
			sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, int(ifIndex))
		}
	}); err != nil {
		return err
	}
	return sockErr
}

func udpConnsFromBind(b wgconn.Bind) (v4, v6 *net.UDPConn) {
	v := reflect.ValueOf(b)
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}
	return exportedUDPConn(v, "ipv4"), exportedUDPConn(v, "ipv6")
}

func exportedUDPConn(v reflect.Value, field string) *net.UDPConn {
	f := v.FieldByName(field)
	if !f.IsValid() || f.Kind() != reflect.Ptr || f.IsNil() {
		return nil
	}
	ptr := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
	c, _ := ptr.Interface().(*net.UDPConn)
	return c
}
