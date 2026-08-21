package types

import (
	"fmt"
	"net"
)

const minWANIPv4Prefix = 8

// ValidateWANCIDR accepts an empty string, or an IPv4 prefix of /8 or
// more specific. IPv6 WANs are rejected (IPv6 is not tunneled).
func ValidateWANCIDR(cidr string) error {
	if cidr == "" {
		return nil
	}
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q", cidr)
	}
	if ip.To4() == nil {
		return fmt.Errorf("IPv6 WAN is not supported")
	}
	ones, bits := ipNet.Mask.Size()
	if bits != 32 || ones < minWANIPv4Prefix {
		return fmt.Errorf("IPv4 WAN prefix must be /%d or more specific, got /%d", minWANIPv4Prefix, ones)
	}
	return nil
}
