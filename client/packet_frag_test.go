package client

import (
	"encoding/binary"
	"net"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

// buildV4 builds a minimal IPv4 TCP/UDP packet with a chosen fragment field.
func buildV4(proto byte, src, dst [4]byte, fragField uint16, l4 []byte) []byte {
	pkt := make([]byte, 20+len(l4))
	pkt[0] = 0x45 // v4, IHL 5
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(pkt)))
	binary.BigEndian.PutUint16(pkt[6:8], fragField)
	pkt[9] = proto
	copy(pkt[12:16], src[:])
	copy(pkt[16:20], dst[:])
	copy(pkt[20:], l4)
	return pkt
}

func newBareTUN() *TUN {
	t := &TUN{}
	t.blockedPortsSet = make(map[[2]byte]uint16)
	t.NATEgress = make(map[[4]byte][4]byte)
	t.NATIngress = make(map[[4]byte][4]byte)
	t.ServerResponse = &types.ServerConnectResponse{}
	return t
}

// A non-first fragment (offset>0) must be forwarded WITHOUT its payload being
// touched — previously the code read a bogus port and overwrote 2 payload bytes
// with a "checksum".
func TestProcessEgress_TrailingFragmentNotCorrupted(t *testing.T) {
	tun := newBareTUN()
	// UDP proto, fragment offset = 185 (in 8-byte units), no MF.
	payload := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	orig := make([]byte, len(payload))
	copy(orig, payload)
	pkt := buildV4(17, [4]byte{10, 0, 0, 2}, [4]byte{8, 8, 8, 8}, 185, payload)
	p := pkt
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("trailing fragment should be forwarded, not dropped")
	}
	got := pkt[20:]
	for i := range orig {
		if got[i] != orig[i] {
			t.Fatalf("payload byte %d corrupted: got %d want %d (full=%v)", i, got[i], orig[i], got)
		}
	}
}

// A blocked port must NOT be inferred from a trailing fragment's payload bytes.
func TestProcessEgress_TrailingFragmentIgnoresPortBlock(t *testing.T) {
	tun := newBareTUN()
	// Make "port" bytes [2:4] of the payload map to a blocked port; a trailing
	// fragment must still pass because those bytes aren't a real port.
	tun.blockedPortsSet[[2]byte{0x00, 0x50}] = 80
	payload := []byte{0, 0, 0x00, 0x50, 0, 0, 0, 0}
	pkt := buildV4(6, [4]byte{10, 0, 0, 2}, [4]byte{8, 8, 8, 8}, 100 /*offset>0*/, payload)
	p := pkt
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("trailing fragment must not be port-blocked")
	}
	// And a real first fragment with that dest port IS blocked.
	first := buildV4(6, [4]byte{10, 0, 0, 2}, [4]byte{8, 8, 8, 8}, 0x2000 /*MF, offset 0*/, make([]byte, 20))
	binary.BigEndian.PutUint16(first[22:24], 80) // dst port in TCP header
	p2 := first
	if tun.ProcessEgressPacket(&p2) {
		t.Fatal("first fragment to a blocked port should be dropped")
	}
}

// IPv6 packets pass through unmodified on both paths (dual-stack support).
func TestProcess_IPv6PassThrough(t *testing.T) {
	tun := newBareTUN()
	v6 := make([]byte, 40)
	v6[0] = 0x60
	orig := make([]byte, len(v6))
	copy(orig, v6)
	p := v6
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("IPv6 egress must pass through")
	}
	if !tun.ProcessIngressPacket(v6) {
		t.Fatal("IPv6 ingress must pass through")
	}
	for i := range orig {
		if v6[i] != orig[i] {
			t.Fatalf("IPv6 packet modified at byte %d", i)
		}
	}
}

// NAT remap must be correct for a /25 network (previously the last octet was
// mishandled for prefixes /25..31).
func TestTransLateIP_Slash25(t *testing.T) {
	tun := newBareTUN()
	_, natNet, _ := net.ParseCIDR("192.168.9.0/24") // clients addressed here
	_, targetNet, _ := net.ParseCIDR("10.10.10.128/25")
	tun.ServerResponse.Networks = []*types.Network{{
		Nat: "192.168.9.0/24", Network: "10.10.10.128/25",
		NatIPNet: natNet, NetIPNet: targetNet,
	}}
	// 192.168.9.5 → network bits of 10.10.10.128/25 (top bit of last octet set)
	// + host bits (5 & 0x7F = 5) => 10.10.10.133.
	got, ok := tun.TransLateIP([4]byte{192, 168, 9, 5})
	if !ok {
		t.Fatal("expected translation")
	}
	want := [4]byte{10, 10, 10, 133}
	if got != want {
		t.Fatalf("/25 NAT wrong: got %v want %v", got, want)
	}
}

// NAT remap for the common /24 and /32 cases is unchanged.
func TestTransLateIP_Slash24And32(t *testing.T) {
	tun := newBareTUN()
	_, n24, _ := net.ParseCIDR("192.168.9.0/24")
	_, t24, _ := net.ParseCIDR("10.0.5.0/24")
	_, n32, _ := net.ParseCIDR("172.16.0.7/32")
	_, t32, _ := net.ParseCIDR("10.9.9.9/32")
	tun.ServerResponse.Networks = []*types.Network{
		{Nat: "192.168.9.0/24", Network: "10.0.5.0/24", NatIPNet: n24, NetIPNet: t24},
		{Nat: "172.16.0.7/32", Network: "10.9.9.9/32", NatIPNet: n32, NetIPNet: t32},
	}
	if got, _ := tun.TransLateIP([4]byte{192, 168, 9, 42}); got != [4]byte{10, 0, 5, 42} {
		t.Fatalf("/24 NAT wrong: got %v", got)
	}
	if got, _ := tun.TransLateIP([4]byte{172, 16, 0, 7}); got != [4]byte{10, 9, 9, 9} {
		t.Fatalf("/32 NAT wrong: got %v", got)
	}
}
