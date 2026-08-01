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

const (
	flowCleanInterval = 15 * time.Minute

	flowSoftCap = 50000
)

type flowKey struct {
	remote netip.Addr
	rport  uint16
	lport  uint16
	proto  byte
}

type flowRec struct {
	packets atomic.Uint64
	prev    uint64
}

type fragKey struct {
	remote netip.Addr
	id     uint32
}

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

type aclEntry struct {
	addr    netip.Addr
	port    uint16
	anyHost bool
}

func parseACLEntry(s string) (aclEntry, bool) {
	if rest, ok := strings.CutPrefix(s, "*:"); ok {
		port, err := strconv.ParseUint(rest, 10, 16)
		if err != nil || port == 0 {
			return aclEntry{}, false
		}
		return aclEntry{anyHost: true, port: uint16(port)}, true
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return aclEntry{addr: a}, true
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		if ap.Port() == 0 {
			return aclEntry{}, false
		}
		return aclEntry{addr: ap.Addr(), port: ap.Port()}, true
	}
	return aclEntry{}, false
}

type peer struct {
	mu       sync.RWMutex
	v6       netip.Addr
	allowAll bool
	allowed  map[netip.Addr]*portSet
	anyPorts map[uint16]struct{}
	flows    map[flowKey]*flowRec

	frags map[fragKey]*flowRec
}

var (
	fwSubnet4 netip.Prefix
	fwBase4   uint32
	fwSubnet6 netip.Prefix

	fwV4Slots []atomic.Pointer[peer]

	fwMu sync.RWMutex
	fwV6 map[netip.Addr]*peer

	fwStop chan struct{}
)

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

func v4Offset(a netip.Addr) (uint32, bool) {
	if !fwSubnet4.IsValid() || !a.Is4() || !fwSubnet4.Contains(a) {
		return 0, false
	}
	return binary.BigEndian.Uint32(a.AsSlice()) - fwBase4, true
}

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

func policyAdmits(allowAll bool, allowed map[netip.Addr]*portSet, anyPorts map[uint16]struct{}, src netip.Addr, port uint16) bool {
	if allowAll {
		return true
	}
	if _, ok := anyPorts[port]; ok {
		return true
	}
	if ps, ok := allowed[src]; ok {
		return ps.contains(port)
	}
	return false
}

func policyAdmitsAny(allowAll bool, allowed map[netip.Addr]*portSet, anyPorts map[uint16]struct{}, src netip.Addr) bool {
	if allowAll || len(anyPorts) > 0 {
		return true
	}
	_, ok := allowed[src]
	return ok
}

func (p *peer) allowedContains(src netip.Addr, dport uint16) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return policyAdmits(p.allowAll, p.allowed, p.anyPorts, src, dport)
}

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

func (p *peer) setAllowed(entries []aclEntry, allowAll bool) {
	allowed := make(map[netip.Addr]*portSet, len(entries))
	anyPorts := make(map[uint16]struct{})
	for _, e := range entries {
		switch {
		case e.anyHost:
			anyPorts[e.port] = struct{}{}
		case e.port == 0:
			ps := allowed[e.addr]
			if ps == nil {
				ps = &portSet{}
				allowed[e.addr] = ps
			}
			ps.all = true
		default:
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
	oldAllowAll, oldAllowed, oldAnyPorts := p.allowAll, p.allowed, p.anyPorts
	p.allowed = allowed
	p.anyPorts = anyPorts
	p.allowAll = allowAll

	for k := range p.flows {
		if policyAdmits(oldAllowAll, oldAllowed, oldAnyPorts, k.remote, k.lport) &&
			!policyAdmits(allowAll, allowed, anyPorts, k.remote, k.lport) {
			delete(p.flows, k)
		}
	}
	for k := range p.frags {
		if policyAdmitsAny(oldAllowAll, oldAllowed, oldAnyPorts, k.remote) &&
			!policyAdmitsAny(allowAll, allowed, anyPorts, k.remote) {
			delete(p.frags, k)
		}
	}
	p.mu.Unlock()
}

func (p *peer) touchFlow(k flowKey) {
	p.mu.RLock()
	if r, ok := p.flows[k]; ok {
		r.packets.Add(1)
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	p.mu.Lock()
	if r, ok := p.flows[k]; ok {
		r.packets.Add(1)
	} else if len(p.flows) < flowSoftCap {
		r := &flowRec{}
		r.packets.Store(1)
		p.flows[k] = r
	}
	p.mu.Unlock()
}

func (p *peer) flowMatch(k flowKey) bool {
	p.mu.RLock()
	r, ok := p.flows[k]
	if ok {
		r.packets.Add(1)
	}
	p.mu.RUnlock()
	return ok
}

func (p *peer) noteFragment(k fragKey) {
	p.mu.RLock()
	if r, ok := p.frags[k]; ok {
		r.packets.Add(1)
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	p.mu.Lock()
	if r, ok := p.frags[k]; ok {
		r.packets.Add(1)
	} else if len(p.frags) < flowSoftCap {
		r := &flowRec{}
		r.packets.Store(1)
		p.frags[k] = r
	}
	p.mu.Unlock()
}

func (p *peer) fragmentAdmitted(k fragKey) bool {
	p.mu.RLock()
	r, ok := p.frags[k]
	if ok {
		r.packets.Add(1)
	}
	p.mu.RUnlock()
	return ok
}

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

func l4Ports(proto byte, l4 []byte) (sport, dport uint16) {
	switch proto {
	case 6, protoUDP:
		if len(l4) >= 4 {
			return binary.BigEndian.Uint16(l4[0:2]), binary.BigEndian.Uint16(l4[2:4])
		}
	}
	return 0, 0
}
