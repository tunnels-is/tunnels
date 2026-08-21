//go:build windows

package client

import (
	"fmt"
	"net"

	wgconn "golang.zx2c4.com/wireguard/conn"
)

func applyEndpointProtect(string, net.IP) error { return nil }

func removeEndpointProtect() {}

func bindProtectToInterface(b wgconn.Bind, ifIndex uint32) error {
	if ifIndex == 0 {
		return fmt.Errorf("interface index is 0")
	}
	bsi, ok := b.(wgconn.BindSocketToInterface)
	if !ok {
		return fmt.Errorf("WireGuard bind does not support interface binding")
	}
	if err := bsi.BindSocketToInterface4(ifIndex, false); err != nil {
		return fmt.Errorf("IPv4 IP_UNICAST_IF: %w", err)
	}
	if err := bsi.BindSocketToInterface6(ifIndex, false); err != nil {
		DEBUG("IPv6 IP_UNICAST_IF skipped: ", err)
	}
	return nil
}
