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

	fmt.Println("FIREWALL REQUEST")
	fmt.Println(fr)

	originalList := make([]*AllowedHost, len(mapping.AllowedHosts))
	copy(originalList, mapping.AllowedHosts)

	for i := range originalList {
		found := false
		for ii := range fr.Hosts {
			ip4, ok := getIP4FromHostOrDHCP(fr.Hosts[ii])
			fmt.Println("FOUND HOSTNAME:", ip4, ok)
			if !ok {
				continue
			}

			fmt.Println("C1:", originalList[i].IP, originalList[i].Type)
			if ip4 == originalList[i].IP && originalList[i].Type == "manual" {
				found = true
				break
			}

		}

		fmt.Println("FOUND:", found)
		if !found {
			mapping.DelHost(originalList[i].IP, "manual")
		}
	}

	for i := range fr.Hosts {
		ip4, ok := getIP4FromHostOrDHCP(fr.Hosts[i])
		fmt.Println("FOUND HOSTNAME:", ip4, ok)
		if !ok {
			continue
		}

		found := false
		for ii := range mapping.AllowedHosts {
			fmt.Println("C2:", mapping.AllowedHosts[ii].IP, ip4)
			if ip4 == mapping.AllowedHosts[ii].IP {
				found = true
				break
			}
		}

		fmt.Println("FOUND:", found)
		if !found {
			mapping.AddHost(ip4, [2]byte{}, "manual")
		}
	}

	fmt.Println("POST MOD:", len(mapping.AllowedHosts))
	for _, v := range mapping.AllowedHosts {
		fmt.Printf("ALLOWED: %v \n", v)
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
	fmt.Println("HOST FROM DHCP")
	for i := range ClientCoreMappings {
		if ClientCoreMappings[i] == nil {
			continue
		}
		if ClientCoreMappings[i].DHCP == nil {
			continue
		}
		fmt.Println("C3:", ClientCoreMappings[i].DHCP.Hostname, hostname)
		if ClientCoreMappings[i].DHCP.Hostname == hostname {
			fmt.Println("FOUND", ClientCoreMappings[i].DHCP.IP)
			return ClientCoreMappings[i].DHCP.IP, true
		}
	}
	return [4]byte{}, false
}

func validateDHCPTokenAndIP(fr *FirewallRequest) (mapping *UserCoreMapping) {
	ip := net.ParseIP(fr.IP)
	ip = ip.To4()
	ip4b := [4]byte{ip[0], ip[1], ip[2], ip[3]}
	fmt.Println("VaLIDATE DHCP", ip, ip4b, fr.DHCPToken)

	for i := range ClientCoreMappings {
		if ClientCoreMappings[i] == nil {
			continue
		}
		if ClientCoreMappings[i].DHCP == nil {
			continue
		}
		fmt.Println("C4:", ClientCoreMappings[i].DHCP.Token, fr.DHCPToken)
		if ClientCoreMappings[i].DHCP.Token == fr.DHCPToken {
			if ClientCoreMappings[i].DHCP.IP == ip4b {
				fmt.Println("FOUND:", ClientCoreMappings[i].DHCP.IP, ip4b)
				return ClientCoreMappings[i]
			}
		}
	}
	return nil
}
