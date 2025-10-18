package client

import (
	"encoding/binary"
	"testing"
)

func TestRecalculateIPv4HeaderChecksum(t *testing.T) {
	tests := []struct {
		name           string
		header         []byte
		expectedResult []byte
	}{
		{
			name: "standard IPv4 header",
			header: []byte{
				0x45, 0x00, // Version, IHL, DSCP, ECN, Total Length
				0x00, 0x28, // Total Length continued
				0x00, 0x00, // Identification
				0x40, 0x00, // Flags, Fragment Offset
				0x40, 0x06, // TTL, Protocol (TCP)
				0x00, 0x00, // Checksum (will be calculated)
				0xc0, 0xa8, 0x01, 0x01, // Source IP (192.168.1.1)
				0xc0, 0xa8, 0x01, 0x02, // Dest IP (192.168.1.2)
			},
			expectedResult: []byte{
				0x45, 0x00,
				0x00, 0x28,
				0x00, 0x00,
				0x40, 0x00,
				0x40, 0x06,
				0xb7, 0x7c, // Corrected expected checksum
				0xc0, 0xa8, 0x01, 0x01,
				0xc0, 0xa8, 0x01, 0x02,
			},
		},
		{
			name: "different source/dest IPs",
			header: []byte{
				0x45, 0x00,
				0x00, 0x3c,
				0x1c, 0x46,
				0x40, 0x00,
				0x40, 0x06,
				0x00, 0x00, // Checksum placeholder
				0xac, 0x10, 0x0a, 0x63, // Source IP (172.16.10.99)
				0xac, 0x10, 0x0a, 0x0c, // Dest IP (172.16.10.12)
			},
			expectedResult: []byte{
				0x45, 0x00,
				0x00, 0x3c,
				0x1c, 0x46,
				0x40, 0x00,
				0x40, 0x06,
				0xb1, 0xe6, // Expected checksum
				0xac, 0x10, 0x0a, 0x63,
				0xac, 0x10, 0x0a, 0x0c,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			header := make([]byte, len(tc.header))
			copy(header, tc.header)

			RecalculateIPv4HeaderChecksum(header)

			// Check the checksum field (bytes 10 and 11)
			if header[10] != tc.expectedResult[10] || header[11] != tc.expectedResult[11] {
				t.Errorf("Checksum mismatch: got [0x%02x, 0x%02x], expected [0x%02x, 0x%02x]",
					header[10], header[11], tc.expectedResult[10], tc.expectedResult[11])
			}

			t.Logf("Checksum: 0x%02x%02x ✓", header[10], header[11])
		})
	}
}

func TestRecalculateIPv4HeaderChecksumConsistency(t *testing.T) {
	// Test that calculating checksum twice gives the same result
	header := []byte{
		0x45, 0x00, 0x00, 0x28, 0x00, 0x00, 0x40, 0x00,
		0x40, 0x06, 0x00, 0x00, 0xc0, 0xa8, 0x01, 0x01,
		0xc0, 0xa8, 0x01, 0x02,
	}

	// Calculate first time
	RecalculateIPv4HeaderChecksum(header)
	checksum1 := binary.BigEndian.Uint16(header[10:12])

	// Calculate second time
	RecalculateIPv4HeaderChecksum(header)
	checksum2 := binary.BigEndian.Uint16(header[10:12])

	if checksum1 != checksum2 {
		t.Errorf("Checksum not consistent: first=0x%04x, second=0x%04x", checksum1, checksum2)
	}

	t.Logf("Consistent checksum: 0x%04x ✓", checksum1)
}

func TestRecalculateTransportChecksum_TCP(t *testing.T) {
	// IPv4 header for TCP packet
	ipHeader := []byte{
		0x45, 0x00, 0x00, 0x3c, // Version, IHL, DSCP, Total Length
		0x1c, 0x46, 0x40, 0x00, // Identification, Flags
		0x40, 0x06, 0xb1, 0xe6, // TTL, Protocol (6=TCP), Checksum
		0xc0, 0xa8, 0x00, 0x68, // Source IP (192.168.0.104)
		0xc0, 0xa8, 0x00, 0x01, // Dest IP (192.168.0.1)
	}

	// Minimal TCP header (20 bytes)
	tcpPacket := []byte{
		0xc3, 0x53, // Source Port (50003)
		0x00, 0x50, // Dest Port (80)
		0x00, 0x00, 0x00, 0x00, // Sequence Number
		0x00, 0x00, 0x00, 0x00, // Acknowledgment Number
		0x50, 0x02, // Data Offset, Flags (SYN)
		0x00, 0x00, // Window Size
		0x00, 0x00, // Checksum (will be calculated)
		0x00, 0x00, // Urgent Pointer
	}

	RecalculateTransportChecksum(ipHeader, tcpPacket)

	// Verify checksum was calculated (not zero)
	checksum := binary.BigEndian.Uint16(tcpPacket[16:18])
	if checksum == 0 {
		t.Error("TCP checksum should not be zero after calculation")
	}

	t.Logf("TCP Checksum: 0x%04x ✓", checksum)
}

func TestRecalculateTransportChecksum_UDP(t *testing.T) {
	// IPv4 header for UDP packet
	ipHeader := []byte{
		0x45, 0x00, 0x00, 0x1c, // Version, IHL, DSCP, Total Length
		0x00, 0x00, 0x40, 0x00, // Identification, Flags
		0x40, 0x11, 0x00, 0x00, // TTL, Protocol (17=UDP), Checksum
		0xc0, 0xa8, 0x01, 0x01, // Source IP (192.168.1.1)
		0xc0, 0xa8, 0x01, 0x02, // Dest IP (192.168.1.2)
	}

	// Minimal UDP header (8 bytes)
	udpPacket := []byte{
		0x04, 0xd2, // Source Port (1234)
		0x00, 0x35, // Dest Port (53 - DNS)
		0x00, 0x08, // Length
		0x00, 0x00, // Checksum (will be calculated)
	}

	RecalculateTransportChecksum(ipHeader, udpPacket)

	// Verify checksum was calculated (not zero)
	checksum := binary.BigEndian.Uint16(udpPacket[6:8])
	if checksum == 0 {
		t.Error("UDP checksum should not be zero after calculation")
	}

	t.Logf("UDP Checksum: 0x%04x ✓", checksum)
}

func TestRecalculateTransportChecksumConsistency_TCP(t *testing.T) {
	// Test that calculating TCP checksum twice gives the same result
	ipHeader := []byte{
		0x45, 0x00, 0x00, 0x3c, 0x1c, 0x46, 0x40, 0x00,
		0x40, 0x06, 0xb1, 0xe6, 0xc0, 0xa8, 0x00, 0x68,
		0xc0, 0xa8, 0x00, 0x01,
	}

	tcpPacket := []byte{
		0xc3, 0x53, 0x00, 0x50, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x50, 0x02, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}

	// Calculate first time
	RecalculateTransportChecksum(ipHeader, tcpPacket)
	checksum1 := binary.BigEndian.Uint16(tcpPacket[16:18])

	// Calculate second time
	RecalculateTransportChecksum(ipHeader, tcpPacket)
	checksum2 := binary.BigEndian.Uint16(tcpPacket[16:18])

	if checksum1 != checksum2 {
		t.Errorf("TCP checksum not consistent: first=0x%04x, second=0x%04x", checksum1, checksum2)
	}

	t.Logf("Consistent TCP checksum: 0x%04x ✓", checksum1)
}

func TestRecalculateTransportChecksumConsistency_UDP(t *testing.T) {
	// Test that calculating UDP checksum twice gives the same result
	ipHeader := []byte{
		0x45, 0x00, 0x00, 0x1c, 0x00, 0x00, 0x40, 0x00,
		0x40, 0x11, 0x00, 0x00, 0xc0, 0xa8, 0x01, 0x01,
		0xc0, 0xa8, 0x01, 0x02,
	}

	udpPacket := []byte{
		0x04, 0xd2, 0x00, 0x35, 0x00, 0x08, 0x00, 0x00,
	}

	// Calculate first time
	RecalculateTransportChecksum(ipHeader, udpPacket)
	checksum1 := binary.BigEndian.Uint16(udpPacket[6:8])

	// Calculate second time
	RecalculateTransportChecksum(ipHeader, udpPacket)
	checksum2 := binary.BigEndian.Uint16(udpPacket[6:8])

	if checksum1 != checksum2 {
		t.Errorf("UDP checksum not consistent: first=0x%04x, second=0x%04x", checksum1, checksum2)
	}

	t.Logf("Consistent UDP checksum: 0x%04x ✓", checksum1)
}

func TestChecksumWithData(t *testing.T) {
	// Test TCP packet with actual data payload
	ipHeader := []byte{
		0x45, 0x00, 0x00, 0x40, // Version, IHL, Total Length (64 bytes)
		0x00, 0x00, 0x40, 0x00,
		0x40, 0x06, 0x00, 0x00,
		0xc0, 0xa8, 0x01, 0x64, // 192.168.1.100
		0xc0, 0xa8, 0x01, 0x01, // 192.168.1.1
	}

	// TCP header + data
	tcpPacketWithData := []byte{
		// TCP header (20 bytes)
		0x04, 0xd2, 0x00, 0x50, 0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x00, 0x50, 0x18, 0x20, 0x00,
		0x00, 0x00, 0x00, 0x00,
		// Data payload (12 bytes)
		'H', 'e', 'l', 'l', 'o', ' ', 'W', 'o', 'r', 'l', 'd', '!',
	}

	RecalculateTransportChecksum(ipHeader, tcpPacketWithData)

	checksum := binary.BigEndian.Uint16(tcpPacketWithData[16:18])
	if checksum == 0 {
		t.Error("TCP checksum with data should not be zero")
	}

	t.Logf("TCP Checksum with data: 0x%04x ✓", checksum)
}
