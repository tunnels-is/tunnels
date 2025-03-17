package dns

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"
)

// Resolver represents a DNS resolver
type Resolver struct {
	mu              sync.RWMutex
	upstreamServers []string
	cache           map[string]*CacheEntry
	cacheTimeout    time.Duration
}

// CacheEntry represents a cached DNS response
type CacheEntry struct {
	Addresses []net.IP
	Expires   time.Time
}

// NewResolver creates a new DNS resolver
func NewResolver(upstreamServers []string) *Resolver {
	return &Resolver{
		upstreamServers: upstreamServers,
		cache:           make(map[string]*CacheEntry),
		cacheTimeout:    5 * time.Minute,
	}
}

// Resolve resolves a hostname to IP addresses
func (r *Resolver) Resolve(ctx context.Context, hostname string) ([]net.IP, error) {
	r.mu.RLock()
	cached, exists := r.cache[hostname]
	r.mu.RUnlock()

	if exists && time.Now().Before(cached.Expires) {
		return cached.Addresses, nil
	}

	// Create a new context with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try each upstream server
	var lastErr error
	for _, server := range r.upstreamServers {
		addresses, err := r.resolveWithServer(ctx, hostname, server)
		if err == nil {
			// Cache the successful result
			r.mu.Lock()
			r.cache[hostname] = &CacheEntry{
				Addresses: addresses,
				Expires:   time.Now().Add(r.cacheTimeout),
			}
			r.mu.Unlock()
			return addresses, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("all DNS servers failed: %w", lastErr)
}

// resolveWithServer resolves a hostname using a specific DNS server
func (r *Resolver) resolveWithServer(ctx context.Context, hostname, server string) ([]net.IP, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Second * 5,
			}
			return d.DialContext(ctx, "udp", server)
		},
	}

	ips, err := resolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		return nil, err
	}

	result := make([]net.IP, len(ips))
	for i, ip := range ips {
		result[i] = ip.IP
	}

	return result, nil
}

// SetUpstreamServers updates the list of upstream DNS servers
func (r *Resolver) SetUpstreamServers(servers []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.upstreamServers = servers
}

// ClearCache clears the DNS cache
func (r *Resolver) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = make(map[string]*CacheEntry)
}
