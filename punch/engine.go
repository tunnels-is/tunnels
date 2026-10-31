package punch

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net/netip"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	Keepalive       = 10 * time.Second
	DirectTimeout   = 25 * time.Second
	RendezvousEvery = 5 * time.Second
	ReflexEvery     = 15 * time.Second
	PongsToTrust    = 2

	maxCands       = 16
	maxPongPaths   = 16
	maxOutstanding = 16
	maxFreshPerSec = 4

	// maxPeers bounds sessions (and so the tag map: at most
	// (2*tagSlack+1)*staticSeqs + (2*sessSlack+1)*sessSeqs tags per peer).
	maxPeers = 512
	// staticGrace is how long static tags are still accepted from a peer
	// after its first session-tagged packet, for packets already in flight.
	staticGrace = 3 * time.Second
	// sessIdle rekeys a session that has not carried a packet for this long
	// while no direct path is up, so a restarted peer can start over.
	sessIdle = 60 * time.Second
)

// timerJitter is the ± fraction applied to keepalive, candidate probe and
// reflex timers (crypto/rand). Tests that pin exact timer boundaries set
// it to 0.
var timerJitter = 0.25

type Peer struct {
	DeviceID uuid.UUID
	DiscoPub [32]byte
	WGPub    [32]byte
	WGIP     netip.Addr
}

type Datagram struct {
	Server bool
	Tunnel bool
	To     netip.AddrPort
	Data   []byte
	// Type is the punch message type of Data, for local bookkeeping only;
	// it is not visible on the wire.
	Type byte
	MTU  bool
	Peer uuid.UUID
}

type Path struct {
	Peer Peer
	Via  netip.AddrPort
}

type Result struct {
	Sends []Datagram
	Up    []Path
	Down  []uuid.UUID
	MTU   []PathMTU
}

type Engine struct {
	mu sync.Mutex

	self        uuid.UUID
	disco       [32]byte
	discoPub    [32]byte
	wgPub       [32]byte
	wgIP        netip.Addr
	server      [32]byte
	haveSrv     bool
	serverEP    netip.AddrPort
	haveSrvEP   bool
	reflex      netip.AddrPort
	reflexAt    time.Time
	reflexNonce uint64
	reflexWait  bool
	reflexPend  ReflexPending
	reflexGap   time.Duration
	jitter      float64

	allowAll bool
	allow    []string
	discover bool

	peers   map[uuid.UUID]*session
	byIP    map[netip.Addr]uuid.UUID
	want    map[netip.Addr]struct{}
	introAt map[netip.Addr]time.Time

	// tags maps every unused receive tag of every session (static slots
	// around tagWin, session slots around the peer's learned window) to its
	// slot, so a direct packet finds its key in O(1). Tags are removed on
	// use, which is the replay check.
	tags      map[[tagLen]byte]tagRef
	tagWin    int64
	tagsDirty bool
}

type tagRef struct {
	id   uuid.UUID
	slot rxSlot
}

type rxSlot struct {
	sess bool
	w    int64
	seq  int
}

type txSlot struct {
	set bool
	w   int64
	seq int
}

type sentPing struct {
	to     netip.AddrPort
	sentAt time.Time
}

type session struct {
	peer        Peer
	cands       []netip.AddrPort
	pongs       map[netip.AddrPort]int
	alive       map[netip.AddrPort]time.Time
	outstanding map[uint64]sentPing
	direct      netip.AddrPort
	directUp    bool
	lastProbe   time.Time
	lastAdv     time.Time
	txid        uint64
	epoch       uint64
	seenEpoch   uint64

	ephPriv     [32]byte
	ephPub      [32]byte
	ephLive     bool
	peerEph     [32]byte
	havePeerEph bool
	static      pairKeys
	haveStatic  bool
	sess        sessKeys
	haveSess    bool
	sessSeen    bool
	confirmedAt time.Time
	skew        int64
	lastRx      time.Time
	txStatic    txSlot
	txSess      txSlot
	rx          map[rxSlot][tagLen]byte
	used        map[rxSlot]struct{}
	probeGap    time.Duration
	freshAt     time.Time
	freshN      int

	mtuActive  bool
	mtuDone    bool
	mtuV6      bool
	mtuLo      int
	mtuHi      int
	mtuOuter   int
	mtuTx      uint64
	mtuSent    time.Time
	mtuFit     int
	mtuRetried bool
	mtuSteps   int
}

// NewEngine ...
func NewEngine(self uuid.UUID, discoPriv, wgPub [32]byte, wgIP netip.Addr) *Engine {
	pub, _ := Public(discoPriv)
	return &Engine{
		self:     self,
		disco:    discoPriv,
		discoPub: pub,
		wgPub:    wgPub,
		wgIP:     wgIP,
		peers:    make(map[uuid.UUID]*session),
		byIP:     make(map[netip.Addr]uuid.UUID),
		want:     make(map[netip.Addr]struct{}),
		introAt:  make(map[netip.Addr]time.Time),
		tags:     make(map[[tagLen]byte]tagRef),
		jitter:   timerJitter,
	}
}

// SetServerEndpoint ...
func (e *Engine) SetServerEndpoint(ep netip.AddrPort) {
	e.mu.Lock()
	e.serverEP = ep
	e.haveSrvEP = ep.IsValid()
	e.mu.Unlock()
}

// SetServerDisco ...
func (e *Engine) SetServerDisco(pub [32]byte) {
	e.mu.Lock()
	e.server = pub
	e.haveSrv = true
	e.mu.Unlock()
}

// SetLocalPolicy ...
func (e *Engine) SetLocalPolicy(firewallOff, allowAll bool, entries []string) Result {
	e.mu.Lock()
	defer e.mu.Unlock()
	if firewallOff || allowAll {
		e.allowAll = true
		e.allow = nil
	} else {
		e.allowAll = false
		e.allow = append([]string(nil), entries...)
	}
	var r Result
	for id, s := range e.peers {
		if e.admitsLocked(s.peer.WGIP) {
			continue
		}
		if s.directUp {
			r.Down = append(r.Down, id)
			if pkt, err := e.teardownLocked(s, time.Now()); err == nil && s.direct.IsValid() {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt, Type: TypeTeardown})
			}
		}
		e.dropSessionLocked(id, s)
	}
	return r
}

func (e *Engine) admitsLocked(peer netip.Addr) bool {
	return AdmitsAllPorts(false, e.allowAll, e.allow, peer)
}

func (e *Engine) punchableLocked(ip netip.Addr) bool {
	return ip.Is4() && ip.IsPrivate() && e.admitsLocked(ip)
}

func publicPunchTarget(ap netip.AddrPort) bool {
	if !ap.IsValid() || ap.Port() == 0 {
		return false
	}
	ip := ap.Addr()
	return ip.Is4() && ip.IsGlobalUnicast() && !ip.IsPrivate()
}

func publicPunchTargets(in []netip.AddrPort) []netip.AddrPort {
	out := make([]netip.AddrPort, 0, len(in))
	for _, c := range in {
		if publicPunchTarget(c) {
			out = append(out, c)
		}
	}
	return out
}

// NoteTraffic ...
func (e *Engine) NoteTraffic(ip netip.Addr) {
	if !ip.Is4() || !ip.IsPrivate() {
		return
	}
	e.mu.Lock()
	e.want[ip] = struct{}{}
	e.mu.Unlock()
}

// Direct ...
func (e *Engine) Direct(ip netip.Addr) (netip.AddrPort, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	id, ok := e.byIP[ip]
	if !ok {
		return netip.AddrPort{}, false
	}
	s := e.peers[id]
	if s == nil || !s.directUp {
		return netip.AddrPort{}, false
	}
	return s.direct, true
}

// Poll ...
func (e *Engine) Poll(now time.Time) Result {
	e.mu.Lock()
	defer e.mu.Unlock()
	var r Result
	if e.haveSrv && (e.reflexAt.IsZero() || now.Sub(e.reflexAt) >= e.reflexGap) {
		if nonce, err := newNonce(); err == nil {
			if pkt, pend, err := BuildReflexReq(e.self, e.disco, e.server, now, nonce); err == nil {
				e.reflexPend.Wipe()
				e.reflexPend = pend
				e.reflexNonce = nonce
				e.reflexWait = true
				r.Sends = append(r.Sends, Datagram{Server: true, Data: pkt, Type: TypeReflexReq})
				e.reflexAt = now
				e.reflexGap = e.jittered(ReflexEvery)
			}
		}
	}
	for ip := range e.want {
		if !e.punchableLocked(ip) {
			continue
		}
		id, ok := e.byIP[ip]
		if !ok {
			if at, seen := e.introAt[ip]; seen && now.Sub(at) < RendezvousEvery {
				continue
			}
			if pkt, ok := e.introPacketLocked(); ok {
				r.Sends = append(r.Sends, Datagram{Tunnel: true, To: netip.AddrPortFrom(ip, IntroPort), Data: pkt, Type: TypeIntro})
				e.introAt[ip] = now
			}
			continue
		}
		s := e.peers[id]
		if s == nil || s.directUp {
			continue
		}
		e.probeCandidatesLocked(s, &r, now)
		if !s.lastAdv.IsZero() && now.Sub(s.lastAdv) < RendezvousEvery {
			continue
		}
		if pkt, ok := e.introPacketLocked(); ok {
			r.Sends = append(r.Sends, Datagram{Tunnel: true, To: netip.AddrPortFrom(ip, IntroPort), Data: pkt, Type: TypeIntro})
			s.lastAdv = now
		}
	}
	for _, s := range e.peers {
		if s.haveSess && !s.directUp && !s.lastRx.IsZero() && now.Sub(s.lastRx) > sessIdle {
			e.rekeyLocked(s)
		}
	}
	for id, s := range e.peers {
		if !s.directUp {
			continue
		}
		if !e.admitsLocked(s.peer.WGIP) {
			r.Down = append(r.Down, id)
			s.directUp = false
			s.resetMTU()
			continue
		}
		seen, ok := s.alive[s.direct]
		if !ok || now.Sub(seen) > DirectTimeout {
			r.Down = append(r.Down, id)
			if pkt, err := e.teardownLocked(s, now); err == nil {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt, Type: TypeTeardown})
			}
			s.dropDirectLocked()
			e.rekeyLocked(s)
			continue
		}
		if now.Sub(s.lastProbe) >= s.gap() {
			if pkt, err := e.pingLocked(s, s.direct, now); err == nil {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt, Type: TypePing})
				s.lastProbe = now
				s.probeGap = e.jittered(Keepalive)
			}
		}
		e.pollMTULocked(s, &r, now)
	}
	return r
}

// OnPacket ...
func (e *Engine) OnPacket(from netip.AddrPort, pkt []byte, now time.Time) Result {
	e.mu.Lock()
	defer e.mu.Unlock()
	var r Result
	if TypeOf(pkt) == TypeIntro {
		// Intros arrive through the tunnel, never on the underlay.
		e.onIntroLocked(&r, from, pkt, now)
		return r
	}
	if !Is(pkt) {
		return r
	}
	if s, ref, ok := e.lookupTagLocked(pkt, now); ok {
		e.onDirectLocked(&r, from, s, ref, pkt, now)
		return r
	}
	if e.haveSrvEP && from == e.serverEP {
		e.onReflexLocked(from, pkt, now)
	}
	return r
}

// jittered spreads a timer by ±e.jitter using crypto/rand.
func (e *Engine) jittered(d time.Duration) time.Duration {
	if e.jitter <= 0 {
		return d
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return d
	}
	u := float64(binary.BigEndian.Uint64(b[:])>>11) / float64(uint64(1)<<53)
	return time.Duration(float64(d) * (1 - e.jitter + 2*e.jitter*u))
}

func (s *session) gap() time.Duration {
	if s.probeGap <= 0 {
		return Keepalive
	}
	return s.probeGap
}

func (e *Engine) onDirectLocked(r *Result, from netip.AddrPort, s *session, ref tagRef, pkt []byte, now time.Time) {
	admitted := e.admitsLocked(s.peer.WGIP)
	typ, body, ok := e.openFromPeer(s, ref, pkt, now)
	if !ok {
		return
	}
	switch typ {
	case TypePing:
		if admitted {
			e.onPingLocked(r, from, s, body, now)
		}
	case TypePong:
		if admitted {
			e.onPongLocked(r, from, s, body, now)
		}
	case TypeTeardown:
		e.onTeardownLocked(r, s, body)
	}
}

func (e *Engine) introPacketLocked() ([]byte, bool) {
	pub, err := Public(e.disco)
	if err != nil {
		return nil, false
	}
	var cands []netip.AddrPort
	if e.reflex.IsValid() {
		cands = append(cands, e.reflex)
	}
	pkt, err := BuildIntro(e.self, pub, e.wgPub, e.wgIP, cands)
	if err != nil {
		return nil, false
	}
	return pkt, true
}

func (e *Engine) onIntroLocked(r *Result, from netip.AddrPort, pkt []byte, now time.Time) {
	msg, err := ParseIntro(pkt)
	if err != nil || msg.Src == e.self || msg.DiscoPub == e.discoPub || !from.IsValid() || from.Addr() != msg.WGIP || !e.punchableLocked(msg.WGIP) {
		return
	}
	s := e.peers[msg.Src]
	if s == nil && len(e.peers) >= maxPeers {
		return
	}
	if s == nil || s.peer.WGPub != msg.WGPub || s.peer.DiscoPub != msg.DiscoPub || s.peer.WGIP != msg.WGIP {
		if s != nil && s.directUp {
			r.Down = append(r.Down, msg.Src)
		}
		if s != nil {
			e.dropSessionLocked(msg.Src, s)
		}
		s = newSession(Peer{DeviceID: msg.Src, DiscoPub: msg.DiscoPub, WGPub: msg.WGPub, WGIP: msg.WGIP})
		e.peers[msg.Src] = s
		e.byIP[msg.WGIP] = msg.Src
		e.tagsDirty = true
	}
	s.cands = capAddrs(publicPunchTargets(msg.Candidates), maxCands)
	if _, ok := e.want[msg.WGIP]; !ok {
		return
	}
	e.probeCandidatesLocked(s, r, now)
	if s.lastAdv.IsZero() || now.Sub(s.lastAdv) >= RendezvousEvery {
		if reply, ok := e.introPacketLocked(); ok {
			r.Sends = append(r.Sends, Datagram{Tunnel: true, To: netip.AddrPortFrom(msg.WGIP, IntroPort), Data: reply, Type: TypeIntro})
			s.lastAdv = now
		}
	}
}

func (e *Engine) probeCandidatesLocked(s *session, r *Result, now time.Time) {
	if s == nil || s.directUp || len(s.cands) == 0 {
		return
	}
	if !s.lastProbe.IsZero() && now.Sub(s.lastProbe) < s.gap() {
		return
	}
	for _, c := range s.cands {
		if ping, err := e.pingLocked(s, c, now); err == nil {
			r.Sends = append(r.Sends, Datagram{To: c, Data: ping, Type: TypePing})
		}
	}
	s.lastProbe = now
	s.probeGap = e.jittered(Keepalive)
}

func (e *Engine) onPingLocked(r *Result, from netip.AddrPort, s *session, plain []byte, now time.Time) {
	txid, err := pingTxid(plain)
	if err != nil {
		return
	}
	known := containsAddr(s.cands, from) || s.pongs[from] > 0
	if !known && !allowFresh(s, now) {
		return
	}
	var body [8]byte
	binary.BigEndian.PutUint64(body[:], txid)
	pong, err := e.sealToPeer(s, TypePong, body[:], now, false)
	if err != nil {
		return
	}
	r.Sends = append(r.Sends, Datagram{To: from, Data: pong, Type: TypePong})
	if containsAddr(s.cands, from) || len(s.cands) >= maxCands || len(s.pongs) >= maxPongPaths {
		return
	}
	if ping, err := e.pingLocked(s, from, now); err == nil {
		r.Sends = append(r.Sends, Datagram{To: from, Data: ping, Type: TypePing})
	}
}

func (e *Engine) onPongLocked(r *Result, from netip.AddrPort, s *session, plain []byte, now time.Time) {
	if len(plain) != 8 {
		return
	}
	txid := binary.BigEndian.Uint64(plain)
	job, ok := s.outstanding[txid]
	if !ok || job.to != from || now.Sub(job.sentAt) > DirectTimeout {
		return
	}
	delete(s.outstanding, txid)
	e.onMTUPongLocked(s, txid, r, now)
	if _, seen := s.pongs[from]; !seen && len(s.pongs) >= maxPongPaths {
		return
	}
	if !containsAddr(s.cands, from) && len(s.cands) < maxCands {
		s.cands = append(s.cands, from)
	}
	s.pongs[from]++
	s.alive[from] = now
	if s.pongs[from] == 1 {
		if ping, err := e.pingLocked(s, from, now); err == nil {
			r.Sends = append(r.Sends, Datagram{To: from, Data: ping, Type: TypePing})
		}
	}
	if s.pongs[from] < PongsToTrust {
		return
	}
	best := e.bestLocked(s, from)
	if s.directUp && s.direct == best {
		return
	}
	changed := s.directUp && s.direct != best
	s.direct = best
	s.directUp = true
	s.lastProbe = now
	s.probeGap = e.jittered(Keepalive)
	r.Up = append(r.Up, Path{Peer: s.peer, Via: best})
	e.armMTULocked(s, r, now, !s.mtuActive || changed)
}

func (e *Engine) onTeardownLocked(r *Result, s *session, plain []byte) {
	claimed := s.peer.DeviceID
	if len(plain) != 8 {
		return
	}
	epoch := binary.BigEndian.Uint64(plain)
	if epoch == 0 || epoch <= s.seenEpoch {
		return
	}
	s.seenEpoch = epoch
	if s.directUp {
		r.Down = append(r.Down, claimed)
	}
	s.dropDirectLocked()
	// The peer rekeys after sending a teardown; follow it so both sides
	// return to the static phase with fresh ephemerals.
	e.rekeyLocked(s)
}

func (e *Engine) pingLocked(s *session, to netip.AddrPort, now time.Time) ([]byte, error) {
	s.txid++
	rememberPing(s, s.txid, to, now)
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], s.txid)
	pkt, err := e.sealToPeer(s, TypePing, plain[:], now, false)
	if err != nil {
		delete(s.outstanding, s.txid)
		return nil, err
	}
	return pkt, nil
}

func (e *Engine) pingPadLocked(s *session, plainLen int, now time.Time) ([]byte, error) {
	s.txid++
	rememberPing(s, s.txid, s.direct, now)
	plain := make([]byte, plainLen)
	binary.BigEndian.PutUint64(plain[:8], s.txid)
	pkt, err := e.sealToPeer(s, TypePing, plain, now, true)
	if err != nil {
		delete(s.outstanding, s.txid)
		return nil, err
	}
	return pkt, nil
}

func (e *Engine) bestLocked(s *session, fresh netip.AddrPort) netip.AddrPort {
	best := fresh
	bestScore := addrScore(fresh.Addr())
	for ap, n := range s.pongs {
		if n < PongsToTrust {
			continue
		}
		sc := addrScore(ap.Addr())
		if sc > bestScore {
			best = ap
			bestScore = sc
		}
	}
	return best
}

func addrScore(a netip.Addr) int {
	switch {
	case a.IsLinkLocalUnicast():
		return 3
	case a.IsPrivate():
		return 2
	default:
		return 1
	}
}

func containsAddr(list []netip.AddrPort, ap netip.AddrPort) bool {
	for _, c := range list {
		if c == ap {
			return true
		}
	}
	return false
}

func capAddrs(in []netip.AddrPort, n int) []netip.AddrPort {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func newSession(peer Peer) *session {
	return &session{
		peer:        peer,
		pongs:       make(map[netip.AddrPort]int),
		alive:       make(map[netip.AddrPort]time.Time),
		outstanding: make(map[uint64]sentPing),
	}
}

func (s *session) dropDirectLocked() {
	s.directUp = false
	s.pongs = make(map[netip.AddrPort]int)
	s.alive = make(map[netip.AddrPort]time.Time)
	s.outstanding = make(map[uint64]sentPing)
	s.resetMTU()
}

func (e *Engine) onReflexLocked(from netip.AddrPort, pkt []byte, now time.Time) {
	if !e.haveSrv || !e.haveSrvEP || !e.reflexWait || from != e.serverEP {
		return
	}
	if e.reflexPend.Nonce() != e.reflexNonce {
		return
	}
	ap, err := OpenReflexResp(&e.reflexPend, pkt)
	if err != nil || !publicPunchTarget(ap) {
		return
	}
	e.reflexPend.Wipe()
	e.reflex = ap
	e.reflexWait = false
	e.reflexAt = now
}

func (e *Engine) dropSessionLocked(id uuid.UUID, s *session) {
	if e.peers[id] == s {
		delete(e.peers, id)
	}
	if e.byIP[s.peer.WGIP] == id {
		delete(e.byIP, s.peer.WGIP)
	}
	for slot := range s.rx {
		e.dropSlotLocked(id, s, slot)
	}
	s.wipeKeys()
	e.tagsDirty = true
}

// staticOpen reports whether static (first-flight) tags from s are
// still accepted: until the first session-tagged packet, plus a short grace.
func staticOpen(s *session, now time.Time) bool {
	return s.confirmedAt.IsZero() || now.Sub(s.confirmedAt) < staticGrace
}

func clampSkew(d int64) int64 {
	if d < -tagSlack {
		return -tagSlack
	}
	if d > tagSlack {
		return tagSlack
	}
	return d
}

// wantSlot reports whether slot should be in the receive map at window w.
func wantSlot(s *session, slot rxSlot, w int64, now time.Time) bool {
	if slot.sess {
		c := w + s.skew
		return s.haveSess && slot.seq < sessSeqs && slot.w >= c-sessSlack && slot.w <= c+sessSlack
	}
	return staticOpen(s, now) && slot.seq < staticSeqs && slot.w >= w-tagSlack && slot.w <= w+tagSlack
}

// refreshTagsLocked keeps e.tags equal to the union of every session's
// wanted, unused receive slots. Work is incremental: on a window change
// each session computes only the slots of the new windows.
func (e *Engine) refreshTagsLocked(now time.Time) {
	w := window(now)
	if !e.tagsDirty && w == e.tagWin {
		return
	}
	for id, s := range e.peers {
		e.syncTagsLocked(id, s, w, now)
	}
	e.tagWin = w
	e.tagsDirty = false
}

func (e *Engine) dropSlotLocked(id uuid.UUID, s *session, slot rxSlot) {
	tag, ok := s.rx[slot]
	if !ok {
		return
	}
	delete(s.rx, slot)
	if ref, ok := e.tags[tag]; ok && ref.id == id && ref.slot == slot {
		delete(e.tags, tag)
	}
}

func (e *Engine) addSlotLocked(id uuid.UUID, s *session, slot rxSlot, key [32]byte) {
	if _, ok := s.rx[slot]; ok {
		return
	}
	if _, used := s.used[slot]; used {
		return
	}
	tag, ok := slotTag(key, slot.w, slot.seq)
	if !ok {
		return
	}
	s.rx[slot] = tag
	e.tags[tag] = tagRef{id: id, slot: slot}
}

func (e *Engine) syncTagsLocked(id uuid.UUID, s *session, w int64, now time.Time) {
	if s.rx == nil {
		s.rx = make(map[rxSlot][tagLen]byte)
	}
	if s.used == nil {
		s.used = make(map[rxSlot]struct{})
	}
	for slot := range s.rx {
		if !wantSlot(s, slot, w, now) {
			e.dropSlotLocked(id, s, slot)
		}
	}
	// A used slot can only become wanted again while its window is within
	// reach of the widest acceptance range, so older ones are forgotten.
	floor := w - tagSlack - sessSlack
	for slot := range s.used {
		if slot.w < floor {
			delete(s.used, slot)
		}
	}
	if err := e.ensureStatic(s); err != nil {
		return
	}
	if staticOpen(s, now) {
		for d := int64(-tagSlack); d <= tagSlack; d++ {
			for seq := 0; seq < staticSeqs; seq++ {
				e.addSlotLocked(id, s, rxSlot{w: w + d, seq: seq}, s.static.tagRecv)
			}
		}
	}
	if s.haveSess {
		c := w + s.skew
		for d := int64(-sessSlack); d <= sessSlack; d++ {
			for seq := 0; seq < sessSeqs; seq++ {
				e.addSlotLocked(id, s, rxSlot{sess: true, w: c + d, seq: seq}, s.sess.tagRecv)
			}
		}
	}
}

func (e *Engine) lookupTagLocked(pkt []byte, now time.Time) (*session, tagRef, bool) {
	if len(pkt) < directPacketLen(0) {
		return nil, tagRef{}, false
	}
	e.refreshTagsLocked(now)
	var tag [tagLen]byte
	copy(tag[:], pkt[:tagLen])
	ref, ok := e.tags[tag]
	if !ok {
		return nil, tagRef{}, false
	}
	s := e.peers[ref.id]
	if s == nil {
		return nil, tagRef{}, false
	}
	if !ref.slot.sess && !staticOpen(s, now) {
		return nil, tagRef{}, false
	}
	return s, ref, true
}

// consumeLocked retires a receive slot after a packet authenticated with it.
func (e *Engine) consumeLocked(s *session, ref tagRef) {
	e.dropSlotLocked(ref.id, s, ref.slot)
	s.used[ref.slot] = struct{}{}
}

// nextTag returns the next unused send tag of the current window. Each tag
// is used once; when a window's budget runs out sends wait for the next.
func (s *session) nextTag(sessPhase bool, now time.Time) ([tagLen]byte, error) {
	tx, key, max := &s.txStatic, s.static.tagSend, staticSeqs
	if sessPhase {
		tx, key, max = &s.txSess, s.sess.tagSend, sessSeqs
	}
	w := window(now)
	if !tx.set || tx.w != w {
		*tx = txSlot{set: true, w: w}
	}
	for tx.seq < max {
		seq := tx.seq
		tx.seq++
		if tag, ok := slotTag(key, w, seq); ok {
			return tag, nil
		}
	}
	return [tagLen]byte{}, errTagBudget
}

var errTagBudget = errors.New("punch tag budget exhausted for this window")

// rekeyLocked forgets the session keys and ephemerals of s, so the next
// exchange starts from the static phase with fresh ephemerals.
func (e *Engine) rekeyLocked(s *session) {
	s.sess.wipe()
	s.haveSess = false
	s.sessSeen = false
	s.confirmedAt = time.Time{}
	s.skew = 0
	s.txSess = txSlot{}
	wipe32(&s.ephPriv)
	s.ephPub = [32]byte{}
	s.ephLive = false
	s.peerEph = [32]byte{}
	s.havePeerEph = false
	for slot := range s.used {
		if slot.sess {
			delete(s.used, slot)
		}
	}
	e.tagsDirty = true
}

func (e *Engine) teardownLocked(s *session, now time.Time) ([]byte, error) {
	s.epoch++
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], s.epoch)
	return e.sealToPeer(s, TypeTeardown, plain[:], now, false)
}

func newNonce() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b[:]), nil
}

func allowFresh(s *session, now time.Time) bool {
	if now.Sub(s.freshAt) >= time.Second {
		s.freshAt = now
		s.freshN = 0
	}
	if s.freshN >= maxFreshPerSec {
		return false
	}
	s.freshN++
	return true
}

func rememberPing(s *session, txid uint64, to netip.AddrPort, now time.Time) {
	if s.outstanding == nil {
		s.outstanding = make(map[uint64]sentPing)
	}
	for id, p := range s.outstanding {
		if now.Sub(p.sentAt) > DirectTimeout {
			delete(s.outstanding, id)
		}
	}
	for len(s.outstanding) >= maxOutstanding {
		var drop uint64
		var at time.Time
		first := true
		for id, p := range s.outstanding {
			if first || p.sentAt.Before(at) {
				drop, at, first = id, p.sentAt, false
			}
		}
		delete(s.outstanding, drop)
	}
	s.outstanding[txid] = sentPing{to: to, sentAt: now}
}

func (s *session) ensureEph() error {
	if s.ephPub != ([32]byte{}) {
		return nil
	}
	priv, err := NewPrivate()
	if err != nil {
		return err
	}
	pub, err := Public(priv)
	if err != nil {
		return err
	}
	s.ephPriv = priv
	s.ephPub = pub
	s.ephLive = true
	return nil
}

func (e *Engine) ensureStatic(s *session) error {
	if s.haveStatic {
		return nil
	}
	k, err := derivePair(e.disco, s.peer.DiscoPub)
	if err != nil {
		return err
	}
	s.static = k
	s.haveStatic = true
	return nil
}

func (s *session) wipeKeys() {
	s.static.wipe()
	s.haveStatic = false
	wipe32(&s.ephPriv)
	s.ephLive = false
	s.sess.wipe()
	s.haveSess = false
	s.rx = nil
	s.used = nil
}

func (e *Engine) commitEELocked(s *session) {
	if s.haveSess || !s.ephLive || !s.havePeerEph || !s.haveStatic {
		return
	}
	k, err := deriveEE(s.ephPriv, s.peerEph, e.discoPub, s.peer.DiscoPub, s.static.bind)
	if err != nil {
		return
	}
	s.sess = k
	wipe32(&s.ephPriv)
	s.ephLive = false
	s.haveSess = true
	e.tagsDirty = true
}

// sealToPeer seals a direct packet. Before the peer has proven it holds the
// session (sessSeen) the static tag and mask are used, with the session AEAD
// key once we have it; afterwards everything comes from the session keys.
func (e *Engine) sealToPeer(s *session, typ byte, plain []byte, now time.Time, exact bool) ([]byte, error) {
	if err := s.ensureEph(); err != nil {
		return nil, err
	}
	if err := e.ensureStatic(s); err != nil {
		return nil, err
	}
	e.commitEELocked(s)
	sessPhase := s.haveSess && s.sessSeen
	out := directOut{key: s.static.send, hp: s.static.hpSend}
	if s.haveSess {
		out.key = s.sess.send
	}
	if sessPhase {
		out.hp = s.sess.hpSend
	}
	tag, err := s.nextTag(sessPhase, now)
	if err != nil {
		return nil, err
	}
	out.tag = tag
	return sealDirect(typ, out, s.ephPub, e.self, s.peer.DeviceID, plain, exact)
}

type openTry struct {
	key  [32]byte
	kind byte
}

// openFromPeer authenticates a direct packet whose tag matched ref. Kinds:
// 0 static AEAD key (first flight, only before the peer used the session),
// 1 session key, 2 tentative session key from the packet's ephemeral.
// Session-tagged packets only open with kind 1.
func (e *Engine) openFromPeer(s *session, ref tagRef, pkt []byte, now time.Time) (typ byte, body []byte, ok bool) {
	if err := s.ensureEph(); err != nil {
		return 0, nil, false
	}
	if err := e.ensureStatic(s); err != nil {
		return 0, nil, false
	}
	parts, err := splitDirect(pkt)
	if err != nil {
		return 0, nil, false
	}
	tries := make([]openTry, 0, 3)
	var eph [32]byte
	var tent sessKeys
	defer tent.wipe()
	if ref.slot.sess {
		if !s.haveSess {
			return 0, nil, false
		}
		tries = append(tries, openTry{key: s.sess.recv, kind: 1})
	} else {
		eph = parts.eph(s.static.hpRecv)
		if s.haveSess {
			tries = append(tries, openTry{key: s.sess.recv, kind: 1})
		}
		if !s.haveSess && s.ephLive && eph != ([32]byte{}) {
			if tent, err = deriveEE(s.ephPriv, eph, e.discoPub, s.peer.DiscoPub, s.static.bind); err == nil {
				tries = append(tries, openTry{key: tent.recv, kind: 2})
			}
		}
		if !s.sessSeen {
			tries = append(tries, openTry{key: s.static.recv, kind: 0})
		}
	}
	var kind byte
	var src, dst uuid.UUID
	opened := false
	for _, try := range tries {
		typ, body, src, dst, err = parts.open(try.key)
		if err == nil {
			kind = try.kind
			opened = true
			break
		}
	}
	if !opened || src != s.peer.DeviceID || dst != e.self {
		return 0, nil, false
	}
	e.consumeLocked(s, ref)
	s.lastRx = now
	if skew := clampSkew(ref.slot.w - window(now)); skew != s.skew {
		s.skew = skew
		e.tagsDirty = true
	}
	if ref.slot.sess && s.confirmedAt.IsZero() {
		s.confirmedAt = now
	}
	if kind == 2 {
		s.sess = tent
		tent = sessKeys{}
		wipe32(&s.ephPriv)
		s.ephLive = false
		s.haveSess = true
		e.tagsDirty = true
	}
	if kind == 1 || kind == 2 {
		s.sessSeen = true
	}
	if !ref.slot.sess && eph != ([32]byte{}) {
		s.peerEph = eph
		s.havePeerEph = true
	}
	e.commitEELocked(s)
	return typ, body, true
}
