//go:build darwin

package client

import (
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func isMsgTooBig(err error) bool {
	return errors.Is(err, syscall.EMSGSIZE)
}

func setUDPDontFrag(c *net.UDPConn, v6 bool) func() {
	level, opt := unix.IPPROTO_IP, unix.IP_DONTFRAG
	if v6 {
		level, opt = unix.IPPROTO_IPV6, unix.IPV6_DONTFRAG
	}
	if c == nil {
		return func() {}
	}
	raw, err := c.SyscallConn()
	if err != nil {
		return func() {}
	}
	var prev int
	havePrev := false
	_ = raw.Control(func(fd uintptr) {
		if p, gerr := unix.GetsockoptInt(int(fd), level, opt); gerr == nil {
			prev = p
			havePrev = true
		}
		_ = unix.SetsockoptInt(int(fd), level, opt, 1)
	})
	if !havePrev {
		return func() {}
	}
	return func() {
		_ = raw.Control(func(fd uintptr) {
			_ = unix.SetsockoptInt(int(fd), level, opt, prev)
		})
	}
}
