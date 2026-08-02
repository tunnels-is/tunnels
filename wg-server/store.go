package wgserver

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
)

type PeerRecord struct {
	PubKeyB64 string
	IP        string
	IPv6      string
}

// PeerStore is a concurrency-safe IP assignment table.
// All access to records/byPubKey goes through methods that hold mu.
type PeerStore struct {
	mu       sync.RWMutex
	records  map[string]PeerRecord // deviceID → record
	byPubKey map[string]string     // pubKeyB64 → deviceID
	subnet   string
	subnet6  string
}

func NewPeerStore(subnet, subnet6 string) *PeerStore {
	return &PeerStore{
		records:  make(map[string]PeerRecord),
		byPubKey: make(map[string]string),
		subnet:   subnet,
		subnet6:  subnet6,
	}
}

func (ps *PeerStore) GetOrAssign(deviceID, pubKeyB64 string) (string, string, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if rec, ok := ps.records[deviceID]; ok {
		if rec.PubKeyB64 != pubKeyB64 {
			if old := rec.PubKeyB64; old != "" && ps.byPubKey[old] == deviceID {
				delete(ps.byPubKey, old)
			}
			rec.PubKeyB64 = pubKeyB64
			ps.records[deviceID] = rec
			ps.byPubKey[pubKeyB64] = deviceID
		}
		return rec.IP, rec.IPv6, nil
	}

	ip, err := ps.nextIPLocked()
	if err != nil {
		return "", "", err
	}

	var ipv6 string
	if ps.subnet6 != "" {
		ipv6, err = ps.nextIPv6Locked()
		if err != nil {
			return "", "", err
		}
	}

	ps.records[deviceID] = PeerRecord{PubKeyB64: pubKeyB64, IP: ip, IPv6: ipv6}
	ps.byPubKey[pubKeyB64] = deviceID
	return ip, ipv6, nil
}

func (ps *PeerStore) Set(deviceID, ip, ipv6, pubKeyB64 string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if old, ok := ps.records[deviceID]; ok && old.PubKeyB64 != pubKeyB64 {
		if ps.byPubKey[old.PubKeyB64] == deviceID {
			delete(ps.byPubKey, old.PubKeyB64)
		}
	}
	ps.records[deviceID] = PeerRecord{PubKeyB64: pubKeyB64, IP: ip, IPv6: ipv6}
	ps.byPubKey[pubKeyB64] = deviceID
}

func (ps *PeerStore) DeleteByPubKey(pubKeyB64 string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	deviceID, ok := ps.byPubKey[pubKeyB64]
	if !ok {
		return
	}
	delete(ps.byPubKey, pubKeyB64)
	if rec, ok := ps.records[deviceID]; ok && rec.PubKeyB64 == pubKeyB64 {
		delete(ps.records, deviceID)
	}
}

// nextIPLocked requires ps.mu held for writing (or exclusive read of records).
func (ps *PeerStore) nextIPLocked() (string, error) {
	_, ipNet, err := net.ParseCIDR(ps.subnet)
	if err != nil {
		return "", fmt.Errorf("invalid subnet %q: %w", ps.subnet, err)
	}

	used := make(map[uint32]bool, len(ps.records))
	for _, rec := range ps.records {
		if ip := net.ParseIP(rec.IP).To4(); ip != nil && ipNet.Contains(ip) {
			used[storeIPToUint32(ip)] = true
		}
	}

	for candidate := storeIPToUint32(ipNet.IP.To4()) + 2; ; candidate++ {
		next := storeUint32ToIP(candidate)
		if !ipNet.Contains(next) {
			return "", fmt.Errorf("WireGuard subnet %s is exhausted", ps.subnet)
		}
		if !used[candidate] {
			return next.String(), nil
		}
	}
}

// nextIPv6Locked requires ps.mu held for writing (or exclusive read of records).
func (ps *PeerStore) nextIPv6Locked() (string, error) {
	prefix, err := netip.ParsePrefix(ps.subnet6)
	if err != nil {
		return "", fmt.Errorf("invalid IPv6 subnet %q: %w", ps.subnet6, err)
	}

	used := make(map[netip.Addr]bool, len(ps.records))
	for _, rec := range ps.records {
		if rec.IPv6 == "" {
			continue
		}
		if addr, parseErr := netip.ParseAddr(rec.IPv6); parseErr == nil && prefix.Contains(addr) {
			used[addr] = true
		}
	}

	candidate := prefix.Addr().Next().Next()
	for prefix.Contains(candidate) {
		if !used[candidate] {
			return candidate.String(), nil
		}
		candidate = candidate.Next()
	}
	return "", fmt.Errorf("WireGuard IPv6 subnet %s is exhausted", ps.subnet6)
}

func storeIPToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func storeUint32ToIP(n uint32) net.IP {
	return net.IP{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}
