package wgserver

import (
	"bytes"
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
	req, pend, err := punch.BuildReflexReq(id, cli, pub, time.Now(), 9)
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
	gotAP, err := punch.OpenReflexResp(&pend, got)
	if err != nil {
		t.Fatal(err)
	}
	if gotAP != from || pend.Nonce() != 9 {
		t.Fatalf("echo addr=%s nonce=%d", gotAP, pend.Nonce())
	}
	if bytes.Contains(got, id[:]) {
		t.Fatal("reflex response carries the device UUID in clear")
	}

	// The same request again (same ephemeral, inside its window) is a
	// replay and is not answered.
	got = nil
	dispatchPunch(from.String(), req)
	if got != nil {
		t.Fatal("replayed reflex request was answered")
	}

	// A tampered request (salt no longer matches its keyed check) is not
	// answered.
	tampered := append([]byte(nil), req...)
	tampered[4] ^= 2
	dispatchPunch(from.String(), tampered)
	if got != nil {
		t.Fatal("tampered reflex request was answered")
	}

	// A request from a client that does not hold its claimed disco key
	// (inner static box swapped in from another request) is not answered.
	other, err := punch.NewPrivate()
	if err != nil {
		t.Fatal(err)
	}
	req2, _, err := punch.BuildReflexReq(id, other, pub, time.Now(), 10)
	if err != nil {
		t.Fatal(err)
	}
	mixed := append([]byte(nil), req...)
	copy(mixed[len(mixed)-72:], req2[len(req2)-72:])
	dispatchPunch(from.String(), mixed)
	if got != nil {
		t.Fatal("spliced reflex request was answered")
	}

	called := false
	restore()
	restore = setDeliver(func(netip.AddrPort, []byte) error {
		called = true
		return nil
	})
	defer restore()
	framed := make([]byte, punch.MinPacket)
	copy(framed, []byte{'T', 'N', 'L', 'P', punch.TypeIntro})
	dispatchPunch(from.String(), framed)
	junk := make([]byte, punch.MinPacket)
	copy(junk, []byte{'T', 'N', 'L', 'P', punch.TypePing})
	dispatchPunch(from.String(), junk)
	if called {
		t.Fatal("a framed packet on the underlay was answered")
	}
}

// TestMTUProbeAnsweredWithBoundAck ...
func TestMTUProbeAnsweredWithBoundAck(t *testing.T) {
	var got []byte
	deliverPunchMu.Lock()
	prev := deliverPunch
	deliverPunch = func(_ netip.AddrPort, pkt []byte) error {
		got = append([]byte(nil), pkt...)
		return nil
	}
	deliverPunchMu.Unlock()
	defer func() {
		deliverPunchMu.Lock()
		deliverPunch = prev
		deliverPunchMu.Unlock()
	}()
	probe, err := punch.BuildMTUProbe(1200)
	if err != nil {
		t.Fatal(err)
	}
	dispatchPunch("203.0.113.9:40000", probe)
	if !punch.MTUAckMatches(probe, got) || len(got) != punch.MinPacket {
		t.Fatalf("probe not acked (%d bytes)", len(got))
	}
	ack := got
	got = nil
	dispatchPunch("203.0.113.9:40000", ack)
	if got != nil {
		t.Fatal("an ack was answered")
	}
}

// TestReplayCacheBoundedUnderFlood ...
func TestReplayCacheBoundedUnderFlood(t *testing.T) {
	c := punch.NewReplayCache(16)
	now := time.Now()
	for i := 0; i < 1000; i++ {
		var id [32]byte
		id[0], id[1] = byte(i), byte(i>>8)
		if !c.Fresh(id, now.Add(time.Minute), now) {
			t.Fatal("fresh id rejected")
		}
		if c.Len() > 16 {
			t.Fatalf("cache grew to %d", c.Len())
		}
	}
}
