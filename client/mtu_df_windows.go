//go:build windows

package client

import (
	"errors"
	"net"
	"reflect"
	"unsafe"

	"golang.org/x/sys/windows"
	wgconn "golang.zx2c4.com/wireguard/conn"
)

func isMsgTooBig(err error) bool {
	return errors.Is(err, windows.WSAEMSGSIZE)
}

func setUDPDontFrag(c *net.UDPConn, v6 bool) func() {
	if c == nil {
		return func() {}
	}
	raw, err := c.SyscallConn()
	if err != nil {
		return func() {}
	}
	level, opt := windows.IPPROTO_IP, windows.IP_MTU_DISCOVER
	if v6 {
		level, opt = windows.IPPROTO_IPV6, windows.IPV6_MTU_DISCOVER
	}
	val := windows.IP_PMTUDISC_DO
	var prev int
	havePrev := false
	_ = raw.Control(func(fd uintptr) {
		h := windows.Handle(fd)
		if p, gerr := windows.GetsockoptInt(h, level, opt); gerr == nil {
			prev = p
			havePrev = true
		}
		_ = windows.SetsockoptInt(h, level, opt, val)
	})
	if !havePrev {
		return func() {}
	}
	return func() {
		_ = raw.Control(func(fd uintptr) {
			_ = windows.SetsockoptInt(windows.Handle(fd), level, opt, prev)
		})
	}
}

// setExtraBindDontFrag covers WinRingBind, which keeps raw sockets instead of net.UDPConn.
func setExtraBindDontFrag(b wgconn.Bind) func() {
	b = unwrapBind(b)
	if b == nil {
		return func() {}
	}
	v := reflect.ValueOf(b)
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return func() {}
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct || !v.CanAddr() {
		return func() {}
	}
	var undos []func()
	for _, name := range []string{"v4", "v6"} {
		f := v.FieldByName(name)
		if !f.IsValid() || f.Kind() != reflect.Struct {
			continue
		}
		sock := f.FieldByName("sock")
		if !sock.IsValid() || !sock.CanAddr() {
			continue
		}
		copied := reflect.NewAt(sock.Type(), unsafe.Pointer(sock.UnsafeAddr())).Elem()
		if copied.Kind() != reflect.Uintptr {
			continue
		}
		h := windows.Handle(copied.Uint())
		if h == 0 {
			continue
		}
		level, opt := windows.IPPROTO_IP, windows.IP_MTU_DISCOVER
		if name == "v6" {
			level, opt = windows.IPPROTO_IPV6, windows.IPV6_MTU_DISCOVER
		}
		prev, gerr := windows.GetsockoptInt(h, level, opt)
		if gerr != nil {
			prev = 0
		}
		if err := windows.SetsockoptInt(h, level, opt, windows.IP_PMTUDISC_DO); err != nil {
			continue
		}
		undos = append(undos, func() {
			_ = windows.SetsockoptInt(h, level, opt, prev)
		})
	}
	return func() {
		for _, u := range undos {
			u()
		}
	}
}
