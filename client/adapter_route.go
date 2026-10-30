package client

import "net"

func ipv4HostOnLink(network string) bool {
	if network == "" || network == "default" {
		return false
	}
	var host net.IP
	if _, dst, err := net.ParseCIDR(network); err == nil && dst != nil {
		ones, bits := dst.Mask.Size()
		if bits != 32 || ones == 0 {
			return false
		}
		host = dst.IP.To4()
	} else {
		host = net.ParseIP(network).To4()
	}
	if host == nil {
		return false
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok || n.IP.To4() == nil {
				continue
			}
			ones, bits := n.Mask.Size()
			if bits != 32 || ones == 0 || ones >= 32 {
				continue
			}
			if n.Contains(host) {
				return true
			}
		}
	}
	return false
}

func ipv4HostFromNetwork(network string) net.IP {
	if _, dst, err := net.ParseCIDR(network); err == nil {
		ones, bits := dst.Mask.Size()
		if bits != 32 || ones != 32 {
			return nil
		}
		return dst.IP.To4()
	}
	return net.ParseIP(network).To4()
}
