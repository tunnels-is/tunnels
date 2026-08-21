//go:build !linux && !darwin && !windows

package client

import (
	"net"

	wgconn "golang.zx2c4.com/wireguard/conn"
)

func applyEndpointProtect(ifName string, gw net.IP) error {
	return nil
}

func removeEndpointProtect() {}

func bindProtectToInterface(_ wgconn.Bind, _ uint32) error {
	return nil
}
