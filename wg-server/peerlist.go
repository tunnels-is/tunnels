package wgserver

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
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

// fragKey identifies an in-flight fragmented IPv4 datagram inbound to a local
// receiver: the source that sent it plus the IPv4 identification field. Only
// the first fragment of a datagram carries an L4 header (and thus a port), so
// trailing fragments cannot be port-matched; instead, when a first fragment is
// admitted the receiver records this key, and trailing fragments are admitted
// iff their key was recorded. The source is authenticated by WireGuard (a peer
// can only send from its own IP), so the (remote, id) namespace is per-source
// and one peer cannot forge notes for another.
type fragKey struct {
	remote netip.Addr
	id     uint16
}

// portSet is the set of destination ports a given source may reach on this
// device. all=true means every port (a bare-IP allowlist entry); otherwise
// only the ports in the map are permitted (IP:PORT entries). Protocol is not
// distinguished.
type portSet struct {
	all   bool
	ports map[uint16]struct{}
}

func (s *portSet) contains(port uint16) bool {
	if s.all {
		return true
	}
	_, ok := s.ports[port]
	return ok
}

// aclEntry is one parsed allowlist entry. Exactly one shape is valid:
//   - bare host  ("IP")      → addr set, port 0  : all ports for that source
//   - host:port  ("IP:PORT") → addr set, port>0  : that port for that source
//   - any:port   ("*:PORT")  → anyHost,  port>0  : that port for any source
type aclEntry struct {
	addr    netip.Addr
	port    uint16
	anyHost bool
}

// parseACLEntry parses one wire allowlist token. Invalid tokens return ok=false
// and are skipped by the caller (the control channel is fire-and-forget).
func parseACLEntry(s string) (aclEntry, bool) {
	if rest, ok := strings.CutPrefix(s, "*:"); ok {
		port, err := strconv.ParseUint(rest, 10, 16)
		if err != nil || port == 0 {
			return aclEntry{}, false
		}
		return aclEntry{anyHost: true, port: uint16(port)}, true
	}
	if a, err := netip.ParseAddr(s); err == nil { // bare IP → all ports
		return aclEntry{addr: a}, true
	}
	if ap, err := netip.ParseAddrPort(s); err == nil { // IP:PORT
		if ap.Port() == 0 {
			return aclEntry{}, false
		}
		return aclEntry{addr: ap.Addr(), port: ap.Port()}, true
	}
	return aclEntry{}, false
}

// peer is one local resident's firewall state. The RWMutex guards allowAll
// and the map structures; the per-flow packet counter is atomic so the
// steady-state path (an established flow) only needs a read lock.
type peer struct {
	mu       sync.RWMutex
	v6       netip.Addr // this device's v6 address (for map cleanup); invalid if none
	allowAll bool       // any source may reach this device (overrides allowed)
	allowed  map[netip.Addr]*portSet
	anyPorts map[uint16]struct{} // "*:PORT" — any source may reach these ports
	flows    map[flowKey]*flowRec
	// frags tracks fragmented datagrams whose first fragment this receiver
	// admitted, so trailing fragments (which carry no port) can be matched.
	// Aged by the same cleaner as flows.
	frags map[fragKey]*flowRec
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
		allowed:  make(map[netip.Addr]*portSet),
		anyPorts: make(map[uint16]struct{}),
		flows:    make(map[flowKey]*flowRec),
		frags:    make(map[fragKey]*flowRec),
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

// allowedContains reports whether src may reach dport on this device under the
// static policy: allow-all, an any-host rule for the port ("*:PORT"), or a
// host entry covering the port (bare IP = all ports, or an explicit IP:PORT).
// (Connection-tracked replies are handled separately by flowMatch.)
func (p *peer) allowedContains(src netip.Addr, dport uint16) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.allowAll {
		return true
	}
	if _, ok := p.anyPorts[dport]; ok {
		return true
	}
	if ps, ok := p.allowed[src]; ok {
		return ps.contains(dport)
	}
	return false
}

// allowedAnyPort reports whether src is admitted on ALL ports by the static
// policy — allow-all, or a bare-host ("IP") entry. Such a grant does not depend
// on the destination port, so it can admit a trailing fragment (which carries
// no port) on its own, order-independently. Port-scoped rules ("*:PORT",
// "IP:PORT") are deliberately excluded: they need the head-admitted note.
func (p *peer) allowedAnyPort(src netip.Addr) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.allowAll {
		return true
	}
	if ps, ok := p.allowed[src]; ok {
		return ps.all
	}
	return false
}

// setAllowed replaces the firewall policy (replace-set semantics): the parsed
// allowlist entries plus the allow-all flag. An empty list with allowAll=false
// denies all peer-to-peer ingress. A bare-host entry (port 0) grants all ports
// and supersedes any IP:PORT entries for the same source.
func (p *peer) setAllowed(entries []aclEntry, allowAll bool) {
	allowed := make(map[netip.Addr]*portSet, len(entries))
	anyPorts := make(map[uint16]struct{})
	for _, e := range entries {
		switch {
		case e.anyHost:
			anyPorts[e.port] = struct{}{}
		case e.port == 0: // bare host: all ports
			ps := allowed[e.addr]
			if ps == nil {
				ps = &portSet{}
				allowed[e.addr] = ps
			}
			ps.all = true
		default: // host:port
			ps := allowed[e.addr]
			if ps == nil {
				ps = &portSet{ports: make(map[uint16]struct{})}
				allowed[e.addr] = ps
			}
			if ps.ports == nil {
				ps.ports = make(map[uint16]struct{})
			}
			ps.ports[e.port] = struct{}{}
		}
	}
	p.mu.Lock()
	p.allowed = allowed
	p.anyPorts = anyPorts
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

// noteFragment records that the first fragment of the datagram identified by k
// was admitted to this receiver, so its trailing fragments will match. Mirrors
// touchFlow (read-lock fast path, write lock only to insert) and is bounded by
// flowSoftCap.
func (p *peer) noteFragment(k fragKey) {
	p.mu.RLock()
	if r, ok := p.frags[k]; ok {
		r.packets.Add(1)
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	p.mu.Lock()
	if r, ok := p.frags[k]; ok { // re-check after lock upgrade
		r.packets.Add(1)
	} else if len(p.frags) < flowSoftCap {
		r := &flowRec{}
		r.packets.Store(1)
		p.frags[k] = r
	}
	p.mu.Unlock()
}

// fragmentAdmitted reports whether the first fragment of the datagram
// identified by k was previously admitted to this receiver, refreshing
// liveness so the entry survives while the datagram is still arriving.
func (p *peer) fragmentAdmitted(k fragKey) bool {
	p.mu.RLock()
	r, ok := p.frags[k]
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
		for k, r := range p.frags {
			if pk := r.packets.Load(); pk == r.prev {
				delete(p.frags, k)
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
