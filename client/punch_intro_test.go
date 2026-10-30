package client

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/punch"
)

// TestTakePunchIntroDemux ...
func TestTakePunchIntroDemux(t *testing.T) {
	priv, err := punch.NewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	pub, err := punch.Public(priv)
	if err != nil {
		t.Fatal(err)
	}
	src := netip.MustParseAddr("10.0.0.5")
	dst := netip.MustParseAddr("10.0.1.8")
	body, err := punch.BuildIntro(uuid.New(), pub, pub, src, []netip.AddrPort{netip.MustParseAddrPort("198.51.100.10:41000")})
	if err != nil {
		t.Fatal(err)
	}
	pkt, err := punch.WrapIPv4UDP(src, dst, punch.IntroPort, punch.IntroPort, body)
	if err != nil {
		t.Fatal(err)
	}
	hdr := append([]byte(nil), pkt[:20]...)
	RecalculateIPv4HeaderChecksum(pkt[:20])
	if !bytes.Equal(hdr, pkt[:20]) {
		t.Fatal("intro header checksum drifted")
	}

	tun := &TUN{punchIn: make(chan punchDatagram, 1)}
	if !tun.takePunchIntro(pkt) {
		t.Fatal("intro was not demuxed")
	}
	got := <-tun.punchIn
	if !bytes.Equal(got.data, body) {
		t.Fatal("intro payload was not preserved")
	}
	if got.from != netip.AddrPortFrom(src, punch.IntroPort).String() {
		t.Fatalf("intro from %s", got.from)
	}

	other, err := punch.WrapIPv4UDP(src, dst, 9, 80, []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if tun.takePunchIntro(other) {
		t.Fatal("ordinary UDP was treated as an introduction")
	}
}
