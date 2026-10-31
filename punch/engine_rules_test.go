package punch

import (
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEngine_RetypeAndReflectionFail(t *testing.T) {
	aPriv := mustKey(t)
	bPriv := mustKey(t)
	aPub, bPub := mustPublic(t, aPriv), mustPublic(t, bPriv)
	aID, bID := uuid.New(), uuid.New()
	ping, err := BuildPing(aPriv, bPub, aID, bID, 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := OpenPing(bPriv, aPub, ping); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := OpenTeardown(bPriv, aPub, ping); err == nil {
		t.Fatal("a ping opened as a teardown")
	}
	if _, _, _, err := OpenPing(aPriv, bPub, ping); err == nil {
		t.Fatal("a reflected ping opened")
	}
}

func TestEngine_PongMustMatchPing(t *testing.T) {
	aPriv := mustKey(t)
	aPub := mustPublic(t, aPriv)
	var aWG [32]byte
	aWG[0] = 1
	aID, bID := uuid.New(), uuid.New()
	aIP := netip.MustParseAddr("10.0.0.5")
	bIP := netip.MustParseAddr("10.0.1.8")
	b := NewEngine(bID, mustKey(t), [32]byte{2}, bIP)
	b.SetLocalPolicy(false, false, []string{aIP.String()})
	current := netip.MustParseAddrPort("198.51.100.10:41000")
	other := netip.MustParseAddrPort("198.51.100.20:42000")
	now := time.Now()
	b.NoteTraffic(aIP)
	pkt, err := BuildIntro(aID, aPub, aWG, aIP, []netip.AddrPort{current})
	if err != nil {
		t.Fatal(err)
	}
	if pings := pingTargets(b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), pkt, now)); len(pings) != 1 {
		t.Fatalf("intro pings %v", pings)
	}
	wrongFrom, err := BuildPong(aPriv, mustPublic(t, b.disco), aID, bID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if up := b.OnPacket(other, wrongFrom, now); len(up.Up) != 0 {
		t.Fatal("pong from a different address promoted a path")
	}
	stale, err := BuildPong(aPriv, mustPublic(t, b.disco), aID, bID, 99)
	if err != nil {
		t.Fatal(err)
	}
	if up := b.OnPacket(current, stale, now); len(up.Up) != 0 {
		t.Fatal("unmatched txid promoted a path")
	}
	match, err := BuildPong(aPriv, mustPublic(t, b.disco), aID, bID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if up := b.OnPacket(current, match, now); len(up.Up) != 0 {
		t.Fatal("one pong promoted a path")
	}
}

func TestEngine_TeardownReplayAndReflexSource(t *testing.T) {
	aPriv := mustKey(t)
	aPub := mustPublic(t, aPriv)
	bPriv := mustKey(t)
	bPub := mustPublic(t, bPriv)
	var aWG [32]byte
	aWG[0] = 1
	aID, bID := uuid.New(), uuid.New()
	aIP := netip.MustParseAddr("10.0.0.5")
	bIP := netip.MustParseAddr("10.0.1.8")
	b := NewEngine(bID, bPriv, [32]byte{2}, bIP)
	b.SetLocalPolicy(false, false, []string{aIP.String()})
	current := netip.MustParseAddrPort("198.51.100.10:41000")
	now := time.Now()
	b.NoteTraffic(aIP)
	pkt, err := BuildIntro(aID, aPub, aWG, aIP, []netip.AddrPort{current})
	if err != nil {
		t.Fatal(err)
	}
	b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), pkt, now)
	for txid := uint64(1); txid <= 2; txid++ {
		pong, err := BuildPong(aPriv, bPub, aID, bID, txid)
		if err != nil {
			t.Fatal(err)
		}
		b.OnPacket(current, pong, now)
	}
	if _, ok := b.Direct(aIP); !ok {
		t.Fatal("path did not come up")
	}
	td, err := BuildTeardown(aPriv, bPub, aID, bID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if down := b.OnPacket(current, td, now); len(down.Down) != 1 {
		t.Fatalf("teardown %+v", down.Down)
	}
	if again := b.OnPacket(current, td, now); len(again.Down) != 0 {
		t.Fatal("replayed teardown was accepted")
	}

	srvPriv := mustKey(t)
	srvPub := mustPublic(t, srvPriv)
	b.SetServerDisco(srvPub)
	b.SetServerEndpoint(testHub)
	res := b.Poll(now.Add(time.Second))
	var req []byte
	for _, d := range res.Sends {
		if d.Server && d.Type == TypeReflexReq {
			req = d.Data
		}
	}
	if req == nil {
		t.Fatal("no reflex request")
	}
	rq, err := OpenReflexReq(srvPriv, req, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if rq.Device != bID {
		t.Fatal("reflex device mismatch")
	}
	resp, err := BuildReflexResp(&rq, current)
	if err != nil {
		t.Fatal(err)
	}
	b.OnPacket(netip.MustParseAddrPort("203.0.113.9:51820"), resp, now.Add(time.Second))
	b.mu.Lock()
	learned := b.reflex.IsValid()
	b.mu.Unlock()
	if learned {
		t.Fatal("reflex response from a different host was stored")
	}
	b.OnPacket(testHub, resp, now.Add(time.Second))
	b.mu.Lock()
	learned = b.reflex == current
	b.mu.Unlock()
	if !learned {
		t.Fatal("reflex response from the hub was dropped")
	}
	b.OnPacket(testHub, resp, now.Add(2*time.Second))
	b.mu.Lock()
	wait := b.reflexWait
	b.mu.Unlock()
	if wait {
		t.Fatal("replayed reflex response was still outstanding")
	}
}

func TestEngine_OtherAddressDoesNotKeepDirectAlive(t *testing.T) {
	aPriv := mustKey(t)
	aPub := mustPublic(t, aPriv)
	bPriv := mustKey(t)
	bPub := mustPublic(t, bPriv)
	var aWG [32]byte
	aWG[0] = 1
	aID, bID := uuid.New(), uuid.New()
	aIP := netip.MustParseAddr("10.0.0.5")
	bIP := netip.MustParseAddr("10.0.1.8")
	b := NewEngine(bID, bPriv, [32]byte{2}, bIP)
	b.SetLocalPolicy(false, false, []string{aIP.String()})
	current := netip.MustParseAddrPort("198.51.100.10:41000")
	other := netip.MustParseAddrPort("198.51.100.20:42000")
	now := time.Now()
	b.NoteTraffic(aIP)
	pkt, err := BuildIntro(aID, aPub, aWG, aIP, []netip.AddrPort{current, other})
	if err != nil {
		t.Fatal(err)
	}
	b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), pkt, now)
	for _, txid := range []uint64{1, 3} {
		pong, err := BuildPong(aPriv, bPub, aID, bID, txid)
		if err != nil {
			t.Fatal(err)
		}
		b.OnPacket(current, pong, now)
	}
	if _, ok := b.Direct(aIP); !ok {
		t.Fatal("direct path did not come up")
	}
	late, err := BuildPong(aPriv, bPub, aID, bID, 2)
	if err != nil {
		t.Fatal(err)
	}
	b.OnPacket(other, late, now.Add(20*time.Second))
	down := b.Poll(now.Add(DirectTimeout + time.Second))
	if len(down.Down) != 1 {
		t.Fatal("a pong on another address kept the direct path up")
	}
	if _, ok := b.Direct(aIP); ok {
		t.Fatal("direct path stayed up past its own timeout")
	}
}

func TestEngine_FreshPingsDoNotGrowCandidates(t *testing.T) {
	aPriv := mustKey(t)
	aPub := mustPublic(t, aPriv)
	bPriv := mustKey(t)
	var aWG [32]byte
	aWG[0] = 1
	aID := uuid.New()
	aIP := netip.MustParseAddr("10.0.0.5")
	bIP := netip.MustParseAddr("10.0.1.8")
	b := NewEngine(uuid.New(), bPriv, [32]byte{2}, bIP)
	b.SetLocalPolicy(false, false, []string{aIP.String()})
	known := netip.MustParseAddrPort("198.51.100.10:41000")
	now := time.Now()
	b.NoteTraffic(aIP)
	intro, err := BuildIntro(aID, aPub, aWG, aIP, []netip.AddrPort{known})
	if err != nil {
		t.Fatal(err)
	}
	b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), intro, now)
	b.mu.Lock()
	s := b.peers[aID]
	b.mu.Unlock()
	if s == nil || len(s.cands) != 1 {
		t.Fatal("intro candidate missing")
	}
	for i := 0; i < 32; i++ {
		ping, err := BuildPing(aPriv, mustPublic(t, b.disco), aID, b.self, uint64(100+i))
		if err != nil {
			t.Fatal(err)
		}
		from := netip.MustParseAddrPort("203.0.113.10:1")
		from = netip.AddrPortFrom(from.Addr(), uint16(1000+i))
		b.OnPacket(from, ping, now)
	}
	b.mu.Lock()
	n := len(s.cands)
	b.mu.Unlock()
	if n != 1 {
		t.Fatalf("fresh pings grew candidates to %d", n)
	}
}
