package client

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

func (t *TUN) RegisterPing(tag string, packet []byte) {
	defer RecoverAndLog()
	if len(packet) > 18 {
		if !bytes.Equal(packet[3:11], []byte{255, 1, 255, 2, 255, 3, 255, 4}) {
			DEEP("Short packet received, not a ping packet")
			return
		}

		t.CPU = packet[0]
		t.MEM = packet[1]
		t.DISK = packet[2]
		count := int64(binary.BigEndian.Uint64(packet[11:]))

		DEEP(fmt.Sprintf("ping from server (%s) cpu(%d) mem(%d) disk(%d)  count(%d)", tag, t.CPU, t.MEM, t.DISK, t.PingInt.Load()))

		localCount := t.PingInt.Load()
		if localCount > (count + 10) {
			ERROR("Ping count of of balance local:", localCount, "server:", count, "server_max:", count+10)
			t.needsReconnect.Store(true)
		} else {
			t.registerPing(time.Now())
		}
	}
}

func (V *TUN) ProcessEgressPacket(p *[]byte) (sendRemote bool) {
	packet := *p

	if (packet[0] >> 4) != 4 {
		return false
	}

	V.EP_Protocol = packet[9]
	if V.EP_Protocol != 17 && V.EP_Protocol != 6 {
		return false
	}

	V.EP_IPv4HeaderLength = (packet[0] & 0x0F) * 4
	V.EP_IPv4Header = packet[:V.EP_IPv4HeaderLength]
	V.EP_TPHeader = packet[V.EP_IPv4HeaderLength:]

	if V.EP_Protocol == 17 && len(V.EP_TPHeader) < 8 {
		return false
	} else if V.EP_Protocol == 6 && len(V.EP_TPHeader) < 20 {
		return false
	}

	V.EP_DstIP[0] = packet[16]
	V.EP_DstIP[1] = packet[17]
	V.EP_DstIP[2] = packet[18]
	V.EP_DstIP[3] = packet[19]

	V.EP_SrcPort[0] = V.EP_TPHeader[0]
	V.EP_SrcPort[1] = V.EP_TPHeader[1]
	V.EP_DstPort[0] = V.EP_TPHeader[2]
	V.EP_DstPort[1] = V.EP_TPHeader[3]

	// TODO .. IMPLEMENT PORT BLOCKING

	if !V.IsEgressVPLIP(V.EP_DstIP) {

		V.EgressMapping = V.CreateNEWPortMapping(p)
		if V.EgressMapping == nil {
			return false
		}
		if V.EP_Protocol == 6 {
			if V.EP_TPHeader[13]&0x1 > 0 {
				V.EgressMapping.finCount.Add(1)
			}

			if V.EP_TPHeader[13]&0x4 == 4 {
				V.EP_TPHeader[13] = 0b00010100
				V.EgressMapping.rstFound.Store(true)
			} else if V.EP_TPHeader[13]&0x4 > 0 {
				V.EgressMapping.rstFound.Store(true)
			}
		}

		V.EP_NAT_IP, V.EP_NAT_OK = V.TransLateIP(V.EP_DstIP)

		V.EP_TPHeader[0] = V.EgressMapping.MappedPort[0]
		V.EP_TPHeader[1] = V.EgressMapping.MappedPort[1]

		V.EP_IPv4Header[12] = V.serverInterfaceIP4bytes[0]
		V.EP_IPv4Header[13] = V.serverInterfaceIP4bytes[1]
		V.EP_IPv4Header[14] = V.serverInterfaceIP4bytes[2]
		V.EP_IPv4Header[15] = V.serverInterfaceIP4bytes[3]

	} else {
		V.EP_NAT_IP, V.EP_NAT_OK = V.TransLateVPLIP(V.EP_DstIP)

		V.EP_IPv4Header[12] = V.serverVPLIP[0]
		V.EP_IPv4Header[13] = V.serverVPLIP[1]
		V.EP_IPv4Header[14] = V.serverVPLIP[2]
		V.EP_IPv4Header[15] = V.serverVPLIP[3]
	}

	if V.EP_NAT_OK {
		V.EP_IPv4Header[16] = V.EP_NAT_IP[0]
		V.EP_IPv4Header[17] = V.EP_NAT_IP[1]
		V.EP_IPv4Header[18] = V.EP_NAT_IP[2]
		V.EP_IPv4Header[19] = V.EP_NAT_IP[3]
	}

	RecalculateIPv4HeaderChecksum(V.EP_IPv4Header)
	RecalculateTransportChecksum(V.EP_IPv4Header, V.EP_TPHeader)

	return true
}

func (V *TUN) ProcessIngressPacket(packet []byte) bool {
	V.IP_SrcIP[0] = packet[12]
	V.IP_SrcIP[1] = packet[13]
	V.IP_SrcIP[2] = packet[14]
	V.IP_SrcIP[3] = packet[15]

	V.IP_Protocol = packet[9]

	V.IP_IPv4HeaderLength = (packet[0] << 4 >> 4) * 4
	V.IP_IPv4Header = packet[:V.IP_IPv4HeaderLength]
	V.IP_TPHeader = packet[V.IP_IPv4HeaderLength:]

	V.IP_SrcPort[0] = V.IP_TPHeader[0]
	V.IP_SrcPort[1] = V.IP_TPHeader[1]
	V.IP_DstPort[0] = V.IP_TPHeader[2]
	V.IP_DstPort[1] = V.IP_TPHeader[3]

	if !V.IsIngressVPLIP(V.IP_SrcIP) {
		V.IP_NAT_IP, V.IP_NAT_OK = V.NATIngress[V.IP_SrcIP]
		if V.IP_NAT_OK {
			V.IP_IPv4Header[12] = V.IP_NAT_IP[0]
			V.IP_IPv4Header[13] = V.IP_NAT_IP[1]
			V.IP_IPv4Header[14] = V.IP_NAT_IP[2]
			V.IP_IPv4Header[15] = V.IP_NAT_IP[3]

			V.IP_SrcIP[0] = V.IP_NAT_IP[0]
			V.IP_SrcIP[1] = V.IP_NAT_IP[1]
			V.IP_SrcIP[2] = V.IP_NAT_IP[2]
			V.IP_SrcIP[3] = V.IP_NAT_IP[3]
		}

		// x := time.Now()
		V.IngressMapping = V.getIngressPortMapping()
		if V.IngressMapping == nil {
			return false
		}

		if V.IP_Protocol == 6 {
			if V.IP_TPHeader[13]&0x4 > 0 {
				V.IngressMapping.rstFound.Store(true)
			}

			if V.IP_TPHeader[13]&0x1 > 0 {
				V.IngressMapping.finCount.Add(1)
			}
		}

		V.IP_TPHeader[2] = V.IngressMapping.SrcPort[0]
		V.IP_TPHeader[3] = V.IngressMapping.SrcPort[1]

		V.IP_IPv4Header[16] = V.IngressMapping.OriginalSourceIP[0]
		V.IP_IPv4Header[17] = V.IngressMapping.OriginalSourceIP[1]
		V.IP_IPv4Header[18] = V.IngressMapping.OriginalSourceIP[2]
		V.IP_IPv4Header[19] = V.IngressMapping.OriginalSourceIP[3]

	} else {
		// if DST == ME ON VPL .. then DST == 127.0.0.1
		// V.IP_IPv4Header[16] = 127
		// V.IP_IPv4Header[17] = 0
		// V.IP_IPv4Header[18] = 0
		// V.IP_IPv4Header[19] = 1
		V.IP_IPv4Header[16] = V.localInterfaceIP4bytes[0]
		V.IP_IPv4Header[17] = V.localInterfaceIP4bytes[1]
		V.IP_IPv4Header[18] = V.localInterfaceIP4bytes[2]
		V.IP_IPv4Header[19] = V.localInterfaceIP4bytes[3]
	}

	RecalculateIPv4HeaderChecksum(V.IP_IPv4Header)
	RecalculateTransportChecksum(V.IP_IPv4Header, V.IP_TPHeader)

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
	// wipe the old checksum before calculating
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
		binary.BigEndian.PutUint16(TPPacket[6:8], ^uint16(csum))
	}
}
