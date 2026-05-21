package wgserver

import (
	"net/netip"
	"sync"
)

// ACLStore holds per-destination allowlists for peer-to-peer traffic.
//
// A "destination" is the WireGuard IP of a peer that has announced a policy
// by sending a control packet. Peers without a stored policy fall through to
// the default-open behavior.
type ACLStore struct {
	mu      sync.RWMutex
	allowed map[netip.Addr]map[netip.Addr]struct{}
}

func NewACLStore() *ACLStore {
	return &ACLStore{allowed: make(map[netip.Addr]map[netip.Addr]struct{})}
}

// Set replaces dst's allowlist with srcs (replace-set semantics).
// An empty srcs list means "deny all peer-to-peer ingress to dst."
func (s *ACLStore) Set(dst netip.Addr, srcs []netip.Addr) {
	set := make(map[netip.Addr]struct{}, len(srcs))
	for _, a := range srcs {
		set[a] = struct{}{}
	}
	s.mu.Lock()
	s.allowed[dst] = set
	s.mu.Unlock()
}

// Clear removes any policy for dst (reverts to default-open).
func (s *ACLStore) Clear(dst netip.Addr) {
	s.mu.Lock()
	delete(s.allowed, dst)
	s.mu.Unlock()
}

// Allowed reports whether src may send a peer-to-peer packet to dst.
// If dst has no stored policy, the call returns true (default-open).
func (s *ACLStore) Allowed(src, dst netip.Addr) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	set, ok := s.allowed[dst]
	if !ok {
		return true
	}
	_, ok = set[src]
	return ok
}

// Snapshot returns a copy of the current ACL table. Intended for diagnostics.
func (s *ACLStore) Snapshot() map[netip.Addr][]netip.Addr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[netip.Addr][]netip.Addr, len(s.allowed))
	for dst, set := range s.allowed {
		srcs := make([]netip.Addr, 0, len(set))
		for src := range set {
			srcs = append(srcs, src)
		}
		out[dst] = srcs
	}
	return out
}
