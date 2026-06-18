package wgserver

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

// Per-peer firewall state + connection tracking for the local WG subnet.
//
// The firewall keeps one `peer` entry per local resident, found in O(1):
// IPv4 residents live in a flat slice indexed by their offset within the
// local subnet (10.0.0.0 → slot 0, 10.0.0.1 → slot 1, …); IPv6 residents
// (which the controller does NOT assign with offset-parity to IPv4) live in
// a small map keyed by address. A dual-stack device shares one `peer` from
// both, so it has a single allowlist and flow table across families.
//
// Each `peer` holds:
//   - allowed: announced allowlist — who may INITIATE traffic to this device.
//   - flows:   connection tracking — remote ends this device has itself
//     initiated to, so return traffic is admitted without an allowlist entry.
//
// Egress (a local resident sends): record/refresh the flow in the sender's
// entry, opening its own return path. Ingress (a local resident receives):
// admit iff the source is in the receiver's allowlist OR matches a tracked
// flow the receiver opened.
//
// Cross-server needs no state sync: the sender's server records the outbound
// half, the receiver's server enforces the inbound half, each consulting only
// its own residents.

const (
	// flowCleanInterval is the conntrack aging period. A flow with no traffic
	// in either direction for a full interval is dropped, so the effective
	// idle timeout is 1–2 intervals. Generous on purpose — memory is bounded
	// by live flows, not by the timeout.
	flowCleanInterval = 15 * time.Minute

	// flowSoftCap bounds a single peer's flow map. A local host spraying
	// random source ports cannot grow its table without limit; past the cap
	// new flows are not tracked (their return traffic falls back to the
	// allowlist) until the cleaner reclaims idle entries. High enough that
	// legitimate use never reaches it.
	flowSoftCap = 50000
)

// flowKey identifies a connection from the local resident's point of view:
// `remote` is the other end, `lport` is the resident's own port. Built that
// way so an outbound packet and its reply produce the same key. netip.Addr is
// comparable and handles v4/v6 uniformly, so it works directly as a map key.
type flowKey struct {
	remote netip.Addr
	rport  uint16
	lport  uint16
	proto  byte
}

// flowRec counts packets for liveness. packets is bumped (atomically, under
// the peer's RLock) on every packet in either direction; the cleaner compares
// it against prev (which only the cleaner touches, under the peer's Lock) to
// decide whether the flow saw traffic since the last pass.
type flowRec struct {
	packets atomic.Uint64
	prev    uint64
}

// peer is one local resident's firewall state. The RWMutex guards allowAll
// and the map structures; the per-flow packet counter is atomic so the
// steady-state path (an established flow) only needs a read lock.
type peer struct {
	mu       sync.RWMutex
	v6       netip.Addr // this device's v6 address (for map cleanup); invalid if none
	allowAll bool       // any source may reach this device (overrides allowed)
	allowed  map[netip.Addr]struct{}
	flows    map[flowKey]*flowRec
}

var (
	fwSubnet4 netip.Prefix
	fwBase4   uint32
	fwSubnet6 netip.Prefix

	// fwV4Slots is the flat spine: one slot per local-subnet offset,
	// pre-allocated at init. nil slot ⇒ not a resident of this server.
	fwV4Slots []atomic.Pointer[peer]

	// fwV6 maps a resident's v6 address to its (shared) peer entry.
	fwMu sync.RWMutex
	fwV6 map[netip.Addr]*peer

	fwStop chan struct{}
)

// initPeerList sizes the firewall tables to the local WireGuard subnet. The
// v4 spine is one pointer per subnet address, so the subnet may be no larger
// than /16 (65536 slots, 512 KB spine).
func initPeerList(subnet4, subnet6 string) error {
	p4, err := netip.ParsePrefix(subnet4)
	if err != nil {
		return fmt.Errorf("parse WireGuard subnet: %w", err)
	}
	p4 = p4.Masked()
	if !p4.Addr().Is4() {
		return fmt.Errorf("WireGuard subnet must be IPv4: %s", subnet4)
	}
	if p4.Bits() < 16 {
		return fmt.Errorf("WireGuard subnet larger than /16 is unsupported: %s", subnet4)
	}
	fwSubnet4 = p4
	fwBase4 = binary.BigEndian.Uint32(p4.Addr().AsSlice())
	fwV4Slots = make([]atomic.Pointer[peer], 1<<(32-p4.Bits()))

	fwMu.Lock()
	fwV6 = make(map[netip.Addr]*peer)
	fwMu.Unlock()

	if subnet6 != "" {
		p6, err := netip.ParsePrefix(subnet6)
		if err != nil {
			return fmt.Errorf("parse WireGuard subnet6: %w", err)
		}
		fwSubnet6 = p6.Masked()
	} else {
		fwSubnet6 = netip.Prefix{}
	}
	return nil
}

func startFlowCleaner() {
	fwStop = make(chan struct{})
	go func() {
		t := time.NewTicker(flowCleanInterval)
		defer t.Stop()
		for {
			select {
			case <-fwStop:
				return
			case <-t.C:
				cleanFlows()
			}
		}
	}()
}

func stopFlowCleaner() {
	if fwStop != nil {
		close(fwStop)
		fwStop = nil
	}
}

// v4Offset returns a's index within the local v4 subnet.
func v4Offset(a netip.Addr) (uint32, bool) {
	if !fwSubnet4.IsValid() || !a.Is4() || !fwSubnet4.Contains(a) {
		return 0, false
	}
	return binary.BigEndian.Uint32(a.AsSlice()) - fwBase4, true
}

// fwClassify reports whether a belongs to a local WG subnet and returns its
// peer entry if one is currently installed (nil otherwise).
func fwClassify(a netip.Addr) (*peer, bool) {
	if a.Is4() {
		off, ok := v4Offset(a)
		if !ok {
			return nil, false
		}
		return fwV4Slots[off].Load(), true
	}
	if fwSubnet6.IsValid() && fwSubnet6.Contains(a) {
		fwMu.RLock()
		p := fwV6[a]
		fwMu.RUnlock()
		return p, true
	}
	return nil, false
}

// resetPeer installs a fresh, empty entry for a resident on (re)connect or IP
// reuse, discarding any allowlist or flows left by a previous holder of the
// address. ips is the device's (v4, v6) — empty/invalid strings are ignored.
func resetPeer(ips ...string) {
	if fwV4Slots == nil {
		return
	}
	p := &peer{
		allowed: make(map[netip.Addr]struct{}),
		flows:   make(map[flowKey]*flowRec),
	}
	var v4 netip.Addr
	for _, s := range ips {
		a, err := netip.ParseAddr(s)
		if err != nil {
			continue
		}
		switch {
		case a.Is4():
			v4 = a
		case a.Is6() && fwSubnet6.IsValid() && fwSubnet6.Contains(a):
			p.v6 = a
		}
	}

	if v4.IsValid() {
		off, ok := v4Offset(v4)
		if ok {
			old := fwV4Slots[off].Swap(p)
			// Reclaim the previous resident's v6 map entry, if any.
			if old != nil && old.v6.IsValid() {
				fwMu.Lock()
				if fwV6[old.v6] == old {
					delete(fwV6, old.v6)
				}
				fwMu.Unlock()
			}
		}
	}
	if p.v6.IsValid() {
		fwMu.Lock()
		fwV6[p.v6] = p
		fwMu.Unlock()
	}
}

// allowedContains reports whether src is permitted by the static policy:
// allow-all, or an explicit allowlist entry. (Connection-tracked replies are
// handled separately by flowMatch.)
func (p *peer) allowedContains(ip netip.Addr) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.allowAll {
		return true
	}
	_, ok := p.allowed[ip]
	return ok
}

// setAllowed replaces the firewall policy (replace-set semantics): the
// allowlist plus the allow-all flag. An empty list with allowAll=false denies
// all peer-to-peer ingress.
func (p *peer) setAllowed(ips []netip.Addr, allowAll bool) {
	m := make(map[netip.Addr]struct{}, len(ips))
	for _, ip := range ips {
		m[ip] = struct{}{}
	}
	p.mu.Lock()
	p.allowed = m
	p.allowAll = allowAll
	p.mu.Unlock()
}

// touchFlow finds or creates the flow for k and counts the packet. The
// steady-state hit takes only a read lock; the write lock is taken only to
// insert a new flow.
func (p *peer) touchFlow(k flowKey) {
	p.mu.RLock()
	if r, ok := p.flows[k]; ok {
		r.packets.Add(1)
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	p.mu.Lock()
	if r, ok := p.flows[k]; ok { // re-check after lock upgrade
		r.packets.Add(1)
	} else if len(p.flows) < flowSoftCap {
		r := &flowRec{}
		r.packets.Store(1)
		p.flows[k] = r
	}
	p.mu.Unlock()
}

// flowMatch reports whether k is a tracked flow this peer opened, counting
// the reply so the flow stays alive while either side is talking.
func (p *peer) flowMatch(k flowKey) bool {
	p.mu.RLock()
	r, ok := p.flows[k]
	if ok {
		r.packets.Add(1)
	}
	p.mu.RUnlock()
	return ok
}

// cleanFlows ages conntrack across all residents: a flow with no traffic
// since the last pass is dropped. Ranges only the v4 spine — every resident
// has a v4 address, so v6 map entries are aliases already covered.
func cleanFlows() {
	for i := range fwV4Slots {
		p := fwV4Slots[i].Load()
		if p == nil {
			continue
		}
		p.mu.Lock()
		for k, r := range p.flows {
			if pk := r.packets.Load(); pk == r.prev {
				delete(p.flows, k)
			} else {
				r.prev = pk
			}
		}
		p.mu.Unlock()
	}
}

// peerListSnapshot returns dst IP → allowed src IPs, for diagnostics.
func peerListSnapshot() map[netip.Addr][]netip.Addr {
	out := make(map[netip.Addr][]netip.Addr)
	for i := range fwV4Slots {
		p := fwV4Slots[i].Load()
		if p == nil {
			continue
		}
		p.mu.RLock()
		if len(p.allowed) > 0 {
			srcs := make([]netip.Addr, 0, len(p.allowed))
			for src := range p.allowed {
				srcs = append(srcs, src)
			}
			out[addrAt(uint32(i))] = srcs
		}
		p.mu.RUnlock()
	}
	return out
}

func addrAt(off uint32) netip.Addr {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], fwBase4+off)
	return netip.AddrFrom4(b)
}

// l4Ports extracts source/destination ports for TCP and UDP; other protocols
// (ICMP, etc.) report 0,0 — one flow per remote host-pair-per-protocol.
func l4Ports(proto byte, l4 []byte) (sport, dport uint16) {
	switch proto {
	case 6, protoUDP: // TCP, UDP
		if len(l4) >= 4 {
			return binary.BigEndian.Uint16(l4[0:2]), binary.BigEndian.Uint16(l4[2:4])
		}
	}
	return 0, 0
}
