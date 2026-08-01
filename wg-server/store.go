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

type PeerStore struct {
	mu      sync.RWMutex
	records map[string]PeerRecord

	byPubKey map[string]string
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

func (ps *PeerStore) setLocked(deviceID string, rec PeerRecord) {
	if old, ok := ps.records[deviceID]; ok && old.PubKeyB64 != rec.PubKeyB64 {
		if ps.byPubKey[old.PubKeyB64] == deviceID {
			delete(ps.byPubKey, old.PubKeyB64)
		}
	}
	ps.records[deviceID] = rec
	ps.byPubKey[rec.PubKeyB64] = deviceID
}

func (ps *PeerStore) GetOrAssign(deviceID, pubKeyB64 string) (string, string, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if rec, ok := ps.records[deviceID]; ok {
		if rec.PubKeyB64 != pubKeyB64 {
			rec.PubKeyB64 = pubKeyB64
			ps.setLocked(deviceID, rec)
		}
		return rec.IP, rec.IPv6, nil
	}

	ip, err := ps.nextIP()
	if err != nil {
		return "", "", err
	}

	var ipv6 string
	if ps.subnet6 != "" {
		ipv6, err = ps.nextIPv6()
		if err != nil {
			return "", "", err
		}
	}

	ps.setLocked(deviceID, PeerRecord{PubKeyB64: pubKeyB64, IP: ip, IPv6: ipv6})
	return ip, ipv6, nil
}

func (ps *PeerStore) Set(deviceID, ip, ipv6, pubKeyB64 string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.setLocked(deviceID, PeerRecord{PubKeyB64: pubKeyB64, IP: ip, IPv6: ipv6})
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

func (ps *PeerStore) nextIP() (string, error) {
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

func (ps *PeerStore) nextIPv6() (string, error) {
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
