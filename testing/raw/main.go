package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
)

const (
	ETH_P_ALL = 0x0003 // Listen for all Ethernet protocols
)

func main() {
	if os.Getuid() != 0 {
		log.Fatal("Must run as root")
	}

	// Create a raw socket.  Note: syscall.SOCK_RAW requires root.
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(ETH_P_ALL)))
	if err != nil {
		log.Fatalf("Error creating socket: %v", err)
	}
	ifaceName := "enx9cbf0d00a640" // Replace with your desired interface name
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		log.Fatalf("Error getting interface %s: %v", ifaceName, err)
	}

	// Construct a sockaddr_ll structure for binding to the interface
	var addr syscall.SockaddrLinklayer
	// addr.Family = syscall.AF_PACKET
	addr.Protocol = htons(ETH_P_ALL)
	addr.Ifindex = iface.Index

	err = syscall.Bind(fd, &addr)
	if err != nil {
		log.Fatalf("Error binding to interface %s: %v", ifaceName, err)
	}

	// Receive packets
	buffer := make([]byte, 1500) // Adjust buffer size as needed

	fmt.Println("Listening for raw packets on", ifaceName)
	for {
		n, addr, err := syscall.Recvfrom(fd, buffer, 0)
		if err != nil {
			log.Printf("Error receiving packet: %v", err)
			continue
		}

		// Process the received packet
		processPacket(buffer[:n], addr)
	}
}

func processPacket(packet []byte, addr syscall.Sockaddr) {
	// This function will process each raw packet.
	// You need to parse the Ethernet header, IP header, and UDP header to extract
	// the desired information.

	// Convert Sockaddr to SockaddrLinklayer
	linkLayerAddr, ok := addr.(*syscall.SockaddrLinklayer)
	if !ok {
		log.Println("Unexpected address type:", addr)
		return
	}
	if len(packet) > 1500 {
		fmt.Println(len(packet))
	}

	// Extract information from Ethernet header
	ethHeader := EthernetHeader(packet)
	ethType := ethHeader.EtherType()

	// Check for IPv4 (0x0800) or IPv6 (0x86DD)
	switch ethType {
	case 0x0800: // IPv4
		ipHeader := IPv4Header(packet[14:]) // Ethernet header is 14 bytes
		protocol := ipHeader.Protocol()
		if protocol == 17 { // UDP
			udpHeader := UDPHeader(packet[14+ipHeader.IHL()*4:]) // IP header length is variable (IHL*4)
			// Print or process the UDP packet here
			fmt.Printf("Received UDP Packet on interface index %d, protocol: %d, source port: %d, dest port: %d, from %s to %s, packet length: %d\n",
				linkLayerAddr.Ifindex,
				protocol,
				udpHeader.SourcePort(),
				udpHeader.DestinationPort(),
				ipHeader.SourceAddress(),
				ipHeader.DestinationAddress(),
				len(packet),
			)
			// Access UDP data:
			udpData := packet[14+ipHeader.IHL()*4+8:] // 8 bytes for UDP header
			_ = udpData                               // Use udpData for further processing

		}
	case 0x86DD: // IPv6
		// IPv6 implementation would be similar
		fmt.Println("Received IPv6 packet (implementation omitted)")
	default:
		fmt.Printf("Received non-IP packet of type 0x%X\n", ethType)
	}

	// Example of printing the whole packet
	//fmt.Printf("Received packet from interface index %d, length: %d, data: %X\n", linkLayerAddr.Ifindex, len(packet), packet)

}

// Helper functions to convert byte slices to network order integers
func ntohs(i uint16) uint16 {
	b := make([]byte, 2)
	b[0] = byte(i >> 8)
	b[1] = byte(i & 0xFF)
	return uint16(b[0])<<8 | uint16(b[1])
}

func htons(i uint16) uint16 {
	b := make([]byte, 2)
	b[0] = byte(i & 0xFF)
	b[1] = byte(i >> 8)
	return uint16(b[0])<<8 | uint16(b[1])
}

// Ethernet Header Structure (simplified)
type EthernetHeader []byte

func (h EthernetHeader) DestinationMAC() net.HardwareAddr {
	return net.HardwareAddr(h[0:6])
}

func (h EthernetHeader) SourceMAC() net.HardwareAddr {
	return net.HardwareAddr(h[6:12])
}

func (h EthernetHeader) EtherType() uint16 {
	return uint16(h[12])<<8 | uint16(h[13])
}

// IPv4 Header Structure (simplified)
type IPv4Header []byte

func (h IPv4Header) Version() uint8 {
	return h[0] >> 4
}

func (h IPv4Header) IHL() uint8 {
	return h[0] & 0x0F
}

func (h IPv4Header) TotalLength() uint16 {
	return uint16(h[2])<<8 | uint16(h[3])
}

func (h IPv4Header) Protocol() uint8 {
	return h[9]
}

func (h IPv4Header) SourceAddress() net.IP {
	return net.IPv4(h[12], h[13], h[14], h[15])
}

func (h IPv4Header) DestinationAddress() net.IP {
	return net.IPv4(h[16], h[17], h[18], h[19])
}

// UDP Header Structure
type UDPHeader []byte

func (h UDPHeader) SourcePort() uint16 {
	return uint16(h[0])<<8 | uint16(h[1])
}

func (h UDPHeader) DestinationPort() uint16 {
	return uint16(h[2])<<8 | uint16(h[3])
}

func (h UDPHeader) Length() uint16 {
	return uint16(h[4])<<8 | uint16(h[5])
}

func (h UDPHeader) Checksum() uint16 {
	return uint16(h[6])<<8 | uint16(h[7])
}
