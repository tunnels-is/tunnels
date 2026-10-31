package punch

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
)

// sealFrom seals a direct packet from e to peer id exactly as the engine
// would on the wire.
func sealFrom(t *testing.T, e *Engine, id uuid.UUID, typ byte, now time.Time) []byte {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.peers[id]
	var body [8]byte
	if _, err := rand.Read(body[:]); err != nil {
		t.Fatal(err)
	}
	pkt, err := e.sealToPeer(s, typ, body[:], now, false)
	if err != nil {
		t.Fatal(err)
	}
	return pkt
}

func refOf(t *testing.T, e *Engine, pkt []byte, now time.Time) (tagRef, bool) {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ref, ok := e.lookupTagLocked(pkt, now)
	return ref, ok
}

func TestHardening_SessionTagsReplaceStaticTags(t *testing.T) {
	now := time.Now()
	p := punchPair(t, now)
	now = now.Add(time.Second)
	p.a.mu.Lock()
	sa := p.a.peers[p.bID]
	ok := sa.haveSess && sa.sessSeen
	p.a.mu.Unlock()
	if !ok {
		t.Fatal("A did not reach the session phase")
	}
	ping := sealFrom(t, p.a, p.bID, TypePing, now)
	ref, ok := refOf(t, p.b, ping, now)
	if !ok || !ref.slot.sess {
		t.Fatal("session-phase packet does not carry a session tag")
	}
	if r := p.b.OnPacket(p.aRef, ping, now); len(r.Sends) == 0 {
		t.Fatal("session-tagged ping not answered")
	}
	p.b.mu.Lock()
	sb := p.b.peers[p.aID]
	confirmed := !sb.confirmedAt.IsZero()
	p.b.mu.Unlock()
	if !confirmed {
		t.Fatal("B did not confirm the session")
	}

	// A first-flight (static-tag, static-key) packet from A is still taken
	// during the grace period by tag, but no longer opens: B already saw
	// A use the session, so the static AEAD key is retired.
	static, _ := buildSealedAt(TypePing, p.aPriv, mustPublic(t, p.bPriv), p.aID, p.bID, make([]byte, 8), now, false)
	if r := p.b.OnPacket(p.aRef, static, now); len(r.Sends) != 0 {
		t.Fatal("static-key ping answered after the session was used")
	}
	// After the grace the static tags are gone altogether.
	late := now.Add(staticGrace + time.Second)
	static2, _ := buildSealedAt(TypePing, p.aPriv, mustPublic(t, p.bPriv), p.aID, p.bID, make([]byte, 8), late, false)
	if _, ok := refOf(t, p.b, static2, late); ok {
		t.Fatal("static tag still accepted after the grace")
	}
	// Force a window change and check no static slot is left in the map.
	later := late.Add(tagWindow)
	p.b.mu.Lock()
	p.b.refreshTagsLocked(later)
	for slot := range sb.rx {
		if !slot.sess {
			p.b.mu.Unlock()
			t.Fatal("static slot kept after confirmation")
		}
	}
	p.b.mu.Unlock()

	// Rollover: across several windows session-tagged pings keep working.
	for i := 0; i < 5; i++ {
		later = later.Add(tagWindow)
		pkt := sealFrom(t, p.a, p.bID, TypePing, later)
		if r := p.b.OnPacket(p.aRef, pkt, later); len(r.Sends) == 0 {
			t.Fatalf("session ping in window +%d not answered", i)
		}
	}

	// Rekey wipes the session keys and drops their tags.
	p.b.mu.Lock()
	old := sb.sess
	p.b.rekeyLocked(sb)
	p.b.refreshTagsLocked(later)
	wiped := sb.sess == (sessKeys{}) && !sb.haveSess && !sb.sessSeen
	var leftover int
	for slot := range sb.rx {
		if slot.sess {
			leftover++
		}
	}
	for _, ref := range p.b.tags {
		if ref.slot.sess {
			leftover++
		}
	}
	p.b.mu.Unlock()
	if old == (sessKeys{}) || !wiped || leftover != 0 {
		t.Fatalf("rekey wiped=%v leftover=%d", wiped, leftover)
	}
	pkt := sealFrom(t, p.a, p.bID, TypePing, later)
	if r := p.b.OnPacket(p.aRef, pkt, later); len(r.Sends) != 0 {
		t.Fatal("old-session ping answered after rekey")
	}
}

func TestHardening_TagsUnlinkableReplayAndReorder(t *testing.T) {
	now := time.Now()
	p := punchPair(t, now)
	now = now.Add(time.Second)
	const n = 20
	pkts := make([][]byte, n)
	seen := map[[tagLen]byte]bool{}
	for i := range pkts {
		pkts[i] = sealFrom(t, p.a, p.bID, TypePing, now)
		var tag [tagLen]byte
		copy(tag[:], pkts[i])
		if seen[tag] {
			t.Fatal("tag reused")
		}
		seen[tag] = true
	}
	// Delivered in reverse order: all inside the receive-ahead window.
	for i := n - 1; i >= 0; i-- {
		if r := p.b.OnPacket(p.aRef, pkts[i], now); len(r.Sends) == 0 {
			t.Fatalf("reordered ping %d not answered", i)
		}
	}
	for i := range pkts {
		if r := p.b.OnPacket(p.aRef, pkts[i], now); len(r.Sends) != 0 {
			t.Fatalf("replayed ping %d answered", i)
		}
	}
	// A replay in the next window (slot already retired) is still refused.
	if r := p.b.OnPacket(p.aRef, pkts[0], now.Add(tagWindow)); len(r.Sends) != 0 {
		t.Fatal("replay answered in the next window")
	}
	// Bounded state: the per-window send budget is enforced and the
	// receive map stays within its cap.
	p.a.mu.Lock()
	sa := p.a.peers[p.bID]
	var err error
	for i := 0; i < sessSeqs+1 && err == nil; i++ {
		_, err = sa.nextTag(true, now)
	}
	p.a.mu.Unlock()
	if err == nil {
		t.Fatal("send budget not enforced")
	}
	p.b.mu.Lock()
	sb := p.b.peers[p.aID]
	max := (2*tagSlack+1)*staticSeqs + (2*sessSlack+1)*sessSeqs
	nrx, ntags := len(sb.rx), len(p.b.tags)
	p.b.mu.Unlock()
	if nrx > max || ntags > max {
		t.Fatalf("receive state %d/%d over cap %d", nrx, ntags, max)
	}
}

func TestHardening_NoFixedBytes(t *testing.T) {
	now := time.Now()
	p := punchPair(t, now)
	now = now.Add(time.Second)
	srvPriv := mustKey(t)
	srvPub := mustPublic(t, srvPriv)
	cli := mustKey(t)
	const n = 64
	// The sender spends one single-use tag per packet; step windows so the
	// per-window budget is not the limit here.
	sent := 0
	direct := func(typ byte) []byte {
		sent++
		return sealFrom(t, p.a, p.bID, typ, now.Add(time.Duration(sent/16)*tagWindow))
	}
	kinds := map[string]func() []byte{
		"ping":     func() []byte { return direct(TypePing) },
		"pong":     func() []byte { return direct(TypePong) },
		"teardown": func() []byte { return direct(TypeTeardown) },
		"static": func() []byte {
			pkt, _ := BuildPing(p.aPriv, mustPublic(t, p.bPriv), p.aID, p.bID, 1)
			return pkt
		},
		"reflex req": func() []byte {
			pkt, _, _ := BuildReflexReq(p.aID, cli, srvPub, now, 3)
			return pkt
		},
		"reflex resp": func() []byte {
			req, _, _ := BuildReflexReq(p.aID, cli, srvPub, now, 3)
			rq, err := OpenReflexReq(srvPriv, req, now)
			if err != nil {
				t.Fatal(err)
			}
			resp, _ := BuildReflexResp(&rq, p.aRef)
			return resp
		},
		"mtu probe": func() []byte {
			pkt, _ := BuildMTUProbe(1200)
			return pkt
		},
		"mtu ack": func() []byte {
			pkt, _ := BuildMTUProbe(1200)
			return MTUAckFor(pkt)
		},
	}
	var all [][]byte
	for name, mk := range kinds {
		var set [][]byte
		for i := 0; i < n; i++ {
			pkt := mk()
			if len(pkt) < MinPacket || LooksLikeWireGuard(pkt) || !Is(pkt) {
				t.Fatalf("%s: bad packet len %d", name, len(pkt))
			}
			set = append(set, pkt)
		}
		if off := fixedByte(set); off >= 0 {
			t.Fatalf("%s: byte %d is the same in all %d packets", name, off, n)
		}
		all = append(all, set...)
	}
	if off := fixedByte(all); off >= 0 {
		t.Fatalf("byte %d is the same across all packet types", off)
	}
}

// fixedByte returns the first offset (within the shortest packet) whose
// value is identical in every packet, or -1.
func fixedByte(pkts [][]byte) int {
	min := len(pkts[0])
	for _, p := range pkts {
		if len(p) < min {
			min = len(p)
		}
	}
	for off := 0; off < min; off++ {
		same := true
		for _, p := range pkts[1:] {
			if p[off] != pkts[0][off] {
				same = false
				break
			}
		}
		if same {
			return off
		}
	}
	return -1
}

func wgLike(typ byte, n int) []byte {
	pkt := make([]byte, n)
	_, _ = rand.Read(pkt)
	binary.LittleEndian.PutUint32(pkt, uint32(typ))
	return pkt
}

func TestHardening_DemuxBothWays(t *testing.T) {
	// WireGuard -> punch: no WireGuard message is ever taken as punch, at
	// any size (init 148, response 92, cookie 64, data >= 32).
	for _, typ := range []byte{1, 2, 3, 4} {
		for _, n := range []int{32, 64, 92, 148, 191, 192, 384, 768, 1420, 1536, 2048} {
			for i := 0; i < 50; i++ {
				if Is(wgLike(typ, n)) {
					t.Fatalf("wireguard type %d len %d taken as punch", typ, n)
				}
			}
		}
	}
	// Engines ignore them too.
	now := time.Now()
	p := punchPair(t, now)
	for _, typ := range []byte{1, 2, 3, 4} {
		if r := p.b.OnPacket(p.aRef, wgLike(typ, 192), now); len(r.Sends)+len(r.Down)+len(r.Up) != 0 {
			t.Fatal("engine acted on a wireguard packet")
		}
	}
	// Punch -> WireGuard: every punch packet kind, many times, never looks
	// like WireGuard, including tags drawn from the slot space.
	srvPub := mustPublic(t, mustKey(t))
	cli := mustKey(t)
	for i := 0; i < 2000; i++ {
		var pkts [][]byte
		pkts = append(pkts, sealFrom(t, p.a, p.bID, TypePing, now.Add(time.Duration(i/50)*tagWindow)))
		st, _ := BuildPing(p.aPriv, mustPublic(t, p.bPriv), p.aID, p.bID, uint64(i))
		rq, _, _ := BuildReflexReq(p.aID, cli, srvPub, now, uint64(i))
		pr, _ := BuildMTUProbe(MinPacket + i%500)
		pkts = append(pkts, st, rq, pr, MTUAckFor(pr))
		for _, pkt := range pkts {
			if LooksLikeWireGuard(pkt) || !Is(pkt) {
				t.Fatalf("punch packet %x... misclassified", pkt[:4])
			}
		}
	}
	for w := int64(0); w < 200; w++ {
		for seq := 0; seq < sessSeqs; seq++ {
			if tag, ok := slotTag([32]byte{byte(w)}, w, seq); ok && LooksLikeWireGuard(tag[:]) {
				t.Fatal("usable slot tag looks like wireguard")
			}
		}
	}
	// Junk that is neither is bounded: too short or too long is never punch.
	if Is(make([]byte, MinPacket-1)) || Is(make([]byte, MaxPacket+1)) {
		t.Fatal("size bounds not enforced")
	}
}

func TestHardening_PaddingBuckets(t *testing.T) {
	aPriv, bPriv := mustKey(t), mustKey(t)
	bPub := mustPublic(t, bPriv)
	in := func(n int) bool {
		for _, b := range padBuckets {
			if n == b {
				return true
			}
		}
		return false
	}
	for _, n := range []int{0, 1, 8, 60, 100, 300, 600, 1000, 1300} {
		pkt, err := buildSealedAt(TypePing, aPriv, bPub, uuid.New(), uuid.New(), make([]byte, n), time.Now(), false)
		if err != nil {
			t.Fatal(err)
		}
		if !in(len(pkt)) || len(pkt) != padLen(directPacketLen(n)) {
			t.Fatalf("body %d -> %d bytes, not a bucket", n, len(pkt))
		}
		if body, _, _, err := openSealedAt(TypePing, bPriv, mustPublic(t, aPriv), pkt, time.Now()); err != nil || len(body) != n {
			t.Fatalf("padded body %d did not round-trip: %v", n, err)
		}
	}
	// MTU pings keep their exact size (they are the measurement).
	pkt, err := BuildPingPad(aPriv, bPub, uuid.New(), uuid.New(), 1, 1000)
	if err != nil || len(pkt) != directPacketLen(1000) {
		t.Fatal("mtu ping size changed")
	}
	if padLen(1) != MinPacket || padLen(1537) != 1537 {
		t.Fatal("padLen bounds")
	}
}

func TestHardening_JitterBounds(t *testing.T) {
	e := NewEngine(uuid.New(), mustKey(t), [32]byte{1}, netip.MustParseAddr("10.0.0.5"))
	e.jitter = 0.25
	lo, hi := Keepalive, Keepalive
	for i := 0; i < 2000; i++ {
		d := e.jittered(Keepalive)
		if d < time.Duration(0.75*float64(Keepalive)) || d > time.Duration(1.25*float64(Keepalive)) {
			t.Fatalf("jitter %s outside ±25%%", d)
		}
		if d < lo {
			lo = d
		}
		if d > hi {
			hi = d
		}
	}
	if hi-lo < Keepalive/4 {
		t.Fatalf("jitter spread only %s", hi-lo)
	}
	e.jitter = 0
	if e.jittered(Keepalive) != Keepalive {
		t.Fatal("zero jitter changed the timer")
	}
	// With jitter on, a punched path keeps alive: every keepalive gap is
	// within the jitter bounds and well under DirectTimeout.
	now := time.Now()
	p := punchPair(t, now)
	p.a.jitter, p.b.jitter = 0.25, 0.25
	var last time.Time
	var gaps []time.Duration
	for step := 0; step < 600; step++ {
		now = now.Add(250 * time.Millisecond)
		for _, d := range p.a.Poll(now).Sends {
			if d.Type != TypePing || d.MTU {
				continue
			}
			if !last.IsZero() {
				gaps = append(gaps, now.Sub(last))
			}
			last = now
			for _, back := range p.b.OnPacket(p.aRef, d.Data, now).Sends {
				if back.Type == TypePong {
					p.a.OnPacket(p.bRef, back.Data, now)
				}
			}
		}
		p.b.Poll(now)
	}
	if len(gaps) < 5 {
		t.Fatalf("only %d keepalives", len(gaps))
	}
	distinct := map[time.Duration]bool{}
	for _, g := range gaps {
		if g < time.Duration(0.75*float64(Keepalive)) || g > time.Duration(1.25*float64(Keepalive))+250*time.Millisecond {
			t.Fatalf("keepalive gap %s outside jitter bounds", g)
		}
		distinct[g] = true
	}
	if len(distinct) < 2 {
		t.Fatal("keepalive gaps not jittered")
	}
	if _, ok := p.a.Direct(p.bIP); !ok {
		t.Fatal("path dropped with jitter on")
	}
}

func TestHardening_ReflexResponseForwardSecret(t *testing.T) {
	srvPriv := mustKey(t)
	srvPub := mustPublic(t, srvPriv)
	cli := mustKey(t)
	now := time.Now()
	obs := netip.MustParseAddrPort("198.51.100.10:41000")
	req, pend, err := BuildReflexReq(uuid.New(), cli, srvPub, now, 5)
	if err != nil {
		t.Fatal(err)
	}
	rq, err := OpenReflexReq(srvPriv, req, now)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := BuildReflexResp(&rq, obs)
	if err != nil {
		t.Fatal(err)
	}
	// An attacker who later steals both static keys can open the request
	// (no FS for it, documented) and so learns the base key and the client
	// ephemeral pub, but not the ephemeral privates.
	stolen, err := OpenReflexReq(srvPriv, req, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, guess := range [][32]byte{{}, cli, srvPriv} {
		fake := ReflexPending{ephPriv: guess, ephPub: stolen.ephPub, base: stolen.base, nonce: 5, live: true}
		if _, err := OpenReflexResp(&fake, resp); err == nil {
			t.Fatal("response opened with static keys only")
		}
	}
	// And the response key really depends on the hub's ephemeral: the same
	// request answered twice gives different, independently keyed boxes.
	rq2, _ := OpenReflexReq(srvPriv, req, now)
	resp2, _ := BuildReflexResp(&rq2, obs)
	if bytes.Equal(resp[saltLen+respChkLen:reflexRespPrefix], resp2[saltLen+respChkLen:reflexRespPrefix]) {
		t.Fatal("hub ephemeral reused")
	}
	if ap, err := OpenReflexResp(&pend, resp); err != nil || ap != obs {
		t.Fatalf("legit client could not open: %v", err)
	}
}

func TestHardening_ReplayCache(t *testing.T) {
	c := NewReplayCache(4)
	now := time.Now()
	id := func(b byte) [32]byte { return [32]byte{b} }
	if !c.Fresh(id(1), now.Add(ReflexSkew), now) {
		t.Fatal("first sighting rejected")
	}
	if c.Fresh(id(1), now.Add(ReflexSkew), now.Add(time.Second)) {
		t.Fatal("replay inside the window accepted")
	}
	if !c.Fresh(id(1), now.Add(3*ReflexSkew), now.Add(2*ReflexSkew)) {
		t.Fatal("expired entry still blocks")
	}
	for i := byte(2); i < 50; i++ {
		c.Fresh(id(i), now.Add(time.Hour), now)
		if c.Len() > 4 {
			t.Fatalf("cache grew to %d", c.Len())
		}
	}
	// Engine-side request expiry is the request's own timestamp window.
	srvPriv := mustKey(t)
	req, _, _ := BuildReflexReq(uuid.New(), mustKey(t), mustPublic(t, srvPriv), now, 1)
	rq, err := OpenReflexReq(srvPriv, req, now)
	if err != nil {
		t.Fatal(err)
	}
	if rq.ReplayID() == ([32]byte{}) || rq.Expires().Sub(now) > ReflexSkew+time.Second {
		t.Fatal("replay id or expiry wrong")
	}
}
