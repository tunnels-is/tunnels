package wgserver

import (
	"fmt"
	"net"
	"sort"
	"sync"
)

type PeerRecord struct {
	PubKeyB64 string
	IP        string
}

type PeerStore struct {
	mu      sync.RWMutex
	records map[string]PeerRecord
	subnet  string
}

func NewPeerStore(subnet string) *PeerStore {
	return &PeerStore{
		records: make(map[string]PeerRecord),
		subnet:  subnet,
	}
}

func (ps *PeerStore) GetOrAssign(deviceID, pubKeyB64 string) (string, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if rec, ok := ps.records[deviceID]; ok {
		if rec.PubKeyB64 != pubKeyB64 {
			rec.PubKeyB64 = pubKeyB64
			ps.records[deviceID] = rec
		}
		return rec.IP, nil
	}

	ip, err := ps.nextIP()
	if err != nil {
		return "", err
	}
	ps.records[deviceID] = PeerRecord{PubKeyB64: pubKeyB64, IP: ip}
	return ip, nil
}

func (ps *PeerStore) Get(deviceID string) (PeerRecord, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	rec, ok := ps.records[deviceID]
	return rec, ok
}

func (ps *PeerStore) GetByPubKey(pubKeyB64 string) (PeerRecord, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	for _, rec := range ps.records {
		if rec.PubKeyB64 == pubKeyB64 {
			return rec, true
		}
	}
	return PeerRecord{}, false
}

func (ps *PeerStore) Set(deviceID, ip, pubKeyB64 string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.records[deviceID] = PeerRecord{PubKeyB64: pubKeyB64, IP: ip}
}

func (ps *PeerStore) GetAll() map[string]PeerRecord {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make(map[string]PeerRecord, len(ps.records))
	for k, v := range ps.records {
		out[k] = v
	}
	return out
}

func (ps *PeerStore) nextIP() (string, error) {
	_, ipNet, err := net.ParseCIDR(ps.subnet)
	if err != nil {
		return "", fmt.Errorf("invalid subnet %q: %w", ps.subnet, err)
	}

	used := make([]uint32, 0, len(ps.records))
	for _, rec := range ps.records {
		if ip := net.ParseIP(rec.IP).To4(); ip != nil && ipNet.Contains(ip) {
			used = append(used, storeIPToUint32(ip))
		}
	}
	sort.Slice(used, func(i, j int) bool { return used[i] < used[j] })

	base := storeIPToUint32(ipNet.IP.To4()) + 2
	if len(used) > 0 && used[len(used)-1] >= base {
		base = used[len(used)-1] + 1
	}

	next := storeUint32ToIP(base)
	if !ipNet.Contains(next) {
		return "", fmt.Errorf("WireGuard subnet %s is exhausted", ps.subnet)
	}
	return next.String(), nil
}

func storeIPToUint32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func storeUint32ToIP(n uint32) net.IP {
	return net.IP{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}
