//go:build linux

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
	return setDFOnConn(c, v6, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO, unix.IPPROTO_IPV6, unix.IPV6_MTU_DISCOVER, unix.IPV6_PMTUDISC_DO)
}

func setDFOnConn(c *net.UDPConn, v6 bool, v4Level, v4Opt, v4Val, v6Level, v6Opt, v6Val int) func() {
	if c == nil {
		return func() {}
	}
	raw, err := c.SyscallConn()
	if err != nil {
		return func() {}
	}
	level, opt, val := v4Level, v4Opt, v4Val
	if v6 {
		level, opt, val = v6Level, v6Opt, v6Val
	}
	var prev int
	havePrev := false
	_ = raw.Control(func(fd uintptr) {
		if p, gerr := unix.GetsockoptInt(int(fd), level, opt); gerr == nil {
			prev = p
			havePrev = true
		}
		_ = unix.SetsockoptInt(int(fd), level, opt, val)
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
