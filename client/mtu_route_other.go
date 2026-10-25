//go:build !linux && !darwin && !windows

package client

import "net"

func addPeerMTURoute(string, string, int) error { return nil }

func delPeerMTURoute(string, string) error { return nil }

func egressMTU(net.IP) int { return 0 }
