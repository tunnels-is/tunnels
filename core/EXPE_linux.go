// go build linux
package core

import (
	"fmt"
	"net"
	"runtime"

	"golang.org/x/sys/unix"
)

func closeAllOpenTCPconnections() (err error) {
	// No-op on Linux, as connections are managed by the kernel and closed automatically
	// when the process exits or the socket is closed.
	return nil
}
func setDontFragment(conn *net.UDPConn) error {
	// Get the underlying file descriptor
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("failed to get raw connection: %w", err)
	}

	var sockOptErr error
	err = rawConn.Control(func(fd uintptr) {
		// --------- Platform Specific ---------
		switch runtime.GOOS {
		case "linux":
			// IP_PMTUDISC_DO = 2: Always set DF flag. Never fragment locally.
			sockOptErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO)
		default:
			sockOptErr = fmt.Errorf("setting DF bit not supported on GOOS=%s", runtime.GOOS)
		}
		// --------- End Platform Specific ---------
	})

	if err != nil {
		return fmt.Errorf("rawconn control error: %w", err)
	}

	if sockOptErr != nil {
		return fmt.Errorf("setsockopt error: %w", sockOptErr)
	}

	return nil
}
