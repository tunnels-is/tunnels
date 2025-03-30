package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
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
		err = syscall.Sendto(fd, buffer[:n], 0, addr)
		if err != nil {
			fmt.Println(err)
		}

		processPacket(buffer[:n], addr)

	}
}

func processPacket(packet []byte, addr syscall.Sockaddr) {
	if binary.BigEndian.Uint16(packet[36:38]) == 22 {
		return
	}
	if binary.BigEndian.Uint16(packet[34:36]) == 22 {
		return
	}
	fmt.Println(packet)
	fmt.Println(reflect.TypeOf(addr))

	ll, ok := addr.(*syscall.SockaddrLinklayer)
	if ok {
		fmt.Printf("%+v\n", ll)
	}
	// This function will process each raw packet.
	// You need to parse the Ethernet header, IP header, and UDP header to extract
	// the desired information.
	// // Print potentially interesting byte segments with labels.  Adjust these
	// based on what you expect to see in your network traffic.  These are *examples*.
	if len(packet) > 14 {
	} else {
		fmt.Println("Packet too short to extract MAC addresses/EtherType")
	}

	if len(packet) > 20 && packet[12] == 0x08 && packet[13] == 0x00 { //Check if it's IP
		fmt.Printf("DM:%X SM:%X ",
			packet[0:6],
			packet[6:12],
		)
		fmt.Printf("SIP: %d.%d.%d.%d ", packet[26], packet[27], packet[28], packet[29])
		fmt.Printf("DIP: %d.%d.%d.%d ", packet[30], packet[31], packet[32], packet[33])
	} else {
		fmt.Println("Not an IP packet, skipping IP header extraction.")
	}

	if len(packet) > 34 && packet[12] == 0x08 && packet[13] == 0x00 { //more checks
		protocolByte := packet[23]
		if protocolByte == 0x06 && len(packet) > 54 { //TCP
			fmt.Printf("TSP: %d ", binary.BigEndian.Uint16(packet[34:36]))
			fmt.Printf("TDP: %d ", binary.BigEndian.Uint16(packet[36:38]))
		} else if protocolByte == 0x11 && len(packet) > 42 { //UDP
			fmt.Printf("USP: %d ", binary.BigEndian.Uint16(packet[34:36]))
			fmt.Printf("UDP %d ", binary.BigEndian.Uint16(packet[36:38]))
		} else {
			fmt.Printf("Other TCP or UDP Protocol: %X\n", protocolByte)
		}
	} else {
		fmt.Println("Not an IP packet, skipping IP header extraction.")
	}
	fmt.Println()

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
