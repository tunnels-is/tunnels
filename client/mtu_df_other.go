//go:build !linux && !darwin && !windows

package client

import "net"

func isMsgTooBig(error) bool { return false }

func setUDPDontFrag(c *net.UDPConn, v6 bool) func() {
	return func() {}
}
