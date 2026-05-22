package wgserver

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/conn"
)

func TestPinnedBind_OpenAndClose(t *testing.T) {
	b := newPinnedBind(netip.MustParseAddr("127.0.0.1"))
	fns, port, err := b.Open(0) // random port
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	if port == 0 {
		t.Fatal("Open should return the actual port")
	}
	if len(fns) != 1 {
		t.Fatalf("expected 1 receive func, got %d", len(fns))
	}
}

func TestPinnedBind_DoubleOpenRejected(t *testing.T) {
	b := newPinnedBind(netip.MustParseAddr("127.0.0.1"))
	if _, _, err := b.Open(0); err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	if _, _, err := b.Open(0); err != conn.ErrBindAlreadyOpen {
		t.Fatalf("expected ErrBindAlreadyOpen, got %v", err)
	}
}

func TestPinnedBind_ParseEndpoint(t *testing.T) {
	b := newPinnedBind(netip.MustParseAddr("127.0.0.1"))
	ep, err := b.ParseEndpoint("127.0.0.1:51820")
	if err != nil {
		t.Fatal(err)
	}
	stdEP, ok := ep.(*conn.StdNetEndpoint)
	if !ok {
		t.Fatalf("expected *conn.StdNetEndpoint, got %T", ep)
	}
	if stdEP.AddrPort.String() != "127.0.0.1:51820" {
		t.Fatalf("unexpected endpoint: %s", stdEP.AddrPort)
	}
}

func TestPinnedBind_SendReceiveLoopback(t *testing.T) {
	b := newPinnedBind(netip.MustParseAddr("127.0.0.1"))
	fns, port, err := b.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Send a packet from an external UDP socket to the bound port.
	sender, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()

	want := []byte("hello-wg")
	dst := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(port)}
	if _, err := sender.WriteToUDP(want, dst); err != nil {
		t.Fatal(err)
	}

	bufs := [][]byte{make([]byte, 2048)}
	sizes := []int{0}
	eps := []conn.Endpoint{nil}

	done := make(chan error, 1)
	go func() {
		n, err := fns[0](bufs, sizes, eps)
		if err != nil {
			done <- err
			return
		}
		if n != 1 {
			done <- nil
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("receive timed out")
	}

	if string(bufs[0][:sizes[0]]) != string(want) {
		t.Fatalf("payload mismatch: got %q want %q", bufs[0][:sizes[0]], want)
	}
	if eps[0] == nil {
		t.Fatal("endpoint not set")
	}
	stdEP := eps[0].(*conn.StdNetEndpoint)
	senderAddr := sender.LocalAddr().(*net.UDPAddr)
	if int(stdEP.AddrPort.Port()) != senderAddr.Port {
		t.Fatalf("endpoint port mismatch: got %d want %d", stdEP.AddrPort.Port(), senderAddr.Port)
	}
}

func TestPinnedBind_SendToBoundPort(t *testing.T) {
	// Bind to a random port, then Send to a peer that's also on loopback.
	b := newPinnedBind(netip.MustParseAddr("127.0.0.1"))
	_, _, err := b.Open(0)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Set up a receiver to confirm Send actually delivers.
	receiver, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer receiver.Close()
	recvAddr := receiver.LocalAddr().(*net.UDPAddr)

	ep := &conn.StdNetEndpoint{AddrPort: netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(recvAddr.Port))}
	if err := b.Send([][]byte{[]byte("ping")}, ep); err != nil {
		t.Fatal(err)
	}

	_ = receiver.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, _, err := receiver.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "ping" {
		t.Fatalf("got %q want ping", buf[:n])
	}
}

func TestPinnedBind_SendRejectsWrongFamily(t *testing.T) {
	b := newPinnedBind(netip.MustParseAddr("127.0.0.1")) // v4 pinned
	if _, _, err := b.Open(0); err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Try to send to a v6 endpoint.
	ep := &conn.StdNetEndpoint{AddrPort: netip.MustParseAddrPort("[::1]:80")}
	if err := b.Send([][]byte{[]byte("x")}, ep); err == nil {
		t.Fatal("expected family mismatch error")
	}
}

func TestPinnedBind_SendAfterCloseFails(t *testing.T) {
	b := newPinnedBind(netip.MustParseAddr("127.0.0.1"))
	if _, _, err := b.Open(0); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	ep := &conn.StdNetEndpoint{AddrPort: netip.MustParseAddrPort("127.0.0.1:1")}
	if err := b.Send([][]byte{[]byte("x")}, ep); err == nil {
		t.Fatal("expected error sending after close")
	}
}

func TestBuildInnerBind_DefaultWhenNoPublicIP(t *testing.T) {
	bind, err := buildInnerBind(&Config{PublicIP: ""})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bind.(*pinnedBind); ok {
		t.Fatal("empty PublicIP should not yield pinnedBind")
	}
}

func TestBuildInnerBind_PinnedWhenPublicIPSet(t *testing.T) {
	bind, err := buildInnerBind(&Config{PublicIP: "127.0.0.1", WireGuardPort: 1234})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bind.(*pinnedBind); !ok {
		t.Fatalf("expected *pinnedBind, got %T", bind)
	}
}

func TestBuildInnerBind_InvalidPublicIP(t *testing.T) {
	_, err := buildInnerBind(&Config{PublicIP: "not-an-ip"})
	if err == nil {
		t.Fatal("expected error for invalid PublicIP")
	}
}
