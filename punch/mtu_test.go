package punch

import (
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestPingPlainForOuter1500 ...
func TestPingPlainForOuter1500(t *testing.T) {
	plain, ok := PingPlainForOuter(1500, false)
	if !ok {
		t.Fatal("1500 must fit a padded ping")
	}
	a, _ := NewPrivate()
	b, _ := NewPrivate()
	bPub, _ := Public(b)
	pkt, err := BuildPingPad(a, bPub, uuid.New(), uuid.New(), 7, plain)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkt) != ProbePayloadLen(1500, false) {
		t.Fatalf("udp payload %d, want %d", len(pkt), ProbePayloadLen(1500, false))
	}
	if len(pkt) != 1472 {
		t.Fatalf("ipv4 1500 probe is %d bytes, want 1472", len(pkt))
	}
	_, _, txid, err := OpenPing(b, mustPublic(t, a), pkt)
	if err != nil {
		t.Fatal(err)
	}
	if txid != 7 {
		t.Fatalf("txid %d", txid)
	}
}

// TestOpenPingStillAcceptsEightBytes ...
func TestOpenPingStillAcceptsEightBytes(t *testing.T) {
	a, _ := NewPrivate()
	b, _ := NewPrivate()
	bPub, _ := Public(b)
	pkt, err := BuildPing(a, bPub, uuid.New(), uuid.New(), 3)
	if err != nil {
		t.Fatal(err)
	}
	_, _, txid, err := OpenPing(b, mustPublic(t, a), pkt)
	if err != nil {
		t.Fatal(err)
	}
	if txid != 3 {
		t.Fatalf("txid %d", txid)
	}
	short := pkt[:len(pkt)-1]
	if _, _, _, err := OpenPing(b, mustPublic(t, a), short); err == nil {
		t.Fatal("truncated ping must fail")
	}
}

// TestMTUProbeAck ...
func TestMTUProbeAck(t *testing.T) {
	payload := ProbePayloadLen(1500, false)
	pkt, err := BuildMTUProbe(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkt) != payload {
		t.Fatalf("probe %d", len(pkt))
	}
	ack := MTUAckFor(pkt)
	if !MTUAckMatches(pkt, ack) {
		t.Fatal("ack")
	}
	if len(ack) != MinPacket {
		t.Fatalf("ack %d bytes, want bucket %d", len(ack), MinPacket)
	}
	if MTUAckFor(ack) != nil {
		t.Fatal("ack must not ack an ack")
	}
	other, err := BuildMTUProbe(payload)
	if err != nil {
		t.Fatal(err)
	}
	if MTUAckMatches(other, ack) {
		t.Fatal("ack must be bound to its probe")
	}
	if _, err := BuildMTUProbe(MinPacket - 1); err == nil {
		t.Fatal("short probe")
	}
}

// TestEngineDiscoversFullPath ...
func TestEngineDiscoversFullPath(t *testing.T) {
	a, b, now := newMTUPair(t)
	a.SetDiscoverMTU(true)
	b.SetDiscoverMTU(true)
	aMTU, bMTU := runMTUExchange(t, a, b, now, func(pkt []byte) bool { return true })
	if aMTU != 1440 || bMTU != 1440 {
		t.Fatalf("inner mtu a=%d b=%d, want 1440", aMTU, bMTU)
	}
}

// TestEngineIgnoredProbeLeavesPathUp ...
func TestEngineIgnoredProbeLeavesPathUp(t *testing.T) {
	a, b, now := newMTUPair(t)
	a.SetDiscoverMTU(true)
	runMTUExchange(t, a, b, now, func(pkt []byte) bool {
		return len(pkt) <= MinPacket
	})
	if _, ok := a.Direct(netip.MustParseAddr("10.0.1.8")); !ok {
		t.Fatal("ignored mtu probe took the direct path down")
	}
	deadline := now.Add(40 * time.Second)
	for now.Before(deadline) {
		now = now.Add(MTUProbeTimeout)
		res := a.Poll(now)
		if len(res.MTU) != 0 {
			t.Fatalf("unanswered search published mtu %+v", res.MTU)
		}
		for _, d := range res.Sends {
			if d.MTU {
				continue
			}
			if d.Type == TypePing {
				back := b.OnPacket(netip.MustParseAddrPort("198.51.100.10:41000"), d.Data, now)
				for _, p := range back.Sends {
					if p.Type == TypePong {
						a.OnPacket(netip.MustParseAddrPort("198.51.100.20:42000"), p.Data, now)
					}
				}
			}
		}
		if _, ok := a.Direct(netip.MustParseAddr("10.0.1.8")); !ok {
			t.Fatal("path dropped while the padded probe was ignored")
		}
	}
}

func mustPublic(t *testing.T, priv [32]byte) [32]byte {
	t.Helper()
	pub, err := Public(priv)
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func newMTUPair(t *testing.T) (a, b *Engine, now time.Time) {
	t.Helper()
	aPriv, _ := NewPrivate()
	bPriv, _ := NewPrivate()
	srvPriv, _ := NewPrivate()
	srvPub, _ := Public(srvPriv)
	var aWG, bWG [32]byte
	aWG[0] = 1
	bWG[0] = 2
	aID, bID := uuid.New(), uuid.New()
	aIP := netip.MustParseAddr("10.0.0.5")
	bIP := netip.MustParseAddr("10.0.1.8")
	a = NewEngine(aID, aPriv, aWG, aIP)
	b = NewEngine(bID, bPriv, bWG, bIP)
	a.SetServerDisco(srvPub)
	b.SetServerDisco(srvPub)
	a.SetLocalPolicy(false, false, []string{bIP.String()})
	b.SetLocalPolicy(false, false, []string{aIP.String()})
	now = time.Now()
	aReflex := netip.MustParseAddrPort("198.51.100.10:41000")
	bReflex := netip.MustParseAddrPort("198.51.100.20:42000")
	deliverReflex(t, a, srvPriv, aID, aReflex, a.Poll(now), now)
	deliverReflex(t, b, srvPriv, bID, bReflex, b.Poll(now), now)
	a.NoteTraffic(bIP)
	b.NoteTraffic(aIP)
	return a, b, now
}

func runMTUExchange(t *testing.T, a, b *Engine, now time.Time, accept func([]byte) bool) (aMTU, bMTU int) {
	t.Helper()
	type tagged struct {
		from string
		d    Datagram
	}
	q := []tagged{}
	push := func(from string, res Result) {
		for _, m := range res.MTU {
			if from == "a" && m.Inner > aMTU {
				aMTU = m.Inner
			}
			if from == "b" && m.Inner > bMTU {
				bMTU = m.Inner
			}
		}
		for _, d := range res.Sends {
			if !accept(d.Data) {
				continue
			}
			q = append(q, tagged{from, d})
		}
	}
	now = now.Add(time.Second)
	push("a", a.Poll(now))
	push("b", b.Poll(now))
	other := map[string]*Engine{"a": b, "b": a}
	selfAddr := map[string]netip.AddrPort{
		"a": netip.MustParseAddrPort("198.51.100.10:41000"),
		"b": netip.MustParseAddrPort("198.51.100.20:42000"),
	}
	overlay := map[string]netip.Addr{
		"a": netip.MustParseAddr("10.0.0.5"),
		"b": netip.MustParseAddr("10.0.1.8"),
	}
	flip := map[string]string{"a": "b", "b": "a"}
	for steps := 0; steps < 80 && len(q) > 0; steps++ {
		item := q[0]
		q = q[1:]
		if item.d.Server {
			continue
		}
		from := selfAddr[item.from]
		if item.d.Tunnel {
			from = netip.AddrPortFrom(overlay[item.from], IntroPort)
		}
		res := other[item.from].OnPacket(from, item.d.Data, now)
		push(flip[item.from], res)
	}
	bIP := netip.MustParseAddr("10.0.1.8")
	aIP := netip.MustParseAddr("10.0.0.5")
	if _, ok := a.Direct(bIP); !ok {
		t.Fatal("A did not promote")
	}
	if _, ok := b.Direct(aIP); !ok {
		t.Fatal("B did not promote")
	}
	return aMTU, bMTU
}
