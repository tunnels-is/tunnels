//go:build darwin

package client

import (
	"fmt"
	"net"
	"reflect"
	"unsafe"

	"golang.org/x/sys/unix"
	wgconn "golang.zx2c4.com/wireguard/conn"
)

func applyEndpointProtect(string, net.IP) error { return nil }

func removeEndpointProtect() {}

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
