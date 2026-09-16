package wgserver

import (
	"net/netip"
	"strconv"
	"strings"
)

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

func (p *fwPeer) allowedContains(src netip.Addr, dport uint16) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return policyAdmits(p.allowAll, p.allowed, p.anyPorts, src, dport)
}

func (p *fwPeer) allowedAnyPort(src netip.Addr) bool {
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

func (p *fwPeer) setAllowed(entries []aclEntry, allowAll bool) {
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
