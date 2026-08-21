//go:build !linux && !darwin && !windows

package client

import (
	"fmt"
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

func runProtectWatcher() {}

func physicalIPv4Default() (string, net.IP, int, error) {
	return "", nil, 0, fmt.Errorf("physical default discovery is not supported on this platform")
}

func protectRoutesPresent(string, net.IP, int, []string) bool { return false }
