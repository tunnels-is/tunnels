package core

import (
	"encoding/binary"
	"testing"
)

// func BenchmarkOriginalIPChecksum(b *testing.B) {
// 	header := []byte{69, 32, 0, 40, 0, 0, 64, 0, 54, 6, 106, 16, 89, 147, 109, 61, 172, 22, 22, 22}
// 	for i := 0; i < b.N; i++ {
// 		RecalculateAndReplaceIPv4HeaderChecksum_old_donotremoveyet(header)
// 	}
// }

// LLMS still kind of suck
// func BenchmarkOriginalIPChecksumGrok(b *testing.B) {
// 	header := []byte{69, 32, 0, 40, 0, 0, 64, 0, 54, 6, 106, 16, 89, 147, 109, 61, 172, 22, 22, 22}
// 	for i := 0; i < b.N; i++ {
// 		RecalculateAndReplaceIPv4HeaderChecksumv2(header)
// 	}
// }

// func BenchmarkOriginalIPChecksumo4Mini(b *testing.B) {
// 	header := []byte{69, 32, 0, 40, 0, 0, 64, 0, 54, 6, 106, 16, 89, 147, 109, 61, 172, 22, 22, 22}
// 	for i := 0; i < b.N; i++ {
// 		RecalculateIPv4HeaderChecksum(header)
// 	}
// }

// func BenchmarkOriginalIPChecksumCopilot(b *testing.B) {
// 	header := []byte{69, 32, 0, 40, 0, 0, 64, 0, 54, 6, 106, 16, 89, 147, 109, 61, 172, 22, 22, 22}
// 	for i := 0; i < b.N; i++ {
// 		RecalculateIPv4HeaderChecksumCopilot(header)
// 	}
// }

func RecalculateAndReplaceIPv4HeaderChecksumv2(bytes []byte) {
	// Reset checksum to zero
	bytes[10] = 0
	bytes[11] = 0

	// Calculate checksum
	var csum uint32
	for i := 0; i < len(bytes); i += 2 {
		csum += uint32(bytes[i]) << 8
		if i+1 < len(bytes) {
			csum += uint32(bytes[i+1])
		}
	}

	// Fold 32-bit sum into 16 bits with one pass
	csum = (csum >> 16) + (csum & 0xFFFF)
	csum += csum >> 16

	// One's complement
	csum = ^csum & 0xFFFF

	// Store the checksum in big endian
	bytes[10] = byte(csum >> 8)
	bytes[11] = byte(csum)
}

func RecalculateIPv4HeaderChecksumCopilot(bytes []byte) {
	// Reset checksum fields
	bytes[10], bytes[11] = 0, 0

	var csum uint32
	length := len(bytes)

	// Unroll the loop for better performance
	for i := 0; i < length-1; i += 2 {
		csum += uint32(bytes[i])<<8 | uint32(bytes[i+1])
	}

	// Add potential trailing byte if length is odd
	// if length%2 != 0 {
	// 	csum += uint32(bytes[length-1]) << 8
	// }

	// Fold 32-bit sum to 16 bits
	for csum > 0xFFFF {
		csum = (csum >> 16) + (csum & 0xFFFF)
	}
	csum = ^csum

	// Set checksum fields
	bytes[10], bytes[11] = byte(csum>>8), byte(csum&0xFF)
}

func BenchmarkOriginalTCPHeader(b *testing.B) {
	tcpp := []byte{60, 91, 144, 61, 68, 212, 99, 115, 41, 155, 21, 178, 88, 133, 108, 242, 143, 84, 34, 228, 76, 253, 83, 210, 13, 44, 28, 149, 144, 5, 231, 38, 6, 237, 194, 240, 38, 138, 205, 111, 126, 224, 12, 179, 224, 74, 105, 18, 13, 5, 215, 175, 223, 104, 107, 193, 244, 123, 180, 38, 89, 230, 255, 137, 103, 213, 242, 58, 118, 59, 227, 106, 187, 18, 95, 9, 238, 94, 21, 76, 106, 197, 245, 103, 60, 174, 115, 206, 221, 13, 100, 160, 0, 59, 228, 150, 92, 165, 246, 98, 190, 82, 156, 89, 231, 105, 178, 5, 226, 180, 38, 27, 28, 24, 67, 229, 153, 94, 64, 236, 62, 243, 249, 6, 63, 31, 70, 150, 214, 26, 162, 192, 98, 176, 176, 248, 20, 6, 236, 120, 98, 125, 20, 237, 226, 243, 158, 85, 238, 243, 174, 93, 113, 218, 46, 96, 202, 210, 232, 167, 189, 38, 195, 103, 194, 223, 102, 152, 117, 179, 171, 187, 22, 173, 10, 48, 47, 227, 118, 135, 10, 109, 231, 229, 172, 97, 161, 218, 142, 98, 78, 191, 245, 0, 2, 49, 211, 24, 117, 180, 225, 84, 216, 230, 85, 160, 6, 30, 202, 48, 90, 21, 55, 191, 173, 242, 7, 150, 128, 245, 187, 112, 47, 138, 250, 99, 68, 158, 183, 24, 106, 246, 242, 137, 50, 17, 26, 11, 86, 180, 203, 219, 38, 251, 36, 12, 43, 87, 251, 38, 20, 109, 110, 183, 64, 226, 83, 170, 20, 208, 246, 195, 184, 195, 109, 3, 165, 131, 138, 64, 166, 4, 101, 217, 95, 178, 59, 157, 131, 250, 39, 55, 158, 227, 44, 23, 166, 136, 151, 170, 6, 157, 46, 106, 229, 148, 33, 110, 19, 195, 162, 26, 91, 220, 40, 36, 52, 176, 128, 18, 241, 19, 10, 252, 69, 113, 122, 117, 45, 0, 72, 168, 194, 7, 250, 71, 74, 237, 64, 132, 242, 8, 196, 218, 65, 27, 140, 43, 47, 251, 162, 155, 20, 202, 150, 41, 249, 93, 216, 150, 197, 83, 137, 94, 88, 235, 223, 225, 162, 223, 81, 23, 9, 117, 0, 53, 170, 248, 19, 251, 135, 28, 16, 27, 152, 218, 2, 218, 10, 175, 59, 21, 98, 96, 188, 211, 176, 148, 56, 231, 251, 63, 56, 183, 175, 162, 73, 107, 230, 94, 102, 124, 50, 211, 78, 34, 137, 225, 137, 160, 247, 124, 229, 4, 95, 192, 241, 97, 122, 202, 161, 0, 206, 183, 78, 239, 25, 72, 185, 28, 32, 128, 194, 226, 194, 113, 106, 53, 234, 196, 188, 222, 43, 158, 71, 228, 79, 109, 222, 213, 207, 179, 131, 240, 95, 108, 188, 118, 123, 77, 12, 119, 203, 70, 146, 17, 21, 15, 243, 146, 24, 47, 139, 201, 110, 240, 226, 71, 181, 95, 142, 154, 212, 128, 189, 83, 224, 138, 78, 172, 161, 166, 245, 209, 155, 230, 62, 8, 1, 198, 181, 1, 253, 44, 21, 191, 232, 189, 52, 89, 55, 35, 109, 118, 74, 195, 223, 121, 209, 53, 186, 30, 22, 199, 172, 209, 91, 157, 137, 108, 240, 183, 33, 98, 160, 70, 193, 21, 164, 224, 192, 78, 198, 117, 106, 228, 237, 82, 52, 202, 3, 223, 117, 39, 141, 159, 141, 120, 110, 96, 56, 141, 117, 155, 111, 166, 231, 63, 165, 88, 77, 110, 248, 66, 37, 190, 160, 48, 177, 245, 190, 209, 135, 215, 39, 101, 238, 239, 32, 215, 232, 187, 94, 253, 95, 45, 134, 127, 79, 71, 193, 128, 105, 67, 44, 215, 147, 36, 9, 79, 71, 0, 212, 154, 12, 15, 121, 194, 78, 130, 213, 184, 199, 253, 107, 173, 26, 42, 252, 234, 12, 115, 90, 110, 253, 244, 148, 17, 119, 23, 102, 165, 80, 231, 232, 3, 51, 230, 201, 60, 124, 126, 211, 238, 192, 161, 176, 169, 163, 84, 13, 124, 148, 206, 153, 70, 195, 66, 209, 19, 88, 167, 157, 216, 203, 19, 142, 110, 22, 110, 192, 96, 147, 99, 252, 92, 97, 9, 88, 42, 15, 41, 13, 16, 254, 18, 193, 169, 54, 163, 252, 247, 151, 58, 169, 231, 167, 212, 26, 63, 59, 255, 109, 93, 189, 8, 64, 104, 144, 13, 93, 181, 99, 4, 100, 76, 25, 156, 54}

	header := []byte{69, 32, 0, 40, 0, 0, 64, 0, 54, 6, 106, 16, 89, 147, 109, 61, 172, 22, 22, 22}

	for i := 0; i < b.N; i++ {
		RecalculateTransportChecksum(header, tcpp)
	}
}

func BenchmarkTCPHeadero4mini(b *testing.B) {
	tcpp := []byte{60, 91, 144, 61, 68, 212, 99, 115, 41, 155, 21, 178, 88, 133, 108, 242, 143, 84, 34, 228, 76, 253, 83, 210, 13, 44, 28, 149, 144, 5, 231, 38, 6, 237, 194, 240, 38, 138, 205, 111, 126, 224, 12, 179, 224, 74, 105, 18, 13, 5, 215, 175, 223, 104, 107, 193, 244, 123, 180, 38, 89, 230, 255, 137, 103, 213, 242, 58, 118, 59, 227, 106, 187, 18, 95, 9, 238, 94, 21, 76, 106, 197, 245, 103, 60, 174, 115, 206, 221, 13, 100, 160, 0, 59, 228, 150, 92, 165, 246, 98, 190, 82, 156, 89, 231, 105, 178, 5, 226, 180, 38, 27, 28, 24, 67, 229, 153, 94, 64, 236, 62, 243, 249, 6, 63, 31, 70, 150, 214, 26, 162, 192, 98, 176, 176, 248, 20, 6, 236, 120, 98, 125, 20, 237, 226, 243, 158, 85, 238, 243, 174, 93, 113, 218, 46, 96, 202, 210, 232, 167, 189, 38, 195, 103, 194, 223, 102, 152, 117, 179, 171, 187, 22, 173, 10, 48, 47, 227, 118, 135, 10, 109, 231, 229, 172, 97, 161, 218, 142, 98, 78, 191, 245, 0, 2, 49, 211, 24, 117, 180, 225, 84, 216, 230, 85, 160, 6, 30, 202, 48, 90, 21, 55, 191, 173, 242, 7, 150, 128, 245, 187, 112, 47, 138, 250, 99, 68, 158, 183, 24, 106, 246, 242, 137, 50, 17, 26, 11, 86, 180, 203, 219, 38, 251, 36, 12, 43, 87, 251, 38, 20, 109, 110, 183, 64, 226, 83, 170, 20, 208, 246, 195, 184, 195, 109, 3, 165, 131, 138, 64, 166, 4, 101, 217, 95, 178, 59, 157, 131, 250, 39, 55, 158, 227, 44, 23, 166, 136, 151, 170, 6, 157, 46, 106, 229, 148, 33, 110, 19, 195, 162, 26, 91, 220, 40, 36, 52, 176, 128, 18, 241, 19, 10, 252, 69, 113, 122, 117, 45, 0, 72, 168, 194, 7, 250, 71, 74, 237, 64, 132, 242, 8, 196, 218, 65, 27, 140, 43, 47, 251, 162, 155, 20, 202, 150, 41, 249, 93, 216, 150, 197, 83, 137, 94, 88, 235, 223, 225, 162, 223, 81, 23, 9, 117, 0, 53, 170, 248, 19, 251, 135, 28, 16, 27, 152, 218, 2, 218, 10, 175, 59, 21, 98, 96, 188, 211, 176, 148, 56, 231, 251, 63, 56, 183, 175, 162, 73, 107, 230, 94, 102, 124, 50, 211, 78, 34, 137, 225, 137, 160, 247, 124, 229, 4, 95, 192, 241, 97, 122, 202, 161, 0, 206, 183, 78, 239, 25, 72, 185, 28, 32, 128, 194, 226, 194, 113, 106, 53, 234, 196, 188, 222, 43, 158, 71, 228, 79, 109, 222, 213, 207, 179, 131, 240, 95, 108, 188, 118, 123, 77, 12, 119, 203, 70, 146, 17, 21, 15, 243, 146, 24, 47, 139, 201, 110, 240, 226, 71, 181, 95, 142, 154, 212, 128, 189, 83, 224, 138, 78, 172, 161, 166, 245, 209, 155, 230, 62, 8, 1, 198, 181, 1, 253, 44, 21, 191, 232, 189, 52, 89, 55, 35, 109, 118, 74, 195, 223, 121, 209, 53, 186, 30, 22, 199, 172, 209, 91, 157, 137, 108, 240, 183, 33, 98, 160, 70, 193, 21, 164, 224, 192, 78, 198, 117, 106, 228, 237, 82, 52, 202, 3, 223, 117, 39, 141, 159, 141, 120, 110, 96, 56, 141, 117, 155, 111, 166, 231, 63, 165, 88, 77, 110, 248, 66, 37, 190, 160, 48, 177, 245, 190, 209, 135, 215, 39, 101, 238, 239, 32, 215, 232, 187, 94, 253, 95, 45, 134, 127, 79, 71, 193, 128, 105, 67, 44, 215, 147, 36, 9, 79, 71, 0, 212, 154, 12, 15, 121, 194, 78, 130, 213, 184, 199, 253, 107, 173, 26, 42, 252, 234, 12, 115, 90, 110, 253, 244, 148, 17, 119, 23, 102, 165, 80, 231, 232, 3, 51, 230, 201, 60, 124, 126, 211, 238, 192, 161, 176, 169, 163, 84, 13, 124, 148, 206, 153, 70, 195, 66, 209, 19, 88, 167, 157, 216, 203, 19, 142, 110, 22, 110, 192, 96, 147, 99, 252, 92, 97, 9, 88, 42, 15, 41, 13, 16, 254, 18, 193, 169, 54, 163, 252, 247, 151, 58, 169, 231, 167, 212, 26, 63, 59, 255, 109, 93, 189, 8, 64, 104, 144, 13, 93, 181, 99, 4, 100, 76, 25, 156, 54}

	header := []byte{69, 32, 0, 40, 0, 0, 64, 0, 54, 6, 106, 16, 89, 147, 109, 61, 172, 22, 22, 22}
	for i := 0; i < b.N; i++ {
		RecalculateTransportChecksumv3(header, tcpp)
	}
}

// CalculateTransportChecksum computes the checksum for the transport layer (TCP/UDP)
func CalculateTransportChecksumv2(ipv4Header []byte, transportPacket []byte) {
	// wipe the old checksum before calculating
	if ipv4Header[9] == 6 {
		transportPacket[16] = 0
		transportPacket[17] = 0
	} else if ipv4Header[9] == 17 {
		transportPacket[6] = 0
		transportPacket[7] = 0
	}
	// Extract protocol type (6 for TCP, 17 for UDP)
	protocol := ipv4Header[9]
	var checksum uint32

	// Add IPv4 header fields to checksum
	checksum += uint32(ipv4Header[12])<<8 + uint32(ipv4Header[13]) // Source IP
	checksum += uint32(ipv4Header[14])<<8 + uint32(ipv4Header[15]) // Destination IP
	checksum += uint32(ipv4Header[16])<<8 + uint32(ipv4Header[17]) // Source Port (for transport layer)
	checksum += uint32(ipv4Header[18])<<8 + uint32(ipv4Header[19]) // Destination Port (for transport layer)
	checksum += uint32(protocol)                                   // Protocol (6 for TCP, 17 for UDP)

	// Add TCP/UDP length to checksum
	packetLength := len(transportPacket)
	checksum += uint32(packetLength) & 0xffff
	checksum += uint32(packetLength) >> 16

	// Process the transport packet in 16-bit chunks
	for i := 0; i < packetLength-1; i += 2 {
		checksum += uint32(transportPacket[i]) << 8
		checksum += uint32(transportPacket[i+1])
	}

	// Handle odd byte (if the packet length is odd)
	if packetLength%2 == 1 {
		checksum += uint32(transportPacket[packetLength-1]) << 8
	}

	// Fold the checksum (carry over) to 16 bits
	for checksum > 0xffff {
		checksum = (checksum >> 16) + (checksum & 0xffff)
	}

	// Return the complement of the checksum
	if ipv4Header[9] == 6 {
		binary.BigEndian.PutUint16(transportPacket[16:18], ^uint16(checksum))
	} else if ipv4Header[9] == 17 {
		binary.BigEndian.PutUint16(transportPacket[6:8], ^uint16(checksum))
	}
}

func RecalculateTransportChecksumv3(IPv4Header, TPPacket []byte) {
	// Zero out the checksum field directly based on protocol
	switch IPv4Header[9] {
	case 6: // TCP
		TPPacket[16], TPPacket[17] = 0, 0
	case 17: // UDP
		TPPacket[6], TPPacket[7] = 0, 0
	}

	var csum uint32

	// Combine source and destination IP addresses in one go
	csum += uint32(binary.BigEndian.Uint16(IPv4Header[12:])) + uint32(binary.BigEndian.Uint16(IPv4Header[14:]))
	csum += uint32(binary.BigEndian.Uint16(IPv4Header[16:])) + uint32(binary.BigEndian.Uint16(IPv4Header[18:]))

	// Add protocol and packet length
	csum += uint32(IPv4Header[9])
	tcpLength := uint32(len(TPPacket))
	csum += tcpLength + (tcpLength >> 16)

	// Use BigEndian for loop to leverage CPU's natural byte order handling
	for i := 0; i+1 < len(TPPacket); i += 2 {
		csum += uint32(binary.BigEndian.Uint16(TPPacket[i:]))
	}

	// Handle odd length
	if len(TPPacket)&1 == 1 {
		csum += uint32(TPPacket[len(TPPacket)-1]) << 8
	}

	// Fold sum to 16 bits
	for csum > 0xffff {
		csum = (csum >> 16) + (csum & 0xffff)
	}

	// Store checksum back in packet
	switch IPv4Header[9] {
	case 6:
		binary.BigEndian.PutUint16(TPPacket[16:], ^uint16(csum))
	case 17:
		binary.BigEndian.PutUint16(TPPacket[6:], ^uint16(csum))
	}
}

func RecalculateTransportChecksumv4(IPv4Header, TPPacket []byte) {
	// Zero out checksum
	checksumOffset := map[byte]int{6: 16, 17: 6}[IPv4Header[9]]
	binary.BigEndian.PutUint16(TPPacket[checksumOffset:], 0)

	var csum uint32

	// Source and Destination IP aggregation
	for i := 12; i < 20; i += 2 {
		csum += uint32(binary.BigEndian.Uint16(IPv4Header[i:]))
	}

	// Protocol and packet length
	csum += uint32(IPv4Header[9])
	length := uint32(len(TPPacket))
	csum += length + (length >> 16)

	// Calculate checksum for TPPacket
	for i := 0; i < len(TPPacket); i += 2 {
		if i == checksumOffset {
			continue // Skip checksum field
		}
		csum += uint32(binary.BigEndian.Uint16(TPPacket[i:]))
	}

	// Handle odd length packet
	if len(TPPacket)&1 == 1 {
		csum += uint32(TPPacket[len(TPPacket)-1]) << 8
	}

	// Fold sum to 16 bits
	for csum > 0xffff {
		csum = (csum >> 16) + (csum & 0xffff)
	}

	// Set checksum
	binary.BigEndian.PutUint16(TPPacket[checksumOffset:], ^uint16(csum))
}

func RecalculateTransportChecksumTest(IPv4Header []byte, TPPacket []byte) {
	// wipe the old checksum before calculating
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

	if IPv4Header[9] == 6 {
		binary.BigEndian.PutUint16(TPPacket[16:18], ^uint16(csum))
	} else if IPv4Header[9] == 17 {
		binary.BigEndian.PutUint16(TPPacket[6:8], ^uint16(csum))
	}
}
