package punch

import (
	"crypto/rand"
	"encoding/binary"
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
)

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
	MTU    bool
	Peer   uuid.UUID
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

	allowAll bool
	allow    []string
	discover bool

	peers   map[uuid.UUID]*session
	byIP    map[netip.Addr]uuid.UUID
	want    map[netip.Addr]struct{}
	introAt map[netip.Addr]time.Time
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
	staticSend  [32]byte
	staticRecv  [32]byte
	haveStatic  bool
	sendKey     [32]byte
	recvKey     [32]byte
	haveSess    bool
	sessSeen    bool
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
			if pkt, err := e.teardownLocked(s); err == nil && s.direct.IsValid() {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt})
			}
		}
		delete(e.peers, id)
		delete(e.byIP, s.peer.WGIP)
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
	if e.haveSrv && (e.reflexAt.IsZero() || now.Sub(e.reflexAt) >= ReflexEvery) {
		if nonce, err := newNonce(); err == nil {
			if pkt, err := BuildReflexReq(e.self, e.disco, e.server, now, nonce); err == nil {
				e.reflexNonce = nonce
				e.reflexWait = true
				r.Sends = append(r.Sends, Datagram{Server: true, Data: pkt})
				e.reflexAt = now
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
				r.Sends = append(r.Sends, Datagram{Tunnel: true, To: netip.AddrPortFrom(ip, IntroPort), Data: pkt})
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
			r.Sends = append(r.Sends, Datagram{Tunnel: true, To: netip.AddrPortFrom(ip, IntroPort), Data: pkt})
			s.lastAdv = now
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
			if pkt, err := e.teardownLocked(s); err == nil {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt})
			}
			s.dropDirectLocked()
			continue
		}
		if now.Sub(s.lastProbe) >= Keepalive {
			if pkt, err := e.pingLocked(s, s.direct, now); err == nil {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt})
				s.lastProbe = now
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
	switch TypeOf(pkt) {
	case TypeReflexResp:
		e.onReflexLocked(from, pkt, now)
	case TypeIntro:
		e.onIntroLocked(&r, from, pkt, now)
	case TypePing:
		e.onPingLocked(&r, from, pkt, now)
	case TypePong:
		e.onPongLocked(&r, from, pkt, now)
	case TypeTeardown:
		e.onTeardownLocked(&r, pkt)
	}
	return r
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
	if err != nil || msg.Src == e.self || !from.IsValid() || from.Addr() != msg.WGIP || !e.punchableLocked(msg.WGIP) {
		return
	}
	s := e.peers[msg.Src]
	if s == nil || s.peer.WGPub != msg.WGPub || s.peer.DiscoPub != msg.DiscoPub || s.peer.WGIP != msg.WGIP {
		if s != nil && s.directUp {
			r.Down = append(r.Down, msg.Src)
		}
		if s != nil {
			delete(e.byIP, s.peer.WGIP)
		}
		s = newSession(Peer{DeviceID: msg.Src, DiscoPub: msg.DiscoPub, WGPub: msg.WGPub, WGIP: msg.WGIP})
		e.peers[msg.Src] = s
		e.byIP[msg.WGIP] = msg.Src
	}
	s.cands = capAddrs(publicPunchTargets(msg.Candidates), maxCands)
	if _, ok := e.want[msg.WGIP]; !ok {
		return
	}
	e.probeCandidatesLocked(s, r, now)
	if s.lastAdv.IsZero() || now.Sub(s.lastAdv) >= RendezvousEvery {
		if reply, ok := e.introPacketLocked(); ok {
			r.Sends = append(r.Sends, Datagram{Tunnel: true, To: netip.AddrPortFrom(msg.WGIP, IntroPort), Data: reply})
			s.lastAdv = now
		}
	}
}

func (e *Engine) probeCandidatesLocked(s *session, r *Result, now time.Time) {
	if s == nil || s.directUp || len(s.cands) == 0 {
		return
	}
	if !s.lastProbe.IsZero() && now.Sub(s.lastProbe) < Keepalive {
		return
	}
	for _, c := range s.cands {
		if ping, err := e.pingLocked(s, c, now); err == nil {
			r.Sends = append(r.Sends, Datagram{To: c, Data: ping})
		}
	}
	s.lastProbe = now
}

func (e *Engine) onPingLocked(r *Result, from netip.AddrPort, pkt []byte, now time.Time) {
	s, claimed := e.peerFromHeader(pkt)
	if s == nil || !e.admitsLocked(s.peer.WGIP) {
		return
	}
	plain, src, dst, err := e.openFromPeer(s, TypePing, pkt)
	if err != nil || src != claimed || dst != e.self {
		return
	}
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
	pong, err := e.sealToPeer(s, TypePong, body[:])
	if err != nil {
		return
	}
	r.Sends = append(r.Sends, Datagram{To: from, Data: pong})
	if containsAddr(s.cands, from) || len(s.cands) >= maxCands || len(s.pongs) >= maxPongPaths {
		return
	}
	if ping, err := e.pingLocked(s, from, now); err == nil {
		r.Sends = append(r.Sends, Datagram{To: from, Data: ping})
	}
}

func (e *Engine) onPongLocked(r *Result, from netip.AddrPort, pkt []byte, now time.Time) {
	s, claimed := e.peerFromHeader(pkt)
	if s == nil || !e.admitsLocked(s.peer.WGIP) {
		return
	}
	plain, src, dst, err := e.openFromPeer(s, TypePong, pkt)
	if err != nil || src != claimed || dst != e.self || len(plain) != 8 {
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
			r.Sends = append(r.Sends, Datagram{To: from, Data: ping})
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
	r.Up = append(r.Up, Path{Peer: s.peer, Via: best})
	e.armMTULocked(s, r, now, !s.mtuActive || changed)
}

func (e *Engine) onTeardownLocked(r *Result, pkt []byte) {
	s, claimed := e.peerFromHeader(pkt)
	if s == nil {
		return
	}
	plain, src, dst, err := e.openFromPeer(s, TypeTeardown, pkt)
	if err != nil || src != claimed || dst != e.self || len(plain) != 8 {
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
}

func (e *Engine) pingLocked(s *session, to netip.AddrPort, now time.Time) ([]byte, error) {
	s.txid++
	rememberPing(s, s.txid, to, now)
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], s.txid)
	pkt, err := e.sealToPeer(s, TypePing, plain[:])
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
	pkt, err := e.sealToPeer(s, TypePing, plain)
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
	id, ap, nonce, err := OpenReflexResp(e.disco, e.server, pkt)
	if err != nil || id != e.self || nonce != e.reflexNonce || !publicPunchTarget(ap) {
		return
	}
	e.reflex = ap
	e.reflexWait = false
	e.reflexAt = now
}

func (e *Engine) peerFromHeader(pkt []byte) (*session, uuid.UUID) {
	if len(pkt) < headerLen+idLen {
		return nil, uuid.UUID{}
	}
	var claimed uuid.UUID
	copy(claimed[:], pkt[headerLen:headerLen+idLen])
	return e.peers[claimed], claimed
}

func (e *Engine) teardownLocked(s *session) ([]byte, error) {
	s.epoch++
	var plain [8]byte
	binary.BigEndian.PutUint64(plain[:], s.epoch)
	return e.sealToPeer(s, TypeTeardown, plain[:])
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
	send, recv, err := directionalKeys(e.disco, s.peer.DiscoPub)
	if err != nil {
		return err
	}
	s.staticSend = send
	s.staticRecv = recv
	s.haveStatic = true
	return nil
}

func (s *session) commitEE(ourPub, peerPub [32]byte) {
	if s.haveSess || !s.ephLive || !s.havePeerEph {
		return
	}
	send, recv, err := deriveEE(s.ephPriv, s.peerEph, ourPub, peerPub)
	if err != nil {
		return
	}
	s.sendKey = send
	s.recvKey = recv
	wipe32(&s.ephPriv)
	s.ephLive = false
	s.haveSess = true
}

func (e *Engine) sealToPeer(s *session, typ byte, plain []byte) ([]byte, error) {
	if err := s.ensureEph(); err != nil {
		return nil, err
	}
	if err := e.ensureStatic(s); err != nil {
		return nil, err
	}
	s.commitEE(e.discoPub, s.peer.DiscoPub)
	key := s.staticSend
	if s.haveSess {
		key = s.sendKey
	}
	return sealDirect(typ, key, s.ephPub, e.self, s.peer.DeviceID, plain)
}

type openTry struct {
	key  [32]byte
	kind byte
}

func (e *Engine) openFromPeer(s *session, typ byte, pkt []byte) (plain []byte, src, dst uuid.UUID, err error) {
	if err = s.ensureEph(); err != nil {
		return nil, src, dst, err
	}
	if err = e.ensureStatic(s); err != nil {
		return nil, src, dst, err
	}
	src, dst, eph, box, err := splitDirect(pkt, typ)
	if err != nil {
		return nil, src, dst, err
	}
	aad := directAAD(typ, src, dst, eph)
	tries := make([]openTry, 0, 3)
	if s.haveSess {
		tries = append(tries, openTry{key: s.recvKey, kind: 1})
	}
	var tentSend, tentRecv [32]byte
	if !s.haveSess && s.ephLive && eph != ([32]byte{}) {
		tentSend, tentRecv, err = deriveEE(s.ephPriv, eph, e.discoPub, s.peer.DiscoPub)
		if err == nil {
			tries = append(tries, openTry{key: tentRecv, kind: 2})
		}
	}
	if !s.sessSeen {
		tries = append(tries, openTry{key: s.staticRecv, kind: 0})
	}
	var kind byte
	opened := false
	for _, try := range tries {
		plain, err = openKey(try.key, box, aad)
		if err == nil {
			kind = try.kind
			opened = true
			break
		}
	}
	if !opened {
		return nil, src, dst, err
	}
	if kind == 2 {
		s.sendKey = tentSend
		s.recvKey = tentRecv
		wipe32(&s.ephPriv)
		s.ephLive = false
		s.haveSess = true
	}
	if kind == 1 || kind == 2 {
		s.sessSeen = true
	}
	if eph != ([32]byte{}) {
		s.peerEph = eph
		s.havePeerEph = true
	}
	s.commitEE(e.discoPub, s.peer.DiscoPub)
	return plain, src, dst, nil
}
