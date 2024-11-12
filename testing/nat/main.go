package main

import (
	"fmt"
	"net"
)

func main() {
	ip := net.ParseIP("11.11.11.11").To4()
	ip2 := net.ParseIP("11.0.11.12").To4()
	newIP := make([]byte, 4)
	newIP2 := make([]byte, 4)
	copy(newIP, ip)
	copy(newIP2, ip2)

	_, NetIPNat, _ := net.ParseCIDR("11.0.0.11/16")
	_, NetIPNet, _ := net.ParseCIDR("12.12.12.12/32")
	if !NetIPNat.Contains(ip) {
		fmt.Println("IP1 IS NOT IN NETWORK")
	}
	if !NetIPNat.Contains(ip2) {
		fmt.Println("IP2 IS NOT IN NETWORK")
	}
	// ip, NetIPNat, _ := net.ParseCIDR("10.10.11.1/32")
	for i := range ip[:4] {
		newIP[i] = NetIPNet.IP[i]&NetIPNet.Mask[i] | ip[i]&^NetIPNet.Mask[i]
	}

	for i := range ip2[:3] {
		newIP2[i] = NetIPNet.IP[i]&NetIPNet.Mask[i] | ip2[i]&^NetIPNet.Mask[i]
	}

	fmt.Println("NEW IP:", newIP)
	fmt.Println("NEW IP2:", newIP2)
}
