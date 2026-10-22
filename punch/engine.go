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
// wg-server instead of a peer address.
type Datagram struct {
	Server bool
	To     netip.AddrPort
	Data   []byte
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

	peers map[uuid.UUID]*session
	byIP  map[netip.Addr]uuid.UUID
	want  map[netip.Addr]struct{}
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
}

// NewEngine returns an engine for this device. discoPriv is the punch key.
func NewEngine(self uuid.UUID, discoPriv, wgPub [32]byte, wgIP netip.Addr) *Engine {
	return &Engine{
		self:  self,
		disco: discoPriv,
		wgPub: wgPub,
		wgIP:  wgIP,
		peers: make(map[uuid.UUID]*session),
		byIP:  make(map[netip.Addr]uuid.UUID),
		want:  make(map[netip.Addr]struct{}),
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

// SetPeers replaces the controller directory. Peers that disappear are dropped.
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
func (e *Engine) NoteTraffic(ip netip.Addr) {
	if !ip.IsValid() {
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

// Poll sends reflex requests, rendezvous advertisements, and direct keepalives.
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
		id, ok := e.byIP[ip]
		if !ok || !e.admitsLocked(ip) {
			continue
		}
		s := e.peers[id]
		if s == nil || s.directUp {
			continue
		}
		if !s.lastAdv.IsZero() && now.Sub(s.lastAdv) < RendezvousEvery {
			continue
		}
		if pkt, ok := e.advertiseLocked(s, now); ok {
			r.Sends = append(r.Sends, Datagram{Server: true, Data: pkt})
		}
	}
	for id, s := range e.peers {
		if !s.directUp {
			continue
		}
		if !e.admitsLocked(s.peer.WGIP) {
			r.Down = append(r.Down, id)
			s.directUp = false
			continue
		}
		if now.Sub(s.lastPong) > DirectTimeout {
			r.Down = append(r.Down, id)
			if pkt, err := BuildTeardown(e.disco, s.peer.DiscoPub, e.self, id); err == nil {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt})
			}
			s.directUp = false
			s.pongs = make(map[netip.AddrPort]int)
			continue
		}
		if now.Sub(s.lastProbe) >= Keepalive {
			if pkt, err := e.pingLocked(s, s.direct); err == nil {
				r.Sends = append(r.Sends, Datagram{To: s.direct, Data: pkt})
				s.lastProbe = now
			}
		}
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
	src, dst, _, observed, err := OpenPong(e.disco, s.peer.DiscoPub, pkt)
	if err != nil || src != claimed || dst != e.self {
		return
	}
	via := from
	if observed.IsValid() && observed.Addr().IsValid() {
		// The peer saw us at observed. Our own NAT mapping toward them can
		// differ from the mapping toward the wg-server. Keep their view.
		_ = observed
	}
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
	s.direct = best
	s.directUp = true
	s.lastProbe = now
	r.Up = append(r.Up, Path{Peer: s.peer, Via: best})
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
