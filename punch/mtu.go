package punch

import (
	"time"

	"github.com/google/uuid"
)

const (
	// MinInnerMTU is the smallest tunnel MTU we will install.
	MinInnerMTU = 1280
	// DefaultInnerMTU is the IPv6-safe fallback when discovery gets no answer.
	DefaultInnerMTU = 1420
	// MaxOuterMTU is the largest outer IP packet a probe may be. A later
	// direct peer on a 1500-byte path must still fit under the interface MTU.
	MaxOuterMTU = 1500
	// IPv4WGOverhead is outer IPv4 + UDP + WireGuard (20+8+32).
	IPv4WGOverhead = 60
	// IPv6WGOverhead is outer IPv6 + UDP + WireGuard (40+8+32).
	IPv6WGOverhead = 80
	// MTUProbeTimeout is how long a padded ping may go unanswered.
	// The punch loop polls once a second, so this is noticed on the next ticks.
	MTUProbeTimeout = 2 * time.Second

	mtuSearchSteps = 16
)

// Overhead is the outer IP, UDP, and WireGuard header cost for an inner packet.
func Overhead(v6 bool) int {
	if v6 {
		return IPv6WGOverhead
	}
	return IPv4WGOverhead
}

// MinOuter is the smallest outer packet that can carry MinInnerMTU.
func MinOuter(v6 bool) int {
	return MinInnerMTU + Overhead(v6)
}

// ProbePayloadLen is the UDP payload length of an outer IP packet of size outer.
func ProbePayloadLen(outer int, v6 bool) int {
	n := 28
	if v6 {
		n = 48
	}
	return outer - n
}

// PingPlainForOuter is the sealed-ping plaintext length that occupies an
// outer IP packet of size outer. The first 8 bytes are the txid.
func PingPlainForOuter(outer int, v6 bool) (int, bool) {
	plain := ProbePayloadLen(outer, v6) - directPacketLen(0)
	if plain < 8 || directPacketLen(plain) > MaxPacket {
		return 0, false
	}
	return plain, true
}

// PathMTU is one peer's measured inner MTU, from this side's send path.
type PathMTU struct {
	Peer  Peer
	Inner int
}

func (s *session) resetMTU() {
	s.mtuActive = false
	s.mtuDone = false
	s.mtuV6 = false
	s.mtuLo = 0
	s.mtuHi = 0
	s.mtuOuter = 0
	s.mtuTx = 0
	s.mtuSent = time.Time{}
	s.mtuFit = 0
	s.mtuRetried = false
	s.mtuSteps = 0
}

// SetDiscoverMTU turns on per-peer probes after a direct path is trusted.
// A configured interface MTU leaves this off: the interface already caps every peer.
func (e *Engine) SetDiscoverMTU(on bool) {
	e.mu.Lock()
	e.discover = on
	e.mu.Unlock()
}

// MTUUnsent reports that the in-flight probe for peer never left the host
// (the local stack refused it, typically EMSGSIZE). That size is too big
// and is not retried.
func (e *Engine) MTUUnsent(peer uuid.UUID, now time.Time) Result {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.peers[peer]
	var r Result
	if s == nil || !s.mtuActive || s.mtuOuter == 0 {
		return r
	}
	s.mtuRetried = true
	e.shrinkMTULocked(s, &r, now)
	return r
}

// armMTULocked starts a search on a newly trusted path. fresh drops a
// previous result so a different endpoint is measured again.
func (e *Engine) armMTULocked(s *session, r *Result, now time.Time, fresh bool) {
	if !e.discover || s == nil || !s.direct.IsValid() {
		return
	}
	if fresh {
		s.resetMTU()
	}
	if s.mtuDone || s.mtuActive {
		return
	}
	v6 := s.direct.Addr().Is6()
	s.mtuV6 = v6
	s.mtuLo = MinOuter(v6)
	s.mtuHi = MaxOuterMTU
	if s.mtuHi < s.mtuLo {
		s.mtuDone = true
		return
	}
	s.mtuActive = true
	e.launchMTULocked(s, r, s.mtuHi, now)
}

func (e *Engine) pollMTULocked(s *session, r *Result, now time.Time) {
	if !s.mtuActive || s.mtuOuter == 0 || s.mtuSent.IsZero() {
		return
	}
	if now.Sub(s.mtuSent) < MTUProbeTimeout {
		return
	}
	// One retry, and only for the opening 1500-byte probe. Later steps are single-shot.
	if s.mtuOuter == MaxOuterMTU && !s.mtuRetried {
		s.mtuRetried = true
		e.launchMTULocked(s, r, s.mtuOuter, now)
		return
	}
	e.shrinkMTULocked(s, r, now)
}

func (e *Engine) onMTUPongLocked(s *session, txid uint64, r *Result, now time.Time) {
	if !s.mtuActive || s.mtuOuter == 0 || txid != s.mtuTx {
		return
	}
	e.growMTULocked(s, r, now)
}

func (e *Engine) launchMTULocked(s *session, r *Result, outer int, now time.Time) {
	s.mtuSteps++
	if s.mtuSteps > mtuSearchSteps {
		e.finishMTULocked(s, r)
		return
	}
	plain, ok := PingPlainForOuter(outer, s.mtuV6)
	if !ok {
		e.finishMTULocked(s, r)
		return
	}
	s.txid++
	pkt, err := BuildPingPad(e.disco, s.peer.DiscoPub, e.self, s.peer.DeviceID, s.txid, plain)
	if err != nil {
		e.finishMTULocked(s, r)
		return
	}
	s.mtuOuter = outer
	s.mtuTx = s.txid
	s.mtuSent = now
	r.Sends = append(r.Sends, Datagram{
		To:   s.direct,
		Data: pkt,
		MTU:  true,
		Peer: s.peer.DeviceID,
	})
}

func (e *Engine) growMTULocked(s *session, r *Result, now time.Time) {
	probed := s.mtuOuter
	s.mtuFit = probed
	s.mtuLo = probed + 1
	s.mtuOuter = 0
	if s.mtuLo > s.mtuHi {
		e.finishMTULocked(s, r)
		return
	}
	next := (s.mtuLo + s.mtuHi) / 2
	if next <= probed {
		e.finishMTULocked(s, r)
		return
	}
	e.launchMTULocked(s, r, next, now)
}

func (e *Engine) shrinkMTULocked(s *session, r *Result, now time.Time) {
	s.mtuHi = s.mtuOuter - 1
	s.mtuOuter = 0
	if s.mtuHi < s.mtuLo {
		e.finishMTULocked(s, r)
		return
	}
	next := (s.mtuLo + s.mtuHi) / 2
	if next < s.mtuLo {
		e.finishMTULocked(s, r)
		return
	}
	e.launchMTULocked(s, r, next, now)
}

func (e *Engine) finishMTULocked(s *session, r *Result) {
	fit := s.mtuFit
	v6 := s.mtuV6
	s.mtuActive = false
	s.mtuDone = true
	s.mtuOuter = 0
	s.mtuSent = time.Time{}
	if fit <= 0 {
		return
	}
	inner := fit - Overhead(v6)
	if inner < MinInnerMTU {
		return
	}
	r.MTU = append(r.MTU, PathMTU{Peer: s.peer, Inner: inner})
}
