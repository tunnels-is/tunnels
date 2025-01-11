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

func (V *Tunnel) TransLateVPLIP(ip [4]byte) ([4]byte, bool) {
	originalIP := (net.IP)(ip[:])
	xxx, ok := V.NAT_CACHE[[4]byte{originalIP[0], originalIP[1], originalIP[2], originalIP[3]}]
	if ok {
		return xxx, true
	}
	newIP := make([]byte, len(originalIP))
	copy(newIP, originalIP)
	if V.CRR.VPLNetwork == nil {
		return [4]byte{newIP[0], newIP[1], newIP[2], newIP[3]}, true
	}

	v := V.CRR.VPLNetwork

	for i := range ip[:3] {
		newIP[i] = v.NetIPNet.IP[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
	}
	// return
	V.NAT_CACHE[[4]byte{originalIP[0], originalIP[1], originalIP[2], originalIP[3]}] = [4]byte{newIP[0], newIP[1], newIP[2], newIP[3]}

	V.REVERSE_NAT_CACHE[[4]byte{newIP[0], newIP[1], newIP[2], newIP[3]}] = [4]byte{originalIP[0], originalIP[1], originalIP[2], originalIP[3]}
	return [4]byte{newIP[0], newIP[1], newIP[2], newIP[3]}, true
}

func (V *Tunnel) TransLateIP(ip [4]byte) ([4]byte, bool) {
	if xxx, ok := V.NAT_CACHE[ip]; ok {
		return xxx, true
	}

	if len(V.CRR.Networks) == 0 {
		return ip, true
	}

	var newIP [4]byte
	for _, v := range V.CRR.Networks {
		if v.Nat == "" {
			continue
		}

		if !v.NatIPNet.Contains(net.IP(ip[:])) {
			continue
		}

		if strings.HasSuffix(v.Network, "/32") {
			for i := 0; i < 4; i++ {
				newIP[i] = v.NetIPNet.IP[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
			}
		} else {
			for i := 0; i < 3; i++ {
				newIP[i] = v.NetIPNet.IP[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
			}
			newIP[3] = ip[3]
		}

		V.NAT_CACHE[ip] = newIP
		V.REVERSE_NAT_CACHE[newIP] = ip
		break
	}

	if newIP == [4]byte{0, 0, 0, 0} {
		newIP = ip
	}

	return newIP, true
}

func (V *Tunnel) IsEgressVPLIP(ip [4]byte) (ok bool) {
	_, ok = V.VPL_E_MAP[ip]
	return
}

func (V *Tunnel) IsIngressVPLIP(ip [4]byte) (ok bool) {
	_, ok = V.VPL_I_MAP[ip]
	return
}

func (V *Tunnel) InitVPLMap() (err error) {
	DEBUG("Initializing VPL/NAT maps for tunnel:", V.Meta.IFName)
	if V.CRR.VPLNetwork == nil {
		return nil
	}

	if V.CRR.VPLNetwork.Nat != "" {
		_, V.CRR.VPLNetwork.NatIPNet, err = net.ParseCIDR(V.CRR.VPLNetwork.Nat)
		if err != nil {
			return err
		}
	}

	_, V.CRR.VPLNetwork.NetIPNet, err = net.ParseCIDR(V.CRR.VPLNetwork.Network)
	if err != nil {
		return err
	}

	toMap := ""
	if V.CRR.VPLNetwork.Nat != "" {
		toMap = V.CRR.VPLNetwork.Nat
	} else {
		toMap = V.CRR.VPLNetwork.Network
	}

	ip, network, err := net.ParseCIDR(toMap)
	if err != nil {
		return err
	}

	ip = ip.Mask(network.Mask)

	V.VPL_E_MAP = make(map[[4]byte]struct{})
	V.VPL_I_MAP = make(map[[4]byte]struct{})
	for network.Contains(ip) {
		V.VPL_E_MAP[[4]byte{ip[0], ip[1], ip[2], ip[3]}] = struct{}{}
		V.VPL_I_MAP[[4]byte{ip[0], ip[1], ip[2], ip[3]}] = struct{}{}
		inc(ip)
	}

	return nil
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
