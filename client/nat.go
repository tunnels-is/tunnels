package client

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

func (t *TUN) TransLateVPLIP(ip [4]byte) ([4]byte, bool) {
	if xxx, ok := t.NATEgress[ip]; ok {
		return xxx, true
	}

	if t.ServerResponse.LAN == nil {
		return ip, true
	}

	v := t.ServerResponse.LAN
	var newIP [4]byte

	for i := range 3 {
		newIP[i] = v.NetIPNet.IP[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
	}
	newIP[3] = ip[3]

	t.NATEgress[ip] = newIP
	t.NATIngress[newIP] = ip

	return newIP, true
}

func (V *TUN) TransLateIP(ip [4]byte) ([4]byte, bool) {
	if xxx, ok := V.NATEgress[ip]; ok {
		return xxx, true
	}

	if len(V.ServerResponse.Networks) == 0 {
		return ip, true
	}

	var newIP [4]byte
	for _, v := range V.ServerResponse.Networks {
		if v.Nat == "" {
			continue
		}

		if !v.NatIPNet.Contains(net.IP(ip[:])) {
			continue
		}

		if strings.HasSuffix(v.Network, "/32") {
			for i := range 4 {
				newIP[i] = v.NetIPNet.IP[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
			}
		} else {
			for i := range 3 {
				newIP[i] = v.NetIPNet.IP[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
			}
			newIP[3] = ip[3]
		}

		V.NATEgress[ip] = newIP
		V.NATIngress[newIP] = ip
		break
	}

	if newIP == [4]byte{0, 0, 0, 0} {
		newIP = ip
	}

	return newIP, true
}

func (V *TUN) IsEgressVPLIP(ip [4]byte) (ok bool) {
	_, ok = V.VPLIngress[ip]
	return
}

func (V *TUN) IsIngressVPLIP(ip [4]byte) (ok bool) {
	_, ok = V.VPLIngress[ip]
	return
}

func (t *TUN) InitVPLMap() (err error) {
	meta := t.meta.Load()
	DEBUG("Initializing VPL/NAT maps for tunnel:", meta.IFName)
	if t.ServerResponse.LAN == nil {
		return nil
	}

	if t.ServerResponse.LAN.Nat != "" {
		_, t.ServerResponse.LAN.NatIPNet, err = net.ParseCIDR(t.ServerResponse.LAN.Nat)
		if err != nil {
			return err
		}
	}

	_, t.ServerResponse.LAN.NetIPNet, err = net.ParseCIDR(t.ServerResponse.LAN.Network)
	if err != nil {
		return err
	}

	toMap := ""
	if t.ServerResponse.LAN.Nat != "" {
		toMap = t.ServerResponse.LAN.Nat
	} else {
		toMap = t.ServerResponse.LAN.Network
	}

	ip, network, err := net.ParseCIDR(toMap)
	if err != nil {
		return err
	}

	ip = ip.Mask(network.Mask)

	t.VPLEgress = make(map[[4]byte]struct{})
	t.VPLIngress = make(map[[4]byte]struct{})
	for network.Contains(ip) {
		t.VPLEgress[[4]byte{ip[0], ip[1], ip[2], ip[3]}] = struct{}{}
		t.VPLIngress[[4]byte{ip[0], ip[1], ip[2], ip[3]}] = struct{}{}
		inc(ip)
	}

	return nil
}

func (t *TUN) InitNatMaps() (err error) {
	meta := t.meta.Load()
	DEBUG("Initializing NAT maps for tunnel:", meta.IFName)
	for _, v := range t.ServerResponse.Networks {
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
	t.NATEgress = make(map[[4]byte][4]byte)
	t.NATIngress = make(map[[4]byte][4]byte)
	return nil
}

// func (V *Tunnel) BuildNATMap() (err error) {
// 	if V.crReponse.Networks == nil {
// 		DEBUG("no NAT map found")
// 		return
// 	}
//
// 	V.NAT_CACHE = make(map[[4]byte][4]byte)
// 	V.REVERSE_NAT_CACHE = make(map[[4]byte][4]byte)
//
// 	for _, v := range V.CRR.Networks {
// 		if v.Nat == "" {
// 			continue
// 		}
// 		if v.Network == "" {
// 			continue
// 		}
// 		ip2, ip2net, err := net.ParseCIDR(v.Nat)
// 		if err != nil {
// 			return err
// 		}
// 		v.NatIPNet = ip2net
// 		ip, ipnet, err := net.ParseCIDR(v.Network)
// 		if err != nil {
// 			return err
// 		}
// 		v.NetIPNet = ipnet
//
// 		ip = ip.Mask(ipnet.Mask)
// 		ip2 = ip2.Mask(ip2net.Mask)
//
// 		for ipnet.Contains(ip) && ip2net.Contains(ip2) {
//
// 			V.NAT_CACHE[[4]byte{ip2[0], ip2[1], ip2[2], ip2[3]}] = [4]byte{ip[0], ip[1], ip[2], ip[3]}
// 			V.REVERSE_NAT_CACHE[[4]byte{ip[0], ip[1], ip[2], ip[3]}] = [4]byte{ip2[0], ip2[1], ip2[2], ip2[3]}
//
// 			inc(ip)
// 			inc(ip2)
// 		}
//
// 	}
// 	return
// }
