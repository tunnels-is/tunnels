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
