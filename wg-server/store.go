package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"sync"
)

// PeerRecord holds the WireGuard public key and assigned IP for a single device.
type PeerRecord struct {
	PubKeyB64 string `json:"PubKeyB64"`
	IP        string `json:"IP"`
}

// PeerStore is the wg-server's authoritative source for device→IP mappings.
// It persists to a JSON file so assignments survive restarts.
type PeerStore struct {
	mu      sync.RWMutex
	records map[string]PeerRecord // key = deviceID (hex ObjectID string)
	path    string
	subnet  string
}

// NewPeerStore loads an existing store from path (or starts empty) and
// validates it against the given subnet.
func NewPeerStore(path, subnet string) (*PeerStore, error) {
	ps := &PeerStore{
		records: make(map[string]PeerRecord),
		path:    path,
		subnet:  subnet,
	}
	if err := ps.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load peer store: %w", err)
	}
	return ps, nil
}

func (ps *PeerStore) load() error {
	data, err := os.ReadFile(ps.path)
	if err != nil {
		return err
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return json.Unmarshal(data, &ps.records)
}

// save writes records to disk. Must be called with ps.mu held (any lock level).
func (ps *PeerStore) save() error {
	data, err := json.MarshalIndent(ps.records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ps.path, data, 0600)
}

// GetOrAssign returns the IP already assigned to deviceID, or allocates the
// next available IP from the subnet and persists it. If the device reconnects
// with a new public key the key is updated but the IP stays stable.
func (ps *PeerStore) GetOrAssign(deviceID, pubKeyB64 string) (string, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if rec, ok := ps.records[deviceID]; ok {
		// Update pubkey if it changed (e.g. key rotation).
		if rec.PubKeyB64 != pubKeyB64 {
			rec.PubKeyB64 = pubKeyB64
			ps.records[deviceID] = rec
			_ = ps.save()
		}
		return rec.IP, nil
	}

	ip, err := ps.nextIP()
	if err != nil {
		return "", err
	}
	ps.records[deviceID] = PeerRecord{PubKeyB64: pubKeyB64, IP: ip}
	if err := ps.save(); err != nil {
		return "", fmt.Errorf("persist store: %w", err)
	}
	return ip, nil
}

// Get returns the stored record for deviceID without allocating.
func (ps *PeerStore) Get(deviceID string) (PeerRecord, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	rec, ok := ps.records[deviceID]
	return rec, ok
}

// GetAll returns a snapshot of all records.
func (ps *PeerStore) GetAll() map[string]PeerRecord {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make(map[string]PeerRecord, len(ps.records))
	for k, v := range ps.records {
		out[k] = v
	}
	return out
}

// nextIP finds the next unallocated IP in the subnet.
// Must be called with ps.mu held (write lock).
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

	// .1 is reserved for the wg-server interface; start allocation at .2
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
