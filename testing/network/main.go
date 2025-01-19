package main

import (
	"fmt"
	"math/rand"
	"strconv"
)

func main() {
	_, packet := generateRandomData()
	out := ""
	for _, v := range packet {
		out += strconv.Itoa(int(v)) + ","
	}
	fmt.Println(out)
}

// Helper function to generate random IPv4 header and transport packet
func generateRandomData() ([]byte, []byte) {
	// Generate random IPv4 header with the protocol set to 6 (TCP) or 17 (UDP)
	IPv4Header := make([]byte, 20)
	// Setting protocol (byte 9) to either 6 (TCP) or 17 (UDP)
	if rand.Intn(2) == 0 {
		IPv4Header[9] = 6 // TCP
	} else {
		IPv4Header[9] = 17 // UDP
	}
	// Generate random transport packet (TCP/UDP)
	TPPacket := make([]byte, rand.Intn(1000)+20) // random length from 20 to 1020 bytes
	// Fill with random data
	rand.Read(TPPacket)

	// Return both slices
	return IPv4Header, TPPacket
}
