package core

import (
	"net"
	"strings"
)

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (V *Tunnel) TransLateIP(ip [4]byte) ([4]byte, bool) {
	originalIP := (net.IP)(ip[:])
	xxx, ok := V.NAT_CACHE[[4]byte{originalIP[0], originalIP[1], originalIP[2], originalIP[3]}]
	if ok {
		return xxx, true
	}

	newIP := make([]byte, len(originalIP))
	copy(newIP, originalIP)
	for _, v := range V.CRR.Networks {
		if v.Nat == "" {
			continue
		}

		if !v.NatIPNet.Contains(originalIP) {
			continue
		}

		if strings.HasSuffix(v.Network, "/32") {
			for i := range ip[:4] {
				newIP[i] = v.NetIPNet.IP[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
			}
		} else {
			for i := range ip[:3] {
				newIP[i] = v.NetIPNet.IP[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
			}
		}

		V.NAT_CACHE[[4]byte{originalIP[0], originalIP[1], originalIP[2], originalIP[3]}] = [4]byte{newIP[0], newIP[1], newIP[2], newIP[3]}

		V.REVERSE_NAT_CACHE[[4]byte{newIP[0], newIP[1], newIP[2], newIP[3]}] = [4]byte{originalIP[0], originalIP[1], originalIP[2], originalIP[3]}
		break
	}

	return [4]byte{newIP[0], newIP[1], newIP[2], newIP[3]}, true
}

func (V *Tunnel) InitNatMaps() (err error) {
	DEBUG("Initializing NAT maps for tunnel:", V.Meta.IFName)
	for _, v := range V.CRR.Networks {
		if v.Nat == "" {
			continue
		}
		_, v.NatIPNet, err = net.ParseCIDR(v.Nat)
		if err != nil {
			return err
		}

		_, v.NetIPNet, err = net.ParseCIDR(v.Network)
		if err != nil {
			return err
		}
	}
	V.NAT_CACHE = make(map[[4]byte][4]byte)
	V.REVERSE_NAT_CACHE = make(map[[4]byte][4]byte)
	return nil
}

func (V *Tunnel) BuildNATMap() (err error) {
	if V.CRR.Networks == nil {
		DEBUG("no NAT map found")
		return
	}

	V.NAT_CACHE = make(map[[4]byte][4]byte)
	V.REVERSE_NAT_CACHE = make(map[[4]byte][4]byte)

	for _, v := range V.CRR.Networks {
		if v.Nat == "" {
			continue
		}
		if v.Network == "" {
			continue
		}
		ip2, ip2net, err := net.ParseCIDR(v.Nat)
		if err != nil {
			return err
		}
		v.NatIPNet = ip2net
		ip, ipnet, err := net.ParseCIDR(v.Network)
		if err != nil {
			return err
		}
		v.NetIPNet = ipnet

		ip = ip.Mask(ipnet.Mask)
		ip2 = ip2.Mask(ip2net.Mask)

		for ipnet.Contains(ip) && ip2net.Contains(ip2) {

			V.NAT_CACHE[[4]byte{ip2[0], ip2[1], ip2[2], ip2[3]}] = [4]byte{ip[0], ip[1], ip[2], ip[3]}
			V.REVERSE_NAT_CACHE[[4]byte{ip[0], ip[1], ip[2], ip[3]}] = [4]byte{ip2[0], ip2[1], ip2[2], ip2[3]}

			inc(ip)
			inc(ip2)
		}

	}
	return
}
