package client

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func buildV4(proto byte, src, dst [4]byte, fragField uint16, l4 []byte) []byte {
	pkt := make([]byte, 20+len(l4))
	pkt[0] = 0x45
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

func TestProcessEgress_TrailingFragmentNotCorrupted(t *testing.T) {
	tun := newBareTUN()

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

func TestProcessEgress_TrailingFragmentIgnoresPortBlock(t *testing.T) {
	tun := newBareTUN()

	tun.blockedPortsSet[[2]byte{0x00, 0x50}] = 80
	payload := []byte{0, 0, 0x00, 0x50, 0, 0, 0, 0}
	pkt := buildV4(6, [4]byte{10, 0, 0, 2}, [4]byte{8, 8, 8, 8}, 100, payload)
	p := pkt
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("trailing fragment must not be port-blocked")
	}

	first := buildV4(6, [4]byte{10, 0, 0, 2}, [4]byte{8, 8, 8, 8}, 0x2000, make([]byte, 20))
	binary.BigEndian.PutUint16(first[22:24], 80)
	p2 := first
	if tun.ProcessEgressPacket(&p2) {
		t.Fatal("first fragment to a blocked port should be dropped")
	}
}

func TestProcessEgress_DropsUDPToWireGuardEndpoint(t *testing.T) {
	tun := newBareTUN()
	tun.wgEndpointSet = true
	tun.serverInterfaceIP4bytes = [4]byte{74, 63, 223, 157}

	for _, payload := range []int{8, 112, 176, 240, 1452} {
		l4 := make([]byte, payload)
		binary.BigEndian.PutUint16(l4[0:2], 51067)
		binary.BigEndian.PutUint16(l4[2:4], 51820)
		pkt := buildV4(17, [4]byte{10, 0, 0, 2}, tun.serverInterfaceIP4bytes, 0, l4)
		p := pkt
		if tun.ProcessEgressPacket(&p) {
			t.Fatalf("UDP length %d to WireGuard endpoint must be dropped to prevent encapsulation loop", payload)
		}
	}
}

func TestProcessEgress_AllowsTCPToWireGuardEndpoint(t *testing.T) {
	tun := newBareTUN()
	tun.wgEndpointSet = true
	tun.serverInterfaceIP4bytes = [4]byte{74, 63, 223, 157}
	l4 := make([]byte, 20)
	binary.BigEndian.PutUint16(l4[2:4], 443)
	pkt := buildV4(6, [4]byte{10, 0, 0, 2}, tun.serverInterfaceIP4bytes, 0, l4)
	p := pkt
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("TCP to the server public IP should still go through the tunnel")
	}
}

func TestProcessEgress_AllowsUDPToOtherHosts(t *testing.T) {
	tun := newBareTUN()
	tun.wgEndpointSet = true
	tun.serverInterfaceIP4bytes = [4]byte{74, 63, 223, 157}
	l4 := make([]byte, 8)
	binary.BigEndian.PutUint16(l4[2:4], 51820)
	pkt := buildV4(17, [4]byte{10, 0, 0, 2}, [4]byte{1, 1, 1, 1}, 0, l4)
	p := pkt
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("UDP to other hosts must not be dropped")
	}
}

func TestProcessEgress_DropsUDPFragmentToEndpoint(t *testing.T) {
	tun := newBareTUN()
	tun.wgEndpointSet = true
	tun.serverInterfaceIP4bytes = [4]byte{74, 63, 223, 157}
	pkt := buildV4(17, [4]byte{10, 0, 0, 2}, tun.serverInterfaceIP4bytes, 185, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	p := pkt
	if tun.ProcessEgressPacket(&p) {
		t.Fatal("UDP fragments to the WireGuard endpoint must be dropped")
	}
}

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

func TestTranslateIP_Slash25(t *testing.T) {
	tun := newBareTUN()
	_, natNet, _ := net.ParseCIDR("192.168.9.0/24")
	_, targetNet, _ := net.ParseCIDR("10.10.10.128/25")
	tun.ServerResponse.Networks = []*types.Network{{
		Nat: "192.168.9.0/24", Network: "10.10.10.128/25",
		NatIPNet: natNet, NetIPNet: targetNet,
	}}

	got, ok := tun.translateIP([4]byte{192, 168, 9, 5})
	if !ok {
		t.Fatal("expected translation")
	}
	want := [4]byte{10, 10, 10, 133}
	if got != want {
		t.Fatalf("/25 NAT wrong: got %v want %v", got, want)
	}
}

func TestTranslateIP_Slash24And32(t *testing.T) {
	tun := newBareTUN()
	_, n24, _ := net.ParseCIDR("192.168.9.0/24")
	_, t24, _ := net.ParseCIDR("10.0.5.0/24")
	_, n32, _ := net.ParseCIDR("172.16.0.7/32")
	_, t32, _ := net.ParseCIDR("10.9.9.9/32")
	tun.ServerResponse.Networks = []*types.Network{
		{Nat: "192.168.9.0/24", Network: "10.0.5.0/24", NatIPNet: n24, NetIPNet: t24},
		{Nat: "172.16.0.7/32", Network: "10.9.9.9/32", NatIPNet: n32, NetIPNet: t32},
	}
	if got, _ := tun.translateIP([4]byte{192, 168, 9, 42}); got != [4]byte{10, 0, 5, 42} {
		t.Fatalf("/24 NAT wrong: got %v", got)
	}
	if got, _ := tun.translateIP([4]byte{172, 16, 0, 7}); got != [4]byte{10, 9, 9, 9} {
		t.Fatalf("/32 NAT wrong: got %v", got)
	}
}

func TestTranslateIP_UnchangedIsFalse(t *testing.T) {
	tun := newBareTUN()
	got, ok := tun.translateIP([4]byte{10, 0, 0, 3})
	if ok || got != [4]byte{10, 0, 0, 3} {
		t.Fatalf("empty networks: got %v ok %v", got, ok)
	}
	_, natNet, _ := net.ParseCIDR("192.168.9.0/24")
	_, targetNet, _ := net.ParseCIDR("10.0.5.0/24")
	tun.ServerResponse.Networks = []*types.Network{{
		Nat: "192.168.9.0/24", Network: "10.0.5.0/24",
		NatIPNet: natNet, NetIPNet: targetNet,
	}}
	got, ok = tun.translateIP([4]byte{10, 0, 0, 9})
	if ok || got != [4]byte{10, 0, 0, 9} {
		t.Fatalf("no match: got %v ok %v", got, ok)
	}
	_, same, _ := net.ParseCIDR("10.0.0.0/24")
	tun.ServerResponse.Networks = []*types.Network{{
		Nat: "10.0.0.0/24", Network: "10.0.0.0/24",
		NatIPNet: same, NetIPNet: same,
	}}
	got, ok = tun.translateIP([4]byte{10, 0, 0, 3})
	if ok || got != [4]byte{10, 0, 0, 3} {
		t.Fatalf("identity nat: got %v ok %v", got, ok)
	}
}

func checksumsMatch(pkt []byte) bool {
	cp := append([]byte(nil), pkt...)
	iphl := int(cp[0]&0x0F) * 4
	RecalculateIPv4HeaderChecksum(cp[:iphl])
	RecalculateTransportChecksum(cp[:iphl], cp[iphl:])
	return bytes.Equal(cp, pkt)
}

func TestProcessEgress_NoNATLeavesChecksums(t *testing.T) {
	tun := newBareTUN()
	l4 := make([]byte, 20)
	pkt := buildV4(6, [4]byte{10, 0, 0, 3}, [4]byte{10, 0, 1, 3}, 0, l4)
	binary.BigEndian.PutUint16(pkt[10:12], 0xABCD)
	binary.BigEndian.PutUint16(pkt[36:38], 0x1234)
	before := append([]byte(nil), pkt...)
	p := pkt
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("packet dropped")
	}
	if !bytes.Equal(pkt, before) {
		t.Fatalf("unchanged flow was rewritten")
	}
}

func TestProcessIngress_NoNATLeavesChecksums(t *testing.T) {
	tun := newBareTUN()
	l4 := make([]byte, 20)
	pkt := buildV4(6, [4]byte{10, 0, 1, 3}, [4]byte{10, 0, 0, 3}, 0, l4)
	binary.BigEndian.PutUint16(pkt[10:12], 0xABCD)
	binary.BigEndian.PutUint16(pkt[36:38], 0x1234)
	before := append([]byte(nil), pkt...)
	if !tun.ProcessIngressPacket(pkt) {
		t.Fatal("packet dropped")
	}
	if !bytes.Equal(pkt, before) {
		t.Fatalf("unchanged flow was rewritten")
	}
}

func TestProcessEgress_NATRewritesChecksum(t *testing.T) {
	tun := newBareTUN()
	_, natNet, _ := net.ParseCIDR("192.168.9.0/24")
	_, targetNet, _ := net.ParseCIDR("10.0.5.0/24")
	tun.ServerResponse.Networks = []*types.Network{{
		Nat: "192.168.9.0/24", Network: "10.0.5.0/24",
		NatIPNet: natNet, NetIPNet: targetNet,
	}}
	l4 := make([]byte, 20)
	pkt := buildV4(6, [4]byte{10, 0, 0, 2}, [4]byte{192, 168, 9, 42}, 0, l4)
	p := pkt
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("packet dropped")
	}
	if pkt[16] != 10 || pkt[17] != 0 || pkt[18] != 5 || pkt[19] != 42 {
		t.Fatalf("dest not translated: %v", pkt[16:20])
	}
	if !checksumsMatch(pkt) {
		t.Fatal("translated packet checksum does not verify")
	}
}
