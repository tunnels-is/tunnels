package client

import (
	"net"
	"reflect"
	"unsafe"

	wgconn "golang.zx2c4.com/wireguard/conn"
)

func unwrapBind(b wgconn.Bind) wgconn.Bind {
	for b != nil {
		switch v := b.(type) {
		case *punchBind:
			if v == nil {
				return nil
			}
			b = v.inner
		case *ifaceProtectBind:
			if v == nil {
				return nil
			}
			b = v.Bind
		default:
			return b
		}
	}
	return nil
}

func walkUDPConns(b wgconn.Bind, fn func(*net.UDPConn, bool)) {
	b = unwrapBind(b)
	if b == nil || fn == nil {
		return
	}
	v := reflect.ValueOf(b)
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	for _, name := range []string{"ipv4", "ipv6"} {
		f := v.FieldByName(name)
		if !f.IsValid() || f.Kind() != reflect.Ptr || f.IsNil() || !f.CanAddr() {
			continue
		}
		ptr := reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
		c, _ := ptr.Interface().(*net.UDPConn)
		if c != nil {
			fn(c, name == "ipv6")
		}
	}
}

func setBindDontFrag(b wgconn.Bind) func() {
	var undos []func()
	walkUDPConns(b, func(c *net.UDPConn, v6 bool) {
		if u := setUDPDontFrag(c, v6); u != nil {
			undos = append(undos, u)
		}
	})
	if u := setExtraBindDontFrag(b); u != nil {
		undos = append(undos, u)
	}
	return func() {
		for _, u := range undos {
			u()
		}
	}
}
