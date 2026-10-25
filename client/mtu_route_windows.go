//go:build windows

package client

import "net"

// Windows has no per-route MTU. TCP is clamped in ProcessEgressPacket and
// UDP larger than the peer MTU is dropped there.
func addPeerMTURoute(string, string, int) error { return nil }

func delPeerMTURoute(string, string) error { return nil }

func egressMTU(net.IP) int { return 0 }
