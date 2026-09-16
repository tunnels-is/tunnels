package client

import (
	"encoding/binary"
	"net"
)

func ipv4FragInfo(packet []byte) (isFragmented, isTrailing bool) {
	f := binary.BigEndian.Uint16(packet[6:8])
	isTrailing = f&0x1FFF != 0
	isFragmented = isTrailing || f&0x2000 != 0
	return
}

func (t *TUN) ProcessEgressPacket(p *[]byte) (sendRemote bool) {
	packet := *p

	if len(packet) < 1 {
		return false
	}

	if (packet[0] >> 4) == 6 {
		return true
	}
	if len(packet) < 20 || (packet[0]>>4) != 4 {
		return false
	}

	t.EP_Protocol = packet[9]
	if t.EP_Protocol != 17 && t.EP_Protocol != 6 {
		return false
	}

	t.EP_IPv4HeaderLength = (packet[0] & 0x0F) * 4
	if int(t.EP_IPv4HeaderLength) < 20 || int(t.EP_IPv4HeaderLength) > len(packet) {
		return false
	}
	t.EP_IPv4Header = packet[:t.EP_IPv4HeaderLength]
	t.EP_TPHeader = packet[t.EP_IPv4HeaderLength:]

	isFragmented, isTrailing := ipv4FragInfo(packet)

	if !isTrailing {
		if t.EP_Protocol == 17 && len(t.EP_TPHeader) < 8 {
			return false
		} else if t.EP_Protocol == 6 && len(t.EP_TPHeader) < 20 {
			return false
		}

		t.EP_DstPort[0] = t.EP_TPHeader[2]
		t.EP_DstPort[1] = t.EP_TPHeader[3]
		if t.blockedPortsSet[t.EP_DstPort] != 0 {
			if CONFIG.Load().LogBlockedPorts {
				INFO("PORT BLOCKED: ", t.blockedPortsSet[t.EP_DstPort])
			}
			return false
		}
	}

	t.EP_DstIP[0] = packet[16]
	t.EP_DstIP[1] = packet[17]
	t.EP_DstIP[2] = packet[18]
	t.EP_DstIP[3] = packet[19]

	if t.wgEndpointSet && t.EP_Protocol == 17 && t.EP_DstIP == t.serverInterfaceIP4bytes {
		if t.wgLoopDropLogged.CompareAndSwap(false, true) {
			SECURITY("dropping UDP to WireGuard endpoint on the tunnel interface — packets to ",
				net.IP(t.serverInterfaceIP4bytes[:]).String(),
				" must leave on the physical interface, not through the tunnel")
		}
		return false
	}

	t.EP_NAT_IP, t.EP_NAT_OK = t.translateIP(t.EP_DstIP)

	if t.EP_NAT_OK {
		t.EP_IPv4Header[16] = t.EP_NAT_IP[0]
		t.EP_IPv4Header[17] = t.EP_NAT_IP[1]
		t.EP_IPv4Header[18] = t.EP_NAT_IP[2]
		t.EP_IPv4Header[19] = t.EP_NAT_IP[3]
	}

	RecalculateIPv4HeaderChecksum(t.EP_IPv4Header)

	if !isFragmented {
		RecalculateTransportChecksum(t.EP_IPv4Header, t.EP_TPHeader)
	}

	return true
}

func (t *TUN) ProcessIngressPacket(packet []byte) bool {
	if len(packet) < 1 {
		return false
	}

	if (packet[0] >> 4) == 6 {
		return true
	}
	if len(packet) < 20 || (packet[0]>>4) != 4 {
		return false
	}

	t.IP_SrcIP[0] = packet[12]
	t.IP_SrcIP[1] = packet[13]
	t.IP_SrcIP[2] = packet[14]
	t.IP_SrcIP[3] = packet[15]

	t.IP_IPv4HeaderLength = (packet[0] & 0x0F) * 4

	if int(t.IP_IPv4HeaderLength) < 20 || int(t.IP_IPv4HeaderLength) > len(packet) {
		return false
	}
	t.IP_IPv4Header = packet[:t.IP_IPv4HeaderLength]
	t.IP_TPHeader = packet[t.IP_IPv4HeaderLength:]

	isFragmented, isTrailing := ipv4FragInfo(packet)

	proto := packet[9]
	if !isTrailing {
		if proto == 17 && len(t.IP_TPHeader) < 8 {
			return false
		} else if proto == 6 && len(t.IP_TPHeader) < 20 {
			return false
		}
	}

	t.natMu.RLock()
	t.IP_NAT_IP, t.IP_NAT_OK = t.NATIngress[t.IP_SrcIP]
	t.natMu.RUnlock()
	if t.IP_NAT_OK {
		t.IP_IPv4Header[12] = t.IP_NAT_IP[0]
		t.IP_IPv4Header[13] = t.IP_NAT_IP[1]
		t.IP_IPv4Header[14] = t.IP_NAT_IP[2]
		t.IP_IPv4Header[15] = t.IP_NAT_IP[3]
	}

	RecalculateIPv4HeaderChecksum(t.IP_IPv4Header)
	if !isFragmented {
		RecalculateTransportChecksum(t.IP_IPv4Header, t.IP_TPHeader)
	}

	return true
}

func RecalculateIPv4HeaderChecksum(bytes []byte) {
	bytes[10] = 0
	bytes[11] = 0

	var csum uint32

	for i := 0; i < len(bytes)-1; i += 2 {
		csum += uint32(bytes[i])<<8 | uint32(bytes[i+1])
	}

	for csum > 0xFFFF {
		csum = (csum >> 16) + (csum & 0xFFFF)
	}

	bytes[10] = byte(^csum >> 8)
	bytes[11] = byte(^csum & 0xFF)
}

func RecalculateTransportChecksum(IPv4Header []byte, TPPacket []byte) {
	switch IPv4Header[9] {
	case 6:
		TPPacket[16] = 0
		TPPacket[17] = 0
	case 17:
		TPPacket[6] = 0
		TPPacket[7] = 0
	}

	var csum uint32
	csum += (uint32(IPv4Header[12]) + uint32(IPv4Header[14])) << 8
	csum += uint32(IPv4Header[13]) + uint32(IPv4Header[15])
	csum += (uint32(IPv4Header[16]) + uint32(IPv4Header[18])) << 8
	csum += uint32(IPv4Header[17]) + uint32(IPv4Header[19])
	csum += uint32(IPv4Header[9])
	tcpLength := uint32(len(TPPacket))

	csum += tcpLength & 0xffff
	csum += tcpLength >> 16

	length := len(TPPacket) - 1
	for i := 0; i < length; i += 2 {
		csum += uint32(TPPacket[i]) << 8
		csum += uint32(TPPacket[i+1])
	}
	if len(TPPacket)%2 == 1 {
		csum += uint32(TPPacket[length]) << 8
	}
	for csum > 0xffff {
		csum = (csum >> 16) + (csum & 0xffff)
	}

	switch IPv4Header[9] {
	case 6:
		binary.BigEndian.PutUint16(TPPacket[16:18], ^uint16(csum))
	case 17:

		udpsum := ^uint16(csum)
		if udpsum == 0 {
			udpsum = 0xFFFF
		}
		binary.BigEndian.PutUint16(TPPacket[6:8], udpsum)
	}
}
