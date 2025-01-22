package main

import (
	"net"
	"runtime/debug"
)

func syncFirewallState(fr *FirewallRequest, mapping *UserCoreMapping) (errors []string) {
	defer func() {
		r := recover()
		if r != nil {
			ERR(r, string(debug.Stack()))
		}
	}()

	originalList := make([]*AllowedHost, len(mapping.AllowedHosts))
	copy(originalList, mapping.AllowedHosts)

	mapping.DisableFirewall = fr.DisableFirewall

	for i := range originalList {
		found := false
		for ii := range fr.Hosts {
			ip4, ok := getIP4FromHostOrDHCP(fr.Hosts[ii])
			if !ok {
				continue
			}

			if ip4 == originalList[i].IP && originalList[i].Type == "manual" {
				found = true
				break
			}

		}

		if !found {
			mapping.DelHost(originalList[i].IP, "manual")
		}
	}

	for i := range fr.Hosts {
		ip4, ok := getIP4FromHostOrDHCP(fr.Hosts[i])
		if !ok {
			continue
		}

		found := false
		for ii := range mapping.AllowedHosts {
			if ip4 == originalList[ii].IP && originalList[ii].Type == "manual" {
				found = true
				break
			}
		}

		if !found {
			mapping.AddHost(ip4, [2]byte{}, "manual")
		}
	}

	return
}

func getIP4FromHostOrDHCP(host string) (ip4 [4]byte, ok bool) {
	ip := net.ParseIP(host)
	if ip != nil {
		ip = ip.To4()
		ip4[0] = ip[0]
		ip4[1] = ip[1]
		ip4[2] = ip[2]
		ip4[3] = ip[3]
		ok = true
	} else {
		ip4, ok = getHostnameFromDHCP(host)
	}
	return
}

func getHostnameFromDHCP(hostname string) (ip4b [4]byte, ok bool) {
	for i := range ClientCoreMappings {
		if ClientCoreMappings[i] == nil {
			continue
		}
		if ClientCoreMappings[i].DHCP == nil {
			continue
		}
		if ClientCoreMappings[i].DHCP.Hostname == hostname {
			return ClientCoreMappings[i].DHCP.IP, true
		}
	}
	return [4]byte{}, false
}

func validateDHCPTokenAndIP(fr *FirewallRequest) (mapping *UserCoreMapping) {
	ip := net.ParseIP(fr.IP)
	ip = ip.To4()
	ip4b := [4]byte{ip[0], ip[1], ip[2], ip[3]}

	for i := range ClientCoreMappings {
		if ClientCoreMappings[i] == nil {
			continue
		}
		if ClientCoreMappings[i].DHCP == nil {
			continue
		}
		if ClientCoreMappings[i].DHCP.Token == fr.DHCPToken {
			if ClientCoreMappings[i].DHCP.IP == ip4b {
				return ClientCoreMappings[i]
			}
		}
	}
	return nil
}
