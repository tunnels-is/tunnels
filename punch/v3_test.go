package punch

import (
	"bytes"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
)

func assertNoClear(t *testing.T, what string, pkt []byte, secrets map[string][]byte) {
	t.Helper()
	for name, s := range secrets {
		if bytes.Contains(pkt, s) {
			t.Fatalf("%s carries %s in clear", what, name)
		}
	}
}

type v3Pair struct {
	a, b         *Engine
	aPriv, bPriv [32]byte
	aID, bID     uuid.UUID
	aIP, bIP     netip.Addr
	aRef, bRef   netip.AddrPort
	wire         [][]byte
}

// punchPair runs two engines through intro, ping and pong until both have
// a direct path, recording every underlay packet.
func punchPair(t *testing.T, now time.Time) *v3Pair {
	t.Helper()
	p := &v3Pair{
		aPriv: mustKey(t), bPriv: mustKey(t),
		aID: uuid.New(), bID: uuid.New(),
		aIP: netip.MustParseAddr("10.0.0.5"), bIP: netip.MustParseAddr("10.0.1.8"),
		aRef: netip.MustParseAddrPort("198.51.100.10:41000"),
		bRef: netip.MustParseAddrPort("198.51.100.20:42000"),
	}
	srvPriv := mustKey(t)
	srvPub := mustPublic(t, srvPriv)
	p.a = NewEngine(p.aID, p.aPriv, [32]byte{1}, p.aIP)
	p.b = NewEngine(p.bID, p.bPriv, [32]byte{2}, p.bIP)
	p.a.SetServerDisco(srvPub)
	p.b.SetServerDisco(srvPub)
	p.a.SetLocalPolicy(false, false, []string{p.bIP.String()})
	p.b.SetLocalPolicy(false, false, []string{p.aIP.String()})
	deliverReflex(t, p.a, srvPriv, p.aID, p.aRef, p.a.Poll(now), now)
	deliverReflex(t, p.b, srvPriv, p.bID, p.bRef, p.b.Poll(now), now)
	p.a.NoteTraffic(p.bIP)
	p.b.NoteTraffic(p.aIP)
	now = now.Add(time.Second)
	type item struct {
		from string
		d    Datagram
	}
	var q []item
	push := func(from string, r Result) {
		for _, d := range r.Sends {
			q = append(q, item{from, d})
		}
	}
	push("a", p.a.Poll(now))
	push("b", p.b.Poll(now))
	other := map[string]*Engine{"a": p.b, "b": p.a}
	self := map[string]netip.AddrPort{"a": p.aRef, "b": p.bRef}
	overlay := map[string]netip.Addr{"a": p.aIP, "b": p.bIP}
	flip := map[string]string{"a": "b", "b": "a"}
	for steps := 0; steps < 64 && len(q) > 0; steps++ {
		it := q[0]
		q = q[1:]
		if it.d.Server {
			continue
		}
		from := self[it.from]
		if it.d.Tunnel {
			from = netip.AddrPortFrom(overlay[it.from], IntroPort)
		} else {
			p.wire = append(p.wire, it.d.Data)
		}
		push(flip[it.from], other[it.from].OnPacket(from, it.d.Data, now))
	}
	if _, ok := p.a.Direct(p.bIP); !ok {
		t.Fatal("A did not promote a direct path")
	}
	if _, ok := p.b.Direct(p.aIP); !ok {
		t.Fatal("B did not promote a direct path")
	}
	return p
}

func TestV3_DirectPacketsCarryNoIdentifiers(t *testing.T) {
	p := punchPair(t, time.Now())
	aPub, bPub := mustPublic(t, p.aPriv), mustPublic(t, p.bPriv)
	p.a.mu.Lock()
	sa := p.a.peers[p.bID]
	aEph, haveSess := sa.ephPub, sa.haveSess
	p.a.mu.Unlock()
	p.b.mu.Lock()
	bEph := p.b.peers[p.aID].ephPub
	p.b.mu.Unlock()
	if !haveSess {
		t.Fatal("no ephemeral session after the exchange")
	}
	secrets := map[string][]byte{
		"A uuid": p.aID[:], "B uuid": p.bID[:],
		"A disco pub": aPub[:], "B disco pub": bPub[:],
		"A session eph": aEph[:], "B session eph": bEph[:],
	}
	if len(p.wire) < 4 {
		t.Fatalf("only %d direct packets", len(p.wire))
	}
	seenMasked := map[[32]byte]bool{}
	for _, pkt := range p.wire {
		assertNoClear(t, "direct packet", pkt, secrets)
		var m [32]byte
		copy(m[:], pkt[tagLen:])
		if seenMasked[m] {
			t.Fatal("masked ephemeral repeated across packets")
		}
		seenMasked[m] = true
	}
	td, err := BuildTeardown(p.aPriv, bPub, p.aID, p.bID, 1)
	if err != nil {
		t.Fatal(err)
	}
	assertNoClear(t, "teardown", td, secrets)
}

func TestV3_TagsRotateAndDifferPerDirection(t *testing.T) {
	aPriv, bPriv := mustKey(t), mustKey(t)
	aPub, bPub := mustPublic(t, aPriv), mustPublic(t, bPriv)
	aID, bID := uuid.New(), uuid.New()
	now := time.Unix(1_800_000_000, 0)
	ka, _ := derivePair(aPriv, bPub)
	kb, _ := derivePair(bPriv, aPub)
	if ka.tagSend != kb.tagRecv || ka.hpSend != kb.hpRecv || ka.bind != kb.bind || ka.tagSend == ka.tagRecv {
		t.Fatal("pair keys do not mirror")
	}
	seen := map[[tagLen]byte]bool{}
	for w := int64(0); w < 8; w++ {
		for seq := 0; seq < staticSeqs; seq++ {
			tag, _ := slotTag(ka.tagSend, w, seq)
			back, _ := slotTag(ka.tagRecv, w, seq)
			if seen[tag] || seen[back] || tag == back {
				t.Fatal("tag repeated across slots or directions")
			}
			seen[tag], seen[back] = true, true
		}
	}
	p1, _ := buildSealedAt(TypePing, aPriv, bPub, aID, bID, make([]byte, 8), now, false)
	if _, _, _, err := openSealedAt(TypePing, bPriv, aPub, p1, now.Add(tagWindow)); err != nil {
		t.Fatalf("one window of skew rejected: %v", err)
	}
	if _, _, _, err := openSealedAt(TypePing, bPriv, aPub, p1, now.Add(-2*tagWindow)); err != nil {
		t.Fatalf("two windows of skew rejected: %v", err)
	}
	if _, _, _, err := openSealedAt(TypePing, bPriv, aPub, p1, now.Add(time.Duration(tagSlack+1)*tagWindow)); err == nil {
		t.Fatal("tag far outside the window opened")
	}
}

func lookupPeer(e *Engine, pkt []byte, now time.Time) *session {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, _, ok := e.lookupTagLocked(pkt, now)
	if !ok {
		return nil
	}
	return s
}

func firstSlotTag(t *testing.T, key [32]byte, w int64) [tagLen]byte {
	t.Helper()
	for seq := 0; seq < staticSeqs; seq++ {
		if tag, ok := slotTag(key, w, seq); ok {
			return tag
		}
	}
	t.Fatal("no usable slot")
	return [tagLen]byte{}
}

func TestV3_WrongKeyOrTagRejected(t *testing.T) {
	aPriv, bPriv, cPriv := mustKey(t), mustKey(t), mustKey(t)
	aPub, bPub := mustPublic(t, aPriv), mustPublic(t, bPriv)
	aID, bID := uuid.New(), uuid.New()
	ping, err := BuildPing(aPriv, bPub, aID, bID, 5)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := OpenPing(bPriv, aPub, ping); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := OpenPing(cPriv, aPub, ping); err == nil {
		t.Fatal("third key opened a ping")
	}
	forged, _ := BuildPing(cPriv, bPub, aID, bID, 5)
	if _, _, _, err := OpenPing(bPriv, aPub, forged); err == nil {
		t.Fatal("ping from another disco key claiming A opened")
	}
	for _, off := range []int{0, tagLen, tagLen + keyLen, len(ping) - 1} {
		bad := append([]byte(nil), ping...)
		bad[off] ^= 1
		if _, _, _, err := OpenPing(bPriv, aPub, bad); err == nil {
			t.Fatalf("flip at %d opened", off)
		}
	}

	// Engine: B knows A from an intro. Packets from C, or with a foreign tag,
	// never reach a key; a valid tag with a broken box does nothing.
	aIP := netip.MustParseAddr("10.0.0.5")
	b := NewEngine(bID, bPriv, [32]byte{2}, netip.MustParseAddr("10.0.1.8"))
	b.SetLocalPolicy(false, false, []string{aIP.String()})
	b.NoteTraffic(aIP)
	now := time.Now()
	cand := netip.MustParseAddrPort("198.51.100.10:41000")
	intro, _ := BuildIntro(aID, aPub, [32]byte{1}, aIP, []netip.AddrPort{cand})
	b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), intro, now)
	if s := lookupPeer(b, ping, now); s == nil || s.peer.DeviceID != aID {
		t.Fatal("tag lookup did not find A")
	}
	if lookupPeer(b, forged, now) != nil {
		t.Fatal("tag from C resolved to a peer")
	}
	for _, pkt := range [][]byte{forged} {
		if r := b.OnPacket(cand, pkt, now); len(r.Sends) != 0 {
			t.Fatal("forged ping was answered")
		}
	}
	broken := append([]byte(nil), ping...)
	broken[len(broken)-1] ^= 1
	if r := b.OnPacket(cand, broken, now); len(r.Sends) != 0 {
		t.Fatal("broken ping was answered")
	}
	if r := b.OnPacket(cand, ping, now); len(r.Sends) == 0 {
		t.Fatal("valid ping was not answered")
	}
	future, _ := buildSealedAt(TypePing, aPriv, bPub, aID, bID, make([]byte, 8), now.Add(10*time.Minute), false)
	if r := b.OnPacket(cand, future, now); len(r.Sends) != 0 {
		t.Fatal("ping with an out-of-window tag was answered")
	}
	wrongDst, _ := BuildPing(aPriv, bPub, aID, uuid.New(), 6)
	if r := b.OnPacket(cand, wrongDst, now); len(r.Sends) != 0 {
		t.Fatal("ping for another device was answered")
	}
	wrongSrc, _ := BuildPing(aPriv, bPub, uuid.New(), bID, 7)
	if r := b.OnPacket(cand, wrongSrc, now); len(r.Sends) != 0 {
		t.Fatal("ping naming another sender was answered")
	}
}

func TestV3_TagMapFollowsWindowAndSessions(t *testing.T) {
	aPriv, bPriv := mustKey(t), mustKey(t)
	aPub, bPub := mustPublic(t, aPriv), mustPublic(t, bPriv)
	aID, bID := uuid.New(), uuid.New()
	aIP := netip.MustParseAddr("10.0.0.5")
	b := NewEngine(bID, bPriv, [32]byte{2}, netip.MustParseAddr("10.0.1.8"))
	b.SetLocalPolicy(false, false, []string{aIP.String()})
	now := time.Now()
	intro, _ := BuildIntro(aID, aPub, [32]byte{1}, aIP, nil)
	b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), intro, now)
	later := now.Add(30 * time.Minute)
	pkt, _ := buildSealedAt(TypePing, aPriv, bPub, aID, bID, make([]byte, 8), later, false)
	if lookupPeer(b, pkt, now) != nil {
		t.Fatal("future tag resolved")
	}
	if lookupPeer(b, pkt, later) == nil {
		t.Fatal("tag map did not roll to the new window")
	}
	b.SetLocalPolicy(false, false, nil)
	if lookupPeer(b, pkt, later) != nil {
		t.Fatal("dropped session still resolves")
	}
	b.mu.Lock()
	n := len(b.tags)
	b.mu.Unlock()
	if n != 0 {
		t.Fatalf("tag map kept %d entries", n)
	}
	self, _ := BuildIntro(uuid.New(), mustPublic(t, bPriv), [32]byte{3}, aIP, nil)
	b.SetLocalPolicy(true, false, nil)
	b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), self, now)
	b.mu.Lock()
	n = len(b.peers)
	b.mu.Unlock()
	if n != 0 {
		t.Fatal("intro carrying our own disco key made a session")
	}
}

func TestV3_SessionKeyNeedsStaticBinding(t *testing.T) {
	aPriv, bPriv := mustKey(t), mustKey(t)
	aPub, bPub := mustPublic(t, aPriv), mustPublic(t, bPriv)
	aID, bID := uuid.New(), uuid.New()
	aIP := netip.MustParseAddr("10.0.0.5")
	cand := netip.MustParseAddrPort("198.51.100.10:41000")
	now := time.Now()
	setup := func() (*Engine, *session) {
		b := NewEngine(bID, bPriv, [32]byte{2}, netip.MustParseAddr("10.0.1.8"))
		b.SetLocalPolicy(false, false, []string{aIP.String()})
		b.NoteTraffic(aIP)
		intro, _ := BuildIntro(aID, aPub, [32]byte{1}, aIP, []netip.AddrPort{cand})
		if pings := pingTargets(b.OnPacket(netip.AddrPortFrom(aIP, IntroPort), intro, now)); len(pings) != 1 {
			t.Fatal("B did not ping A")
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		s := b.peers[aID]
		if s.haveSess || !s.ephLive {
			t.Fatal("B should be waiting for A's ephemeral")
		}
		return b, s
	}
	// Worst case: an attacker who somehow knows B's session ephemeral, the
	// header-protection key and the tag key tries a first-flight ee packet.
	forge := func(b *Engine, s *session, bind [32]byte) []byte {
		b.mu.Lock()
		bEph, hp, tagKey := s.ephPub, s.static.hpRecv, s.static.tagRecv
		b.mu.Unlock()
		evil := mustKey(t)
		k, err := deriveEE(evil, bEph, aPub, bPub, bind)
		if err != nil {
			t.Fatal(err)
		}
		out := directOut{key: k.send, hp: hp, tag: firstSlotTag(t, tagKey, window(now))}
		var body [8]byte
		body[7] = 9
		pkt, err := sealDirect(TypePing, out, mustPublic(t, evil), aID, bID, body[:], false)
		if err != nil {
			t.Fatal(err)
		}
		return pkt
	}
	b, s := setup()
	if r := b.OnPacket(cand, forge(b, s, [32]byte{}), now); len(r.Sends) != 0 {
		t.Fatal("ee ping without the static binding was answered")
	}
	b.mu.Lock()
	hijacked := s.haveSess
	b.mu.Unlock()
	if hijacked {
		t.Fatal("unbound ee ping installed a session key")
	}
	// Control: with the binding (i.e. holding the static DH) it works, so
	// the binding is what stops the forgery above.
	b, s = setup()
	b.mu.Lock()
	bind := s.static.bind
	b.mu.Unlock()
	if r := b.OnPacket(cand, forge(b, s, bind), now); len(r.Sends) == 0 {
		t.Fatal("bound ee ping was not answered")
	}
}

func TestV3_EngineTeardownEpochAndReplay(t *testing.T) {
	p := punchPair(t, time.Now())
	now := time.Now().Add(2 * time.Second)
	p.a.mu.Lock()
	sa := p.a.peers[p.bID]
	td, err := p.a.teardownLocked(sa, now)
	p.a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if r := p.b.OnPacket(p.aRef, td, now); len(r.Down) != 1 {
		t.Fatal("session teardown not accepted")
	}
	if _, ok := p.b.Direct(p.aIP); ok {
		t.Fatal("path survived teardown")
	}
	p.b.mu.Lock()
	sb := p.b.peers[p.aID]
	sb.directUp = true
	p.b.mu.Unlock()
	if r := p.b.OnPacket(p.aRef, td, now); len(r.Down) != 0 {
		t.Fatal("replayed teardown accepted")
	}
}

func TestV3_ReflexCarriesNoIdentifiers(t *testing.T) {
	srvPriv := mustKey(t)
	srvPub := mustPublic(t, srvPriv)
	cli := mustKey(t)
	cliPub := mustPublic(t, cli)
	dev := uuid.New()
	now := time.Now()
	obs := netip.MustParseAddrPort("198.51.100.10:41000")
	obs16 := obs.Addr().As16()
	req1, pend1, err := BuildReflexReq(dev, cli, srvPub, now, 11)
	if err != nil {
		t.Fatal(err)
	}
	req2, _, err := BuildReflexReq(dev, cli, srvPub, now, 11)
	if err != nil {
		t.Fatal(err)
	}
	secrets := map[string][]byte{"uuid": dev[:], "disco pub": cliPub[:], "hub pub": srvPub[:]}
	assertNoClear(t, "reflex request", req1, secrets)
	if bytes.Equal(req1[saltLen+reqChkLen:reflexReqPrefix], req2[saltLen+reqChkLen:reflexReqPrefix]) {
		t.Fatal("reflex ephemeral reused")
	}
	if len(req1) != reflexReqLen {
		t.Fatalf("reflex request is %d bytes", len(req1))
	}
	rq, err := OpenReflexReq(srvPriv, req1, now)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := BuildReflexResp(&rq, obs)
	if err != nil {
		t.Fatal(err)
	}
	secrets["observed addr"] = obs16[:]
	assertNoClear(t, "reflex response", resp, secrets)
	if len(resp) != reflexRespLen {
		t.Fatalf("reflex response is %d bytes", len(resp))
	}
	if ap, err := OpenReflexResp(&pend1, resp); err != nil || ap != obs {
		t.Fatalf("reflex response %v %v", ap, err)
	}
	if pend1.ephPub == ([32]byte{}) || pend1.base == ([32]byte{}) {
		t.Fatal("pending reflex lost its state")
	}
	pend1.Wipe()
	if pend1.base != ([32]byte{}) || pend1.ephPriv != ([32]byte{}) {
		t.Fatal("pending reflex key not wiped")
	}
	if _, err := OpenReflexResp(&pend1, resp); err == nil {
		t.Fatal("wiped pending reflex opened a response")
	}
}

func TestV3_ReflexRejections(t *testing.T) {
	srvPriv := mustKey(t)
	srvPub := mustPublic(t, srvPriv)
	cli := mustKey(t)
	dev := uuid.New()
	now := time.Now()
	obs := netip.MustParseAddrPort("198.51.100.10:41000")
	req, pend, _ := BuildReflexReq(dev, cli, srvPub, now, 21)
	if _, err := OpenReflexReq(mustKey(t), req, now); err == nil {
		t.Fatal("reflex request opened by another hub key")
	}
	for _, off := range []int{0, saltLen, saltLen + reqChkLen + 1, reflexReqPrefix + 1, reflexReqPrefix + reflexSBoxLen + 1, len(req) - 1} {
		bad := append([]byte(nil), req...)
		bad[off] ^= 1
		if _, err := OpenReflexReq(srvPriv, bad, now); err == nil {
			t.Fatalf("flip at %d opened", off)
		}
	}
	// The inner box must be sealed with the static key named in the
	// first box: splicing another client's inner box fails.
	other, _, _ := BuildReflexReq(dev, mustKey(t), srvPub, now, 21)
	spliced := append([]byte(nil), req...)
	copy(spliced[reflexReqPrefix+reflexSBoxLen:], other[reflexReqPrefix+reflexSBoxLen:])
	if _, err := OpenReflexReq(srvPriv, spliced, now); err == nil {
		t.Fatal("spliced reflex request opened")
	}
	rq, err := OpenReflexReq(srvPriv, req, now)
	if err != nil {
		t.Fatal(err)
	}
	wrongNonce := rq
	wrongNonce.Nonce = 22
	resp, _ := BuildReflexResp(&wrongNonce, obs)
	if _, err := OpenReflexResp(&pend, resp); err == nil {
		t.Fatal("reflex response with the wrong nonce opened")
	}
	// A response to a different request (other ephemeral) does not open.
	req2, _, _ := BuildReflexReq(dev, cli, srvPub, now, 21)
	rq2, _ := OpenReflexReq(srvPriv, req2, now)
	resp2, _ := BuildReflexResp(&rq2, obs)
	if _, err := OpenReflexResp(&pend, resp2); err == nil {
		t.Fatal("response to another request opened")
	}
	// A response from a hub that is not the one we sealed to does not open.
	fake := ReflexRequest{Nonce: 21, ephPub: rq.ephPub, base: mustKey(t)}
	resp3, _ := BuildReflexResp(&fake, obs)
	if _, err := OpenReflexResp(&pend, resp3); err == nil {
		t.Fatal("response from another hub opened")
	}
	rq.Wipe()
	if _, err := BuildReflexResp(&rq, obs); err == nil {
		t.Fatal("wiped request still built a response")
	}
}

func TestV3_EngineReflexMatchesNonce(t *testing.T) {
	srvPriv := mustKey(t)
	srvPub := mustPublic(t, srvPriv)
	id := uuid.New()
	e := NewEngine(id, mustKey(t), [32]byte{2}, netip.MustParseAddr("10.0.1.8"))
	e.SetServerDisco(srvPub)
	e.SetServerEndpoint(testHub)
	now := time.Now()
	reqOf := func(r Result) []byte {
		for _, d := range r.Sends {
			if d.Server && d.Type == TypeReflexReq {
				return d.Data
			}
		}
		t.Fatal("no reflex request")
		return nil
	}
	first := reqOf(e.Poll(now))
	second := reqOf(e.Poll(now.Add(ReflexEvery)))
	obs := netip.MustParseAddrPort("198.51.100.10:41000")
	rq1, err := OpenReflexReq(srvPriv, first, now)
	if err != nil {
		t.Fatal(err)
	}
	stale, _ := BuildReflexResp(&rq1, obs)
	e.OnPacket(testHub, stale, now.Add(ReflexEvery))
	e.mu.Lock()
	learned := e.reflex.IsValid()
	e.mu.Unlock()
	if learned {
		t.Fatal("response to a superseded request was stored")
	}
	rq2, err := OpenReflexReq(srvPriv, second, now.Add(ReflexEvery))
	if err != nil {
		t.Fatal(err)
	}
	if rq2.Device != id {
		t.Fatal("hub did not learn the device")
	}
	fresh, _ := BuildReflexResp(&rq2, obs)
	e.OnPacket(testHub, fresh, now.Add(ReflexEvery))
	e.mu.Lock()
	learned = e.reflex == obs
	wiped := e.reflexPend.base == ([32]byte{}) && e.reflexPend.ephPriv == ([32]byte{})
	e.mu.Unlock()
	if !learned || !wiped {
		t.Fatalf("reflex learned=%v wiped=%v", learned, wiped)
	}
}

func TestV3_PacketLengthsMatchMTUMath(t *testing.T) {
	aPriv, bPriv := mustKey(t), mustKey(t)
	for _, n := range []int{8, 100, 1000} {
		pkt, err := BuildPingPad(aPriv, mustPublic(t, bPriv), uuid.New(), uuid.New(), 1, n)
		if err != nil {
			t.Fatal(err)
		}
		if len(pkt) != directPacketLen(n) {
			t.Fatalf("ping %d is %d bytes, want %d", n, len(pkt), directPacketLen(n))
		}
	}
	if directPacketLen(0) != 16+32+24+4+32+16 {
		t.Fatal("direct overhead changed")
	}
	if _, err := BuildPingPad(aPriv, mustPublic(t, bPriv), uuid.New(), uuid.New(), 1, MaxPacket); err == nil {
		t.Fatal("oversized ping built")
	}
}
