package wgserver

import (
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/blake2s"
)

func TestValidMAC1(t *testing.T) {
	serverPub := make([]byte, 32)
	if _, err := rand.Read(serverPub); err != nil {
		t.Fatal(err)
	}

	pkt := make([]byte, msgInitiationSize)
	if _, err := rand.Read(pkt[:116]); err != nil {
		t.Fatal(err)
	}
	pkt[0], pkt[1], pkt[2], pkt[3] = msgInitiation, 0, 0, 0

	key := noiseHash([]byte(labelMAC1), serverPub)
	mac, err := blake2s.New128(key)
	if err != nil {
		t.Fatal(err)
	}
	mac.Write(pkt[:116])
	mac.Sum(pkt[116:116])

	if !validMAC1(pkt, serverPub) {
		t.Fatal("a correctly-stamped mac1 was rejected")
	}

	pkt[10] ^= 0xff
	if validMAC1(pkt, serverPub) {
		t.Fatal("tampered packet accepted")
	}
	pkt[10] ^= 0xff

	otherPub := make([]byte, 32)
	if _, err := rand.Read(otherPub); err != nil {
		t.Fatal(err)
	}
	if validMAC1(pkt, otherPub) {
		t.Fatal("packet validated against the wrong server pubkey")
	}

	if validMAC1(pkt[:100], serverPub) {
		t.Fatal("short packet accepted")
	}
}
