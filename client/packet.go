package client

import (
	"encoding/binary"
)

// ipv4IsTrailingFragment reports whether an IPv4 packet is a non-first fragment
// (fragment offset > 0), i.e. it carries no L4 header. isFragmented reports
// whether the packet is part of a fragmented datagram at all (MF set or a
// non-zero offset). frag lives in bytes 6-7: top 3 bits are flags (0x2000=MF),
// low 13 bits are the offset in 8-byte units.
func ipv4FragInfo(packet []byte) (isFragmented, isTrailing bool) {
	f := binary.BigEndian.Uint16(packet[6:8])
	isTrailing = f&0x1FFF != 0
	isFragmented = isTrailing || f&0x2000 != 0
	return
}

func (V *TUN) ProcessEgressPacket(p *[]byte) (sendRemote bool) {
	packet := *p

	if len(packet) < 1 {
		return false
	}
	// IPv6 has no NAT/port-control here and no header checksum; pass it through
	// untouched so dual-stack tunnels are not black-holed.
	if (packet[0] >> 4) == 6 {
		return true
	}
	if len(packet) < 20 || (packet[0]>>4) != 4 {
		return false
	}

	V.EP_Protocol = packet[9]
	if V.EP_Protocol != 17 && V.EP_Protocol != 6 {
		return false
	}

	V.EP_IPv4HeaderLength = (packet[0] & 0x0F) * 4
	if int(V.EP_IPv4HeaderLength) < 20 || int(V.EP_IPv4HeaderLength) > len(packet) {
		return false
	}
	V.EP_IPv4Header = packet[:V.EP_IPv4HeaderLength]
	V.EP_TPHeader = packet[V.EP_IPv4HeaderLength:]

	isFragmented, isTrailing := ipv4FragInfo(packet)

	// The L4 header (ports, checksum) only exists on the first/only fragment.
	// A trailing fragment's "transport bytes" are raw payload — never read a
	// port from them or overwrite them with a checksum.
	if !isTrailing {
		if V.EP_Protocol == 17 && len(V.EP_TPHeader) < 8 {
			return false
		} else if V.EP_Protocol == 6 && len(V.EP_TPHeader) < 20 {
			return false
		}

		V.EP_DstPort[0] = V.EP_TPHeader[2]
		V.EP_DstPort[1] = V.EP_TPHeader[3]
		if V.blockedPortsSet[V.EP_DstPort] != 0 {
			if CONFIG.Load().LogBlockedPorts {
				INFO("PORT BLOCKED: ", V.blockedPortsSet[V.EP_DstPort])
			}
			return false
		}
	}

	V.EP_DstIP[0] = packet[16]
	V.EP_DstIP[1] = packet[17]
	V.EP_DstIP[2] = packet[18]
	V.EP_DstIP[3] = packet[19]

	V.EP_NAT_IP, V.EP_NAT_OK = V.TransLateIP(V.EP_DstIP)

	if V.EP_NAT_OK {
		V.EP_IPv4Header[16] = V.EP_NAT_IP[0]
		V.EP_IPv4Header[17] = V.EP_NAT_IP[1]
		V.EP_IPv4Header[18] = V.EP_NAT_IP[2]
		V.EP_IPv4Header[19] = V.EP_NAT_IP[3]
	}

	RecalculateIPv4HeaderChecksum(V.EP_IPv4Header)
	// The transport checksum covers the whole (reassembled) datagram, so it can
	// only be recomputed from a complete, unfragmented packet. Fragments already
	// carry a software-computed L4 checksum (offload can't span fragments), so
	// leaving it intact is correct for the non-NAT case. (NAT of a fragmented
	// datagram would need reassembly/incremental fixup and is unsupported.)
	if !isFragmented {
		RecalculateTransportChecksum(V.EP_IPv4Header, V.EP_TPHeader)
	}

	return true
}

func (V *TUN) ProcessIngressPacket(packet []byte) bool {
	if len(packet) < 1 {
		return false
	}
	// IPv6: pass through untouched (see egress note).
	if (packet[0] >> 4) == 6 {
		return true
	}
	if len(packet) < 20 || (packet[0]>>4) != 4 {
		return false
	}

	V.IP_SrcIP[0] = packet[12]
	V.IP_SrcIP[1] = packet[13]
	V.IP_SrcIP[2] = packet[14]
	V.IP_SrcIP[3] = packet[15]

	V.IP_IPv4HeaderLength = (packet[0] & 0x0F) * 4
	// A header shorter than the minimum 20 bytes would make the checksum
	// recalculation index past a too-short slice and panic. A hostile/
	// compromised server can send exactly this, so reject it here.
	if int(V.IP_IPv4HeaderLength) < 20 || int(V.IP_IPv4HeaderLength) > len(packet) {
		return false
	}
	V.IP_IPv4Header = packet[:V.IP_IPv4HeaderLength]
	V.IP_TPHeader = packet[V.IP_IPv4HeaderLength:]

	isFragmented, isTrailing := ipv4FragInfo(packet)

	proto := packet[9]
	if !isTrailing {
		if proto == 17 && len(V.IP_TPHeader) < 8 {
			return false
		} else if proto == 6 && len(V.IP_TPHeader) < 20 {
			return false
		}
	}

	V.natMu.RLock()
	V.IP_NAT_IP, V.IP_NAT_OK = V.NATIngress[V.IP_SrcIP]
	V.natMu.RUnlock()
	if V.IP_NAT_OK {
		V.IP_IPv4Header[12] = V.IP_NAT_IP[0]
		V.IP_IPv4Header[13] = V.IP_NAT_IP[1]
		V.IP_IPv4Header[14] = V.IP_NAT_IP[2]
		V.IP_IPv4Header[15] = V.IP_NAT_IP[3]
	}

	RecalculateIPv4HeaderChecksum(V.IP_IPv4Header)
	if !isFragmented {
		RecalculateTransportChecksum(V.IP_IPv4Header, V.IP_TPHeader)
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
		// RFC 768: a computed UDP checksum of zero must be sent as 0xFFFF, since
		// 0x0000 in the field means "no checksum" and the receiver would skip
		// validation. (TCP has no such rule — 0 is a legal TCP checksum.)
		udpsum := ^uint16(csum)
		if udpsum == 0 {
			udpsum = 0xFFFF
		}
		binary.BigEndian.PutUint16(TPPacket[6:8], udpsum)
	}
}
