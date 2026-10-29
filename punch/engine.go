package punch

import (
	"net/netip"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// Keepalive is how often a direct path is re-probed. Short enough that
	// a carrier NAT does not forget the mapping between WireGuard keepalives.
	Keepalive = 10 * time.Second
	// DirectTimeout drops a direct path that has not ponged.
	DirectTimeout = 25 * time.Second
	// RendezvousEvery limits how often candidates are re-advertised.
	RendezvousEvery = 5 * time.Second
	// ReflexEvery refreshes the mapping measured at the wg-server.
	ReflexEvery = 15 * time.Second
	// PongsToTrust is how many pongs from one address promote it.
	PongsToTrust = 2
)

// Peer is one device the controller says we may punch.
type Peer struct {
	DeviceID uuid.UUID
	DiscoPub [32]byte
	WGPub    [32]byte
	WGIP     netip.Addr
}

// Datagram is one punch packet to send. Server is set when it goes to the
// wg-server instead of a peer address. Tunnel is set when it goes through
// the existing WireGuard tunnel to To, which is the other client's tunnel
// address. MTU marks a padded path-MTU probe; Peer is the device the probe
// measures.
type Datagram struct {
	Server bool
	Tunnel bool
	To     netip.AddrPort
	Data   []byte
	MTU    bool
	Peer   uuid.UUID
}

// Path is a direct address that has answered twice.
type Path struct {
	Peer Peer
	Via  netip.AddrPort
}

// Result is what an engine step wants the caller to do.
type Result struct {
	Sends []Datagram
	Up    []Path
	Down  []uuid.UUID
	// MTU is set when a trusted path's send direction has a measured inner MTU.
	MTU []PathMTU
}

// Engine is the client punch state machine. It does not own a socket.
type Engine struct {
	mu sync.Mutex

	self     uuid.UUID
	disco    [32]byte
	wgPub    [32]byte
	wgIP     netip.Addr
	server   [32]byte
	haveSrv  bool
	reflex   netip.AddrPort
	reflexAt time.Time

	allowAll bool
	allow    []string
	discover bool

	peers   map[uuid.UUID]*session
	byIP    map[netip.Addr]uuid.UUID
	want    map[netip.Addr]struct{}
	introAt map[netip.Addr]time.Time
}

type session struct {
	peer       Peer
	cands      []netip.AddrPort
	pongs      map[netip.AddrPort]int
	direct     netip.AddrPort
	directUp   bool
	lastPong   time.Time
	lastProbe  time.Time
	lastAdv    time.Time
	lastEpoch  uint64
	repliedFor uint64
	txid       uint64

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

// NewEngine returns an engine for this device. discoPriv is the punch key.
func NewEngine(self uuid.UUID, discoPriv, wgPub [32]byte, wgIP netip.Addr) *Engine {
	return &Engine{
		self:    self,
		disco:   discoPriv,
		wgPub:   wgPub,
		wgIP:    wgIP,
		peers:   make(map[uuid.UUID]*session),
		byIP:    make(map[netip.Addr]uuid.UUID),
		want:    make(map[netip.Addr]struct{}),
		introAt: make(map[netip.Addr]time.Time),
	}
}

// SetServerDisco records the wg-server disco key used to seal reflex requests.
func (e *Engine) SetServerDisco(pub [32]byte) {
	e.mu.Lock()
	e.server = pub
	e.haveSrv = true
	e.mu.Unlock()
}

// SetLocalPolicy records which tunnel IPs this device admits on every port.
// firewallOff admits every peer. Peers that stop qualifying are dropped and,
// when a direct path was up, told to drop it too.
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
			if pkt, err := BuildTeardown(e.disco, s.peer.DiscoPub, e.self, id); err == nil && s.direct.IsValid() {
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

// SetPeers installs peers the caller already knows. The live client does not
// use it: a peer is learned from the introduction that arrives on its tunnel
// IP. Peers that disappear are dropped.
func (e *Engine) SetPeers(peers []Peer) Result {
	e.mu.Lock()
	defer e.mu.Unlock()
	var r Result
	next := make(map[uuid.UUID]Peer, len(peers))
	for _, p := range peers {
		if p.DeviceID == e.self || !p.WGIP.IsValid() || !p.WGIP.Is4() {
			continue
		}
		next[p.DeviceID] = p
	}
	for id, s := range e.peers {
		p, ok := next[id]
		if !ok || p.WGPub != s.peer.WGPub || p.DiscoPub != s.peer.DiscoPub || p.WGIP != s.peer.WGIP {
			if s.directUp {
				r.Down = append(r.Down, id)
				if pkt, err := BuildTeardown(e.disco, s.peer.DiscoPub, e.self, id); err == nil && s.direct.IsValid() {
					r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt})
				}
			}
			delete(e.peers, id)
			delete(e.byIP, s.peer.WGIP)
		}
	}
	e.byIP = make(map[netip.Addr]uuid.UUID, len(next))
	for id, p := range next {
		e.byIP[p.WGIP] = id
		if s, ok := e.peers[id]; ok {
			s.peer = p
			continue
		}
		e.peers[id] = &session{peer: p, pongs: make(map[netip.AddrPort]int)}
	}
	return r
}

// NoteTraffic asks the engine to punch the device that owns this tunnel IP.
// Only private IPv4 is a candidate. A public destination is WAN traffic, and
// an introduction must not be sent there.
func (e *Engine) NoteTraffic(ip netip.Addr) {
	if !ip.Is4() || !ip.IsPrivate() {
		return
	}
	e.mu.Lock()
	e.want[ip] = struct{}{}
	e.mu.Unlock()
}

// Direct reports the current direct address for a tunnel IP, if any.
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

// Poll sends reflex requests, tunnel introductions, and direct keepalives.
func (e *Engine) Poll(now time.Time) Result {
	e.mu.Lock()
	defer e.mu.Unlock()
	var r Result
	if e.haveSrv && (e.reflexAt.IsZero() || now.Sub(e.reflexAt) >= ReflexEvery) {
		if pkt, err := BuildReflexReq(e.self, e.disco, e.server, now); err == nil {
			r.Sends = append(r.Sends, Datagram{Server: true, Data: pkt})
			e.reflexAt = now
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
		if len(s.cands) > 0 && (s.lastProbe.IsZero() || now.Sub(s.lastProbe) >= Keepalive) {
			for _, c := range s.cands {
				if ping, err := e.pingLocked(s, c); err == nil {
					r.Sends = append(r.Sends, Datagram{To: c, Data: ping})
				}
			}
			s.lastProbe = now
		}
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
		if now.Sub(s.lastPong) > DirectTimeout {
			r.Down = append(r.Down, id)
			if pkt, err := BuildTeardown(e.disco, s.peer.DiscoPub, e.self, id); err == nil {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt})
			}
			s.directUp = false
			s.pongs = make(map[netip.AddrPort]int)
			s.resetMTU()
			continue
		}
		if now.Sub(s.lastProbe) >= Keepalive {
			if pkt, err := e.pingLocked(s, s.direct); err == nil {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt})
				s.lastProbe = now
			}
		}
		e.pollMTULocked(s, &r, now)
	}
	return r
}

// OnPacket handles one datagram from from.
func (e *Engine) OnPacket(from netip.AddrPort, pkt []byte, now time.Time) Result {
	e.mu.Lock()
	defer e.mu.Unlock()
	var r Result
	switch TypeOf(pkt) {
	case TypeReflexResp:
		id, ap, err := ParseReflexResp(pkt)
		if err != nil || id != e.self || !ap.IsValid() {
			return r
		}
		e.reflex = ap
		e.reflexAt = now
	case TypeIntro:
		e.onIntroLocked(&r, pkt, now)
	case TypeRendezvous:
		e.onRendezvousLocked(&r, pkt, now)
	case TypePing:
		e.onPingLocked(&r, from, pkt)
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

func (e *Engine) onIntroLocked(r *Result, pkt []byte, now time.Time) {
	msg, err := ParseIntro(pkt)
	if err != nil || msg.Src == e.self || !e.punchableLocked(msg.WGIP) {
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
		s = &session{
			peer:  Peer{DeviceID: msg.Src, DiscoPub: msg.DiscoPub, WGPub: msg.WGPub, WGIP: msg.WGIP},
			pongs: make(map[netip.AddrPort]int),
		}
		e.peers[msg.Src] = s
		e.byIP[msg.WGIP] = msg.Src
	}
	s.cands = append([]netip.AddrPort(nil), msg.Candidates...)
	for _, c := range s.cands {
		if !c.IsValid() {
			continue
		}
		if ping, err := e.pingLocked(s, c); err == nil {
			r.Sends = append(r.Sends, Datagram{To: c, Data: ping})
		}
	}
	if len(s.cands) > 0 {
		s.lastProbe = now
	}
	if s.lastAdv.IsZero() || now.Sub(s.lastAdv) >= RendezvousEvery {
		if reply, ok := e.introPacketLocked(); ok {
			r.Sends = append(r.Sends, Datagram{Tunnel: true, To: netip.AddrPortFrom(msg.WGIP, IntroPort), Data: reply})
			s.lastAdv = now
		}
	}
}

func (e *Engine) onRendezvousLocked(r *Result, pkt []byte, now time.Time) {
	body := pkt
	if len(body) < headerLen+idLen*2 {
		return
	}
	src := uuid.UUID{}
	copy(src[:], body[headerLen:headerLen+idLen])
	s := e.peers[src]
	if s == nil || !e.admitsLocked(s.peer.WGIP) {
		return
	}
	msg, err := OpenRendezvous(e.disco, s.peer.DiscoPub, pkt)
	if err != nil || msg.Dst != e.self || msg.WGPub != s.peer.WGPub || msg.WGIP != s.peer.WGIP {
		return
	}
	if msg.Epoch < s.lastEpoch {
		return
	}
	s.lastEpoch = msg.Epoch
	s.cands = append([]netip.AddrPort(nil), msg.Candidates...)
	for _, c := range s.cands {
		if pkt, err := e.pingLocked(s, c); err == nil {
			r.Sends = append(r.Sends, Datagram{To: c, Data: pkt})
		}
	}
	if s.lastAdv.IsZero() || now.Sub(s.lastAdv) >= RendezvousEvery {
		if adv, ok := e.advertiseLocked(s, now); ok {
			r.Sends = append(r.Sends, Datagram{Server: true, Data: adv})
			s.repliedFor = msg.Epoch
		}
	}
}

func (e *Engine) onPingLocked(r *Result, from netip.AddrPort, pkt []byte) {
	if len(pkt) < headerLen+idLen {
		return
	}
	var claimed uuid.UUID
	copy(claimed[:], pkt[headerLen:headerLen+idLen])
	s := e.peers[claimed]
	if s == nil || !e.admitsLocked(s.peer.WGIP) {
		return
	}
	src, dst, txid, err := OpenPing(e.disco, s.peer.DiscoPub, pkt)
	if err != nil || src != claimed || dst != e.self {
		return
	}
	fresh := !containsAddr(s.cands, from)
	if fresh {
		s.cands = append(s.cands, from)
	}
	pong, err := BuildPong(e.disco, s.peer.DiscoPub, e.self, s.peer.DeviceID, txid, from)
	if err != nil {
		return
	}
	r.Sends = append(r.Sends, Datagram{To: from, Data: pong})
	if fresh {
		if ping, err := e.pingLocked(s, from); err == nil {
			r.Sends = append(r.Sends, Datagram{To: from, Data: ping})
		}
	}
}

func (e *Engine) onPongLocked(r *Result, from netip.AddrPort, pkt []byte, now time.Time) {
	if len(pkt) < headerLen+idLen {
		return
	}
	var claimed uuid.UUID
	copy(claimed[:], pkt[headerLen:headerLen+idLen])
	s := e.peers[claimed]
	if s == nil || !e.admitsLocked(s.peer.WGIP) {
		return
	}
	src, dst, txid, observed, err := OpenPong(e.disco, s.peer.DiscoPub, pkt)
	if err != nil || src != claimed || dst != e.self {
		return
	}
	via := from
	if observed.IsValid() && observed.Addr().IsValid() {
		// The peer saw us at observed. Our own NAT mapping toward them can
		// differ from the mapping toward the wg-server. Keep their view.
		_ = observed
	}
	// Match a padded probe before the "already direct" return. An ignored
	// padded ping must not be what keeps or drops the path; keepalives do that.
	e.onMTUPongLocked(s, txid, r, now)
	s.pongs[via]++
	s.lastPong = now
	if s.pongs[via] == 1 {
		if ping, err := e.pingLocked(s, via); err == nil {
			r.Sends = append(r.Sends, Datagram{To: via, Data: ping})
		}
	}
	if s.pongs[via] < PongsToTrust {
		return
	}
	best := e.bestLocked(s, via)
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
	if len(pkt) < headerLen+idLen {
		return
	}
	var claimed uuid.UUID
	copy(claimed[:], pkt[headerLen:headerLen+idLen])
	s := e.peers[claimed]
	if s == nil {
		return
	}
	src, dst, err := OpenTeardown(e.disco, s.peer.DiscoPub, pkt)
	if err != nil || src != claimed || dst != e.self {
		return
	}
	if s.directUp {
		r.Down = append(r.Down, claimed)
	}
	s.directUp = false
	s.pongs = make(map[netip.AddrPort]int)
	s.resetMTU()
}

func (e *Engine) advertiseLocked(s *session, now time.Time) ([]byte, bool) {
	if !e.reflex.IsValid() {
		return nil, false
	}
	var cands []netip.AddrPort
	cands = append(cands, e.reflex)
	pkt, err := BuildRendezvous(e.disco, s.peer.DiscoPub, Rendezvous{
		Src:        e.self,
		Dst:        s.peer.DeviceID,
		Epoch:      uint64(now.UnixNano()),
		WGPub:      e.wgPub,
		WGIP:       e.wgIP,
		Candidates: cands,
	})
	if err != nil {
		return nil, false
	}
	s.lastAdv = now
	return pkt, true
}

func (e *Engine) pingLocked(s *session, to netip.AddrPort) ([]byte, error) {
	s.txid++
	return BuildPing(e.disco, s.peer.DiscoPub, e.self, s.peer.DeviceID, s.txid)
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
