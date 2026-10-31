package wgserver

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

const (
	peerQueryTimeout = 2 * time.Second
	peerQueryRate    = 20
	peerQueryBurst   = 20
	peerAuthRefresh  = 2 * time.Minute
	maxPeerStates    = 100_000
	peerQueryBodyMax = 1 << 20
	peerDenyBodyMax  = 4096
)

var httpClient *http.Client

var peerQueryBudget = newTokenBucket(peerQueryRate, peerQueryBurst)

func initSyncClient(cfg *Config) {
	insecure := false
	if cfg != nil {
		insecure = cfg.InsecureSkipVerify
	}
	httpClient = newControllerHTTPClient(insecure)
	peerQueryBudget.reset(time.Now())
}

type authResult int

const (
	authAllowed authResult = iota
	authDenied
	authUnknown
)

type peerState struct {
	mu       sync.Mutex
	gen      uint64
	inflight int
}

type flightCall struct {
	wg   sync.WaitGroup
	res  authResult
	peer *types.WGPeer
}

type flightGroup struct {
	mu sync.Mutex
	m  map[string]*flightCall
}

var (
	peerStatesMu sync.Mutex
	peerStates   = map[string]*peerState{}

	peerFlight flightGroup
	// afterPeerFlightJoin is nil outside tests. A joined lookup calls it before waiting.
	afterPeerFlightJoin func()

	peerRefreshMu sync.Mutex
	peerRefreshAt = map[string]time.Time{}
)

func reconcilePeer(pubKeyB64 string) authResult {
	cfg := activeConfig.Load()
	if cfg == nil {
		return authUnknown
	}
	hexKey, err := b64ToHex(pubKeyB64)
	if err != nil {
		WARN("reconcilePeer: invalid pubkey: ", err)
		return authUnknown
	}

	st, myGen, ok := beginPeerState(pubKeyB64)
	if !ok {
		return authUnknown
	}
	defer releasePeerState(pubKeyB64, st)

	res, peer := peerFlight.Do(pubKeyB64, func() (authResult, *types.WGPeer) {
		return queryPeer(cfg, pubKeyB64)
	})
	return withCurrentPeerGen(st, myGen, func() authResult {
		switch res {
		case authDenied:
			forgetPeer(hexKey, pubKeyB64)
			return authDenied
		case authAllowed:
			return installAuthorizedPeer(cfg, hexKey, pubKeyB64, peer)
		default:
			return authUnknown
		}
	})
}

func beginPeerState(key string) (*peerState, uint64, bool) {
	peerStatesMu.Lock()
	st := peerStates[key]
	if st == nil {
		if len(peerStates) >= maxPeerStates {
			peerStatesMu.Unlock()
			return nil, 0, false
		}
		st = &peerState{}
		peerStates[key] = st
	}
	peerStatesMu.Unlock()

	st.mu.Lock()
	st.gen++
	st.inflight++
	gen := st.gen
	st.mu.Unlock()
	return st, gen, true
}

func releasePeerState(key string, st *peerState) {
	peerStatesMu.Lock()
	defer peerStatesMu.Unlock()
	st.mu.Lock()
	defer st.mu.Unlock()
	st.inflight--
	if st.inflight == 0 && peerStates[key] == st {
		delete(peerStates, key)
	}
}

func withCurrentPeerGen(st *peerState, gen uint64, fn func() authResult) authResult {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.gen != gen {
		return authUnknown
	}
	return fn()
}

func (g *flightGroup) Do(key string, fn func() (authResult, *types.WGPeer)) (authResult, *types.WGPeer) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*flightCall)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		if afterPeerFlightJoin != nil {
			afterPeerFlightJoin()
		}
		c.wg.Wait()
		return c.res, c.peer
	}
	c := &flightCall{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.res, c.peer = fn()
	c.wg.Done()

	g.mu.Lock()
	if g.m[key] == c {
		delete(g.m, key)
	}
	g.mu.Unlock()
	return c.res, c.peer
}

func installAuthorizedPeer(cfg *Config, hexKey, pubKeyB64 string, p *types.WGPeer) authResult {
	if p == nil {
		return authUnknown
	}
	var ip, ipv6 string
	if p.WireGuardIP != "" {
		ip = p.WireGuardIP
		ipv6 = p.WireGuardIPv6
		if !subnetContains(cfg.WireGuardSubnet, ip) {
			WARN("reconcilePeer: controller-assigned IP outside subnet: ", ip)
			return authUnknown
		}
		if err := types.ValidateDeviceWireGuardAddrs(cfg.WireGuardSubnet, cfg.WireGuardSubnet6, ip, ipv6); err != nil {
			WARN("reconcilePeer: reserved or invalid assigned IP: ", err)
			return authUnknown
		}
		if ipv6 != "" && cfg.WireGuardSubnet6 != "" && !subnetContains(cfg.WireGuardSubnet6, ipv6) {
			WARN("reconcilePeer: controller-assigned IPv6 outside subnet: ", ipv6)
			return authUnknown
		}
		if peerStore != nil {
			peerStore.Set(p.DeviceID, ip, ipv6, pubKeyB64)
		}
	} else {
		if peerStore == nil {
			WARN("reconcilePeer: GetOrAssign failed: ", "peer store is not ready")
			return authUnknown
		}
		var assignErr error
		ip, ipv6, assignErr = peerStore.GetOrAssign(p.DeviceID, pubKeyB64)
		if assignErr != nil {
			WARN("reconcilePeer: GetOrAssign failed: ", assignErr)
			return authUnknown
		}
	}

	if sameInstalledPeer(hexKey, ip, ipv6) {
		markPeerRefreshed(pubKeyB64, time.Now())
		return authAllowed
	}
	applyPeerFirewallSlot(hexKey, ip, ipv6)
	if err := addPeer(hexKey, peerAllowedIPs(ip, ipv6)...); err != nil {
		WARN("reconcilePeer: AddPeer failed: ", err)
		return authUnknown
	}
	markPeerRefreshed(pubKeyB64, time.Now())
	INFO("reconcilePeer: authorized → ", pubKeyB64[:12], "… ip=", ip)
	return authAllowed
}

func sameInstalledPeer(hexKey, ip, ipv6 string) bool {
	prev, ok := installedPeerAddrs.Load(hexKey)
	if !ok {
		return false
	}
	old, ok := prev.(peerAddrs)
	return ok && old.ip == ip && old.ipv6 == ipv6
}

func forgetPeer(hexKey, pubKeyB64 string) {
	_ = removePeer(hexKey)
	installedPeerAddrs.Delete(hexKey)
	if peerStore != nil {
		peerStore.DeleteByPubKey(pubKeyB64)
	}
	clearPeerRefresh(pubKeyB64)
}

func claimPeerRefresh(key string, now time.Time) bool {
	peerRefreshMu.Lock()
	defer peerRefreshMu.Unlock()
	if now.Sub(peerRefreshAt[key]) < peerAuthRefresh {
		return false
	}
	peerRefreshAt[key] = now
	return true
}

func markPeerRefreshed(key string, now time.Time) {
	peerRefreshMu.Lock()
	peerRefreshAt[key] = now
	peerRefreshMu.Unlock()
}

func clearPeerRefresh(key string) {
	peerRefreshMu.Lock()
	delete(peerRefreshAt, key)
	peerRefreshMu.Unlock()
}

type tokenBucket struct {
	mu     sync.Mutex
	tokens int
	last   time.Time
	rate   int
	burst  int
}

func newTokenBucket(rate, burst int) *tokenBucket {
	return &tokenBucket{rate: rate, burst: burst, tokens: burst, last: time.Now()}
}

func (b *tokenBucket) reset(now time.Time) {
	b.mu.Lock()
	b.tokens = b.burst
	b.last = now
	b.mu.Unlock()
}

func (b *tokenBucket) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.last.IsZero() {
		b.tokens = b.burst
		b.last = now
	} else if now.After(b.last) && b.rate > 0 {
		add := int(now.Sub(b.last)/time.Second) * b.rate
		if add > 0 {
			b.tokens += add
			if b.tokens > b.burst {
				b.tokens = b.burst
			}
			b.last = b.last.Add(time.Duration(add/b.rate) * time.Second)
		}
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

type peerAddrs struct {
	ip, ipv6 string
}

func applyPeerFirewallSlot(hexKey, ip, ipv6 string) {
	next := peerAddrs{ip: ip, ipv6: ipv6}
	prev, loaded := installedPeerAddrs.Load(hexKey)
	if !loaded {
		resetPeer(ip, ipv6)
		installedPeerAddrs.Store(hexKey, next)
		return
	}
	old, _ := prev.(peerAddrs)
	if old.ip == ip && old.ipv6 == ipv6 {
		return
	}
	dropPeer(old.ip, old.ipv6)
	resetPeer(ip, ipv6)
	installedPeerAddrs.Store(hexKey, next)
}

func peerAllowedIPs(ip, ipv6 string) []string {
	ips := []string{ip + "/32"}
	if ipv6 != "" {
		ips = append(ips, ipv6+"/128")
	}
	return ips
}

func subnetContains(cidr, ipStr string) bool {
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return subnet.Contains(ip)
}

func queryPeer(cfg *Config, pubKeyB64 string) (authResult, *types.WGPeer) {
	if err := requireHTTPSControllerURL(cfg.ControllerURL); err != nil {
		WARN("queryPeer: ", err)
		return authUnknown, nil
	}
	if !peerQueryBudget.allow(time.Now()) {
		slowWarn.do("peer-budget", "queryPeer: controller query budget exhausted")
		return authUnknown, nil
	}
	endpoint := strings.TrimRight(cfg.ControllerURL, "/") + "/wg/peer?pubkey=" + url.QueryEscape(pubKeyB64)
	ctx, cancel := context.WithTimeout(context.Background(), peerQueryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		WARN("queryPeer: build request: ", err)
		return authUnknown, nil
	}
	req.Header.Set("X-WG-KEY", cfg.APIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		slowWarn.do("peer-request", "queryPeer: request failed: ", err)
		return authUnknown, nil
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var peer types.WGPeer
		if err := json.NewDecoder(io.LimitReader(resp.Body, peerQueryBodyMax)).Decode(&peer); err != nil {
			WARN("queryPeer: decode response: ", err)
			return authUnknown, nil
		}
		return authAllowed, &peer
	default:
		var body struct {
			Decision string `json:"Decision"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, peerDenyBodyMax)).Decode(&body)
		if body.Decision == types.WGPeerDecisionDeny {
			return authDenied, nil
		}
		slowWarn.do("peer-status", "queryPeer: controller returned ", resp.StatusCode)
		return authUnknown, nil
	}
}
