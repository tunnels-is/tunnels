package main

import (
	"net"
	"runtime/debug"

	"github.com/tunnels-is/tunnels/types"
)

func syncFirewallState(fr *types.FirewallRequest, mapping *UserCoreMapping) {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()

	mapping.DisableFirewall = fr.DisableFirewall

	// Build the set of IPs from the new host list.
	newIPs := make(map[[4]byte]struct{}, len(fr.Hosts))
	for i := range fr.Hosts {
		ip4, ok := getIP4FromHostOrDHCP(fr.Hosts[i])
		if !ok {
			continue
		}
		newIPs[ip4] = struct{}{}
	}

	// Remove manual entries not present in the new list.
	mapping.ManualHosts.Range(func(ip [4]byte, _ *AllowedHost) bool {
		if _, ok := newIPs[ip]; !ok {
			mapping.ManualHosts.Delete(ip)
		}
		return true
	})

	// Add new manual entries (LoadOrStore is idempotent for existing IPs).
	for ip := range newIPs {
		ip := ip // avoid loop-variable capture
		mapping.ManualHosts.LoadOrStore(ip, &AllowedHost{IP: ip, Type: "manual"})
	}
}

func getIP4FromHostOrDHCP(host string) (ip4 [4]byte, ok bool) {
	ip := net.ParseIP(host)
	if ip != nil {
		ip = ip.To4()
		if ip != nil {
			ip4[0] = ip[0]
			ip4[1] = ip[1]
			ip4[2] = ip[2]
			ip4[3] = ip[3]
			ok = true
		} else {
			// Pure IPv6 address (not IPv4-mapped), try DHCP lookup
			ip4, ok = getHostnameFromDHCP(host)
		}
	} else {
		ip4, ok = getHostnameFromDHCP(host)
	}
	return ip4, ok
}

func getHostnameFromDHCP(hostname string) (ip4b [4]byte, ok bool) {
	for i := range clientCoreMappings {
		if clientCoreMappings[i] == nil {
			continue
		}
		if clientCoreMappings[i].DHCP == nil {
			continue
		}
		if clientCoreMappings[i].DHCP.Hostname == hostname {
			return clientCoreMappings[i].DHCP.IP, true
		}
	}
	return [4]byte{}, false
}

func validateDHCPTokenAndIP(fr *types.FirewallRequest) (mapping *UserCoreMapping) {
	ip := net.ParseIP(fr.IP)
	if ip == nil {
		return nil
	}
	ip = ip.To4()
	if ip == nil {
		return nil
	}
	ip4b := [4]byte{ip[0], ip[1], ip[2], ip[3]}

	for i := range clientCoreMappings {
		if clientCoreMappings[i] == nil {
			continue
		}
		if clientCoreMappings[i].DHCP == nil {
			continue
		}
		if clientCoreMappings[i].DHCP.Token == fr.DHCPToken {
			if clientCoreMappings[i].DHCP.IP == ip4b {
				return clientCoreMappings[i]
			}
		}
	}
	return nil
}
