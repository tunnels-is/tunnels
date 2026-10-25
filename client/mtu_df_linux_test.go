//go:build linux

package client

import (
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSetUDPDontFragRestores(t *testing.T) {
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	raw, err := conn.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	get := func() int {
		t.Helper()
		var v int
		var gerr error
		if err := raw.Control(func(fd uintptr) {
			v, gerr = unix.GetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MTU_DISCOVER)
		}); err != nil {
			t.Fatal(err)
		}
		if gerr != nil {
			t.Fatal(gerr)
		}
		return v
	}
	before := get()
	restore := setUDPDontFrag(conn, false)
	if got := get(); got != unix.IP_PMTUDISC_DO {
		t.Fatalf("df during probe %d, want %d", got, unix.IP_PMTUDISC_DO)
	}
	restore()
	if got := get(); got != before {
		t.Fatalf("df after probe %d, want restored %d", got, before)
	}
}
