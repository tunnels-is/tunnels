package wgserver

import (
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/punch"
)

// TestReflexEchoesSourceAndDropsRendezvous ...
func TestReflexEchoesSourceAndDropsRendezvous(t *testing.T) {
	priv, err := punch.NewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	pub, err := punch.Public(priv)
	if err != nil {
		t.Fatal(err)
	}
	setPunchDisco(priv)

	cli, err := punch.NewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	from := netip.MustParseAddrPort("203.0.113.9:40000")
	req, err := punch.BuildReflexReq(id, cli, pub, time.Now(), 9)
	if err != nil {
		t.Fatal(err)
	}

	var gotTo netip.AddrPort
	var got []byte
	setDeliver := func(fn func(netip.AddrPort, []byte) error) func() {
		t.Helper()
		deliverPunchMu.Lock()
		prev := deliverPunch
		deliverPunch = fn
		deliverPunchMu.Unlock()
		return func() {
			deliverPunchMu.Lock()
			deliverPunch = prev
			deliverPunchMu.Unlock()
		}
	}
	restore := setDeliver(func(to netip.AddrPort, pkt []byte) error {
		gotTo = to
		got = append([]byte(nil), pkt...)
		return nil
	})
	defer restore()

	dispatchPunch(from.String(), req)
	if gotTo != from {
		t.Fatalf("echo to %s, want %s", gotTo, from)
	}
	gotID, gotAP, gotNonce, err := punch.OpenReflexResp(cli, pub, got)
	if err != nil {
		t.Fatal(err)
	}
	if gotID != id || gotAP != from || gotNonce != 9 {
		t.Fatalf("echo id=%s addr=%s nonce=%d", gotID, gotAP, gotNonce)
	}

	called := false
	restore()
	restore = setDeliver(func(netip.AddrPort, []byte) error {
		called = true
		return nil
	})
	defer restore()
	old := make([]byte, 48)
	copy(old, []byte{'T', 'N', 'L', 'P', punch.Version, punch.TypeRendezvous})
	dispatchPunch(from.String(), old)
	fwd := make([]byte, 48)
	copy(fwd, []byte{'T', 'N', 'L', 'P', punch.Version, punch.TypeForward})
	dispatchPunch(from.String(), fwd)
	if called {
		t.Fatal("rendezvous or forward was answered")
	}
}
