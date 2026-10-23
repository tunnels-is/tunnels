package client

import (
	"net"
)

// translateIP rewrites ip when a NAT rule matches. The bool is true only when
// the address changed, so the packet path can leave an already-valid checksum
// alone. Walking a full TCP segment here is most of the per-packet cost.
func (t *TUN) translateIP(ip [4]byte) ([4]byte, bool) {
	t.natMu.RLock()
	mapped, ok := t.NATEgress[ip]
	t.natMu.RUnlock()
	if ok {
		return mapped, mapped != ip
	}

	if t.ServerResponse == nil || len(t.ServerResponse.Networks) == 0 {
		return ip, false
	}

	for _, v := range t.ServerResponse.Networks {
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
		var newIP [4]byte
		for i := range 4 {
			newIP[i] = net4[i]&v.NetIPNet.Mask[i] | ip[i]&^v.NetIPNet.Mask[i]
		}
		if newIP == ip {
			return ip, false
		}

		t.natMu.Lock()
		t.NATEgress[ip] = newIP
		t.NATIngress[newIP] = ip
		t.natMu.Unlock()
		return newIP, true
	}

	return ip, false
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
