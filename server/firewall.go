package main

import (
	"fmt"
	"log"
	"net"
	"runtime/debug"
)

func syncFirewallState(fr *FirewallRequest, mapping *UserCoreMapping) (errors []string) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()
	hostMap := make(map[[4]byte]bool)

	for _, v := range fr.Hosts {
		ip := net.ParseIP(v)
		if ip != nil {
			ip = ip.To4()
			hostMap[[4]byte{ip[0], ip[1], ip[2], ip[3]}] = true
			continue
		}

		ip4b, ok := getHostnameFromDHCP(v)
		if !ok {
			errors = append(errors, fmt.Sprintf("invalid host/ip (%s)", v))
			continue
		}
		hostMap[ip4b] = true
	}

	mapping.Allowedm.Lock()
	mapping.AllowedHosts = hostMap
	mapping.Allowedm.Unlock()
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
