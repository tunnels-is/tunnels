package client

import (
	"fmt"
	"net"
	"strconv"

	"github.com/tunnels-is/tunnels/types"
)

func validateRouteArgs(network, gateway, metric string) error {
	if network != "" && network != "default" {
		if _, _, err := net.ParseCIDR(network); err != nil {
			if net.ParseIP(network) == nil {
				return fmt.Errorf("invalid route network %q", network)
			}
		}
	}
	if gateway != "" && net.ParseIP(gateway) == nil {
		return fmt.Errorf("invalid route gateway %q", gateway)
	}
	if metric != "" {
		if _, err := strconv.Atoi(metric); err != nil {
			return fmt.Errorf("invalid route metric %q", metric)
		}
	}
	return nil
}

func validateWGServerConfig(ip, serverIP, subnet, subnet6, wanCIDR string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("controller returned an invalid WireGuard IP %q", ip)
	}
	if serverIP != "" && net.ParseIP(serverIP) == nil {
		return fmt.Errorf("controller returned an invalid server IP %q", serverIP)
	}
	for _, cidr := range []string{subnet, subnet6} {
		if cidr == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("controller returned an invalid CIDR %q", cidr)
		}
	}
	if err := types.ValidateWANCIDR(wanCIDR); err != nil {
		return fmt.Errorf("controller returned an invalid WAN CIDR: %w", err)
	}
	return nil
}

func validateWGPort(port string) error {
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return fmt.Errorf("controller returned an invalid WireGuard port %q", port)
	}
	return nil
}
