package core

import (
	"encoding/binary"
	"time"
)

type packetDebugOut struct {
	Version byte
	Proto   byte
	SrcIP   []byte
	DstIP   []byte
	Flags   byte
	TCPH    []byte
}

func (V *Tunnel) RegisterPing(packet []byte) {
	defer RecoverAndLogToFile()
	V.TunnelSTATS.PingTime = time.Now()
	V.TunnelSTATS.CPU = packet[0]
	V.TunnelSTATS.MEM = packet[1]
	V.TunnelSTATS.DISK = packet[2]
	V.TunnelSTATS.ServerToClientMicro = time.Since(time.Unix(0, int64(binary.BigEndian.Uint64(packet[3:])))).Microseconds()
}

func debugProcessPacket(packet []byte) (P *packetDebugOut) {
	defer RecoverAndLogToFile()
	P = new(packetDebugOut)
	P.Version = packet[0] >> 4
	P.Proto = packet[9]
	P.SrcIP = append(P.DstIP, packet[12:16]...)
	P.DstIP = append(P.DstIP, packet[16:20]...)
	il := (packet[0] << 4 >> 4) * 4
	P.TCPH = packet[il:]
	if len(P.TCPH) > 13 {
		P.Flags = P.TCPH[13]
	} else {
		P.Flags = 0
	}
	return
}

func (V *Tunnel) ProcessEgressPacket(p *[]byte) (sendRemote bool) {
	packet := *p

	V.EP_Version = packet[0] >> 4
	if V.EP_Version != 4 {
		return false
	}

	V.EP_Protocol = packet[9]
	if V.EP_Protocol != 6 && V.EP_Protocol != 17 {
		return false
	}

	V.EP_IPv4HeaderLength = (packet[0] << 4 >> 4) * 4

	V.EP_IPv4Header = packet[:V.EP_IPv4HeaderLength]
	V.EP_TPHeader = packet[V.EP_IPv4HeaderLength:]

	V.EP_DstIP[0] = packet[16]
	V.EP_DstIP[1] = packet[17]
	V.EP_DstIP[2] = packet[18]
	V.EP_DstIP[3] = packet[19]

	V.EP_SrcPort[0] = V.EP_TPHeader[0]
	V.EP_SrcPort[1] = V.EP_TPHeader[1]
	V.EP_DstPort[0] = V.EP_TPHeader[2]
	V.EP_DstPort[1] = V.EP_TPHeader[3]

	if V.EP_Protocol == 6 {

		V.EP_SYN = V.EP_TPHeader[13] & 0x2

		V.EP_MP = V.CreateNEWPortMapping(V.TCP_EM, V.TCP_M, packet[12:20], V.EP_TPHeader[0:4])
		if V.EP_MP == nil {
			debugMissingEgressMapping(packet)
			return false
		}

	} else if V.EP_Protocol == 17 {

		V.EP_MP = V.CreateNEWPortMapping(V.UDP_EM, V.UDP_M, packet[12:20], V.EP_TPHeader[0:4])
		if V.EP_MP == nil {
			debugMissingEgressMapping(packet)
			return false
		}

	}

	if V.EP_Protocol == 6 {
		V.EP_MP.ERST = V.EP_TPHeader[13] & 0x4
		if V.EP_MP.EFIN == 0 {
			V.EP_MP.EFIN = V.EP_TPHeader[13] & 0x1
		}
		if V.EP_MP.ERST == 4 {
			V.EP_TPHeader[13] = 0b00010100
		}
	}

	V.EP_NAT_IP, V.EP_NAT_OK = V.TransLateIP(V.EP_DstIP)
	if V.EP_NAT_OK {
		V.EP_IPv4Header[16] = V.EP_NAT_IP[0]
		V.EP_IPv4Header[17] = V.EP_NAT_IP[1]
		V.EP_IPv4Header[18] = V.EP_NAT_IP[2]
		V.EP_IPv4Header[19] = V.EP_NAT_IP[3]
	}

	V.EP_TPHeader[0] = V.EP_MP.VPNPort[0]
	V.EP_TPHeader[1] = V.EP_MP.VPNPort[1]

	V.EP_IPv4Header[12] = V.EP_VPNSrcIP[0]
	V.EP_IPv4Header[13] = V.EP_VPNSrcIP[1]
	V.EP_IPv4Header[14] = V.EP_VPNSrcIP[2]
	V.EP_IPv4Header[15] = V.EP_VPNSrcIP[3]

	RecalculateAndReplaceIPv4HeaderChecksum(V.EP_IPv4Header)
	RecalculateAndReplaceTransportChecksum(V.EP_IPv4Header, V.EP_TPHeader)

	return true
}

func (V *Tunnel) ProcessIngressPacket(packet []byte) bool {
	V.IP_SrcIP[0] = packet[12]
	V.IP_SrcIP[1] = packet[13]
	V.IP_SrcIP[2] = packet[14]
	V.IP_SrcIP[3] = packet[15]

	V.IP_Protocol = packet[9]

	V.IP_IPv4HeaderLength = (packet[0] << 4 >> 4) * 4
	V.IP_IPv4Header = packet[:V.IP_IPv4HeaderLength]
	V.IP_TPHeader = packet[V.IP_IPv4HeaderLength:]

	V.IP_DstPort[0] = V.IP_TPHeader[2]
	V.IP_DstPort[1] = V.IP_TPHeader[3]

	V.IP_NAT_IP, V.IP_NAT_OK = V.REVERSE_NAT_CACHE[V.IP_SrcIP]
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

	if V.IP_Protocol == 6 {
		V.IP_MP = V.getIngressPortMapping(V.TCP_M, packet[12:16], V.IP_DstPort)
		if V.IP_MP == nil {
			return false
		}

	} else if V.IP_Protocol == 17 {
		V.IP_MP = V.getIngressPortMapping(V.UDP_M, packet[12:16], V.IP_DstPort)
		if V.IP_MP == nil {
			return false
		}
	}

	if V.IP_Protocol == 6 {
		if V.IP_MP.IRST == 0 {
			V.IP_MP.IRST = V.IP_TPHeader[13] & 0x4
		}
		if V.IP_MP.IFIN == 0 {
			V.IP_MP.IFIN = V.IP_TPHeader[13] & 0x1
		}
	}

	V.IP_TPHeader[2] = V.IP_MP.LocalPort[0]
	V.IP_TPHeader[3] = V.IP_MP.LocalPort[1]

	V.IP_IPv4Header[16] = V.IP_MP.OriginalSourceIP[0]
	V.IP_IPv4Header[17] = V.IP_MP.OriginalSourceIP[1]
	V.IP_IPv4Header[18] = V.IP_MP.OriginalSourceIP[2]
	V.IP_IPv4Header[19] = V.IP_MP.OriginalSourceIP[3]

	RecalculateAndReplaceIPv4HeaderChecksum(V.IP_IPv4Header)
	RecalculateAndReplaceTransportChecksum(V.IP_IPv4Header, V.IP_TPHeader)

	return true
}

func RecalculateAndReplaceIPv4HeaderChecksum(bytes []byte) {
	bytes[10] = 0
	bytes[11] = 0

	var csum uint32
	for i := 0; i < len(bytes); i += 2 {
		csum += uint32(bytes[i]) << 8
		csum += uint32(bytes[i+1])
	}
	for {
		if csum <= 65535 {
			break
		}
		csum = (csum >> 16) + uint32(uint16(csum))
	}

	binary.BigEndian.PutUint16(bytes[10:12], ^uint16(csum))
}

func RecalculateAndReplaceTransportChecksum(IPv4Header []byte, TPPacket []byte) {
	if IPv4Header[9] == 6 {
		TPPacket[16] = 0
		TPPacket[17] = 0
	} else if IPv4Header[9] == 17 {
		TPPacket[6] = 0
		TPPacket[7] = 0
	}

	var csum uint32
	csum += (uint32(IPv4Header[12]) + uint32(IPv4Header[14])) << 8
	csum += uint32(IPv4Header[13]) + uint32(IPv4Header[15])
	csum += (uint32(IPv4Header[16]) + uint32(IPv4Header[18])) << 8
	csum += uint32(IPv4Header[17]) + uint32(IPv4Header[19])
	csum += uint32(uint8(IPv4Header[9]))
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

	if IPv4Header[9] == 6 {
		binary.BigEndian.PutUint16(TPPacket[16:18], ^uint16(csum))
	} else if IPv4Header[9] == 17 {
		binary.BigEndian.PutUint16(TPPacket[6:8], ^uint16(csum))
	}
}
