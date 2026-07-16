package client

import (
	"fmt"
	"net"
	"strconv"
)

// validateRouteArgs rejects malformed route parameters before they are handed
// to external tools (route/netsh/ifconfig). The netlink-based linux path
// validates implicitly via net.ParseCIDR; the exec-based platforms share this
// check so a malicious or buggy controller cannot smuggle unexpected
// arguments into system commands. Empty strings are allowed — callers pass ""
// for parameters a given platform doesn't use.
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

// validateWGServerConfig sanity-checks the controller-provided WireGuard
// connection parameters at the source, before any of them reach interface or
// route configuration.
func validateWGServerConfig(ip, serverIP, subnet, subnet6, wanCIDR string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("controller returned an invalid WireGuard IP %q", ip)
	}
	if serverIP != "" && net.ParseIP(serverIP) == nil {
		return fmt.Errorf("controller returned an invalid server IP %q", serverIP)
	}
	for _, cidr := range []string{subnet, subnet6, wanCIDR} {
		if cidr == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("controller returned an invalid CIDR %q", cidr)
		}
	}
	return nil
}

// validateWGPort rejects a non-numeric or out-of-range WireGuard port. The port
// is interpolated into the line-oriented WireGuard IPC config
// ("endpoint=<ip>:<port>\n..."), so a value containing a newline from a
// compromised/allowlisted controller could inject extra IPC directives (peers,
// allowed-ips). Requiring a plain 1–65535 integer closes that.
func validateWGPort(port string) error {
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return fmt.Errorf("controller returned an invalid WireGuard port %q", port)
	}
	return nil
}
