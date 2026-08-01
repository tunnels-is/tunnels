package client

import (
	"net"
)

func (V *TUN) TransLateIP(ip [4]byte) ([4]byte, bool) {
	V.natMu.RLock()
	xxx, ok := V.NATEgress[ip]
	V.natMu.RUnlock()
	if ok {
		return xxx, true
	}

	if len(V.ServerResponse.Networks) == 0 {
		return ip, true
	}

	var newIP [4]byte
	matched := false
	for _, v := range V.ServerResponse.Networks {
		if v.Nat == "" {
			continue
		}

		if !v.NatIPNet.Contains(net.IP(ip[:])) {
			continue
		}

		net4 := v.NetIPNet.IP.To4()
		if net4 == nil || len(v.NetIPNet.Mask) != 4 {
			continue
		}
		for i := range 4 {
			newIP[i] = net4[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
		}

		V.natMu.Lock()
		V.NATEgress[ip] = newIP
		V.NATIngress[newIP] = ip
		V.natMu.Unlock()
		matched = true
		break
	}

	if !matched {
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
