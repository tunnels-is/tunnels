package client

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/punch"
)

// TestResolvedMTUKeepsConfigured ...
func TestResolvedMTUKeepsConfigured(t *testing.T) {
	meta := &TunnelMeta{MTU: 1320}
	if got := resolvedMTU(meta, "192.0.2.1", "51820"); got != 1320 {
		t.Fatalf("configured mtu %d", got)
	}
	if meta.MTU != 1320 {
		t.Fatal("configured mtu was rewritten")
	}
}

// TestDiscoverPathMTULoopback ...
func TestDiscoverPathMTULoopback(t *testing.T) {
	pc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	go func() {
		buf := make([]byte, 2048)
		for {
			_ = pc.SetReadDeadline(time.Now().Add(3 * time.Second))
			n, addr, err := pc.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if ack := punch.MTUAckFor(buf[:n]); ack != nil {
				_, _ = pc.WriteToUDP(ack, addr)
			}
		}
	}()
	inner, err := discoverPathMTU(pc.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	if inner != 1440 {
		t.Fatalf("loopback inner %d, want 1440", inner)
	}
}

// TestPeerMTUClampsSYNAndDropsUDP ...
func TestPeerMTUClampsSYNAndDropsUDP(t *testing.T) {
	tun := newBareTUN()
	peer := [4]byte{10, 0, 1, 8}
	tun.punchMTU = map[[4]byte]int{peer: 1280}

	tcp := make([]byte, 24)
	tcp[12] = 6 << 4
	tcp[13] = 0x02
	tcp[20] = 2
	tcp[21] = 4
	binary.BigEndian.PutUint16(tcp[22:24], 1460)
	syn := buildV4(6, [4]byte{10, 0, 0, 4}, peer, 0, tcp)
	p := syn
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("syn should be forwarded")
	}
	if got := binary.BigEndian.Uint16(syn[42:44]); got != 1240 {
		t.Fatalf("mss %d, want 1240", got)
	}

	udp := buildV4(17, [4]byte{10, 0, 0, 4}, peer, 0, make([]byte, 1300))
	p = udp
	if tun.ProcessEgressPacket(&p) {
		t.Fatal("udp larger than the peer mtu must be dropped")
	}
	small := buildV4(17, [4]byte{10, 0, 0, 4}, peer, 0, make([]byte, 8))
	p = small
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("small udp must pass")
	}

	data := make([]byte, 1300)
	data[12] = 5 << 4
	data[13] = 0x10
	ack := buildV4(6, [4]byte{10, 0, 0, 4}, peer, 0, data)
	p = ack
	if !tun.ProcessEgressPacket(&p) {
		t.Fatal("tcp data must not be dropped")
	}
}
