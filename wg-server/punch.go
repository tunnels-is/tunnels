package wgserver

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/punch"
	"github.com/tunnels-is/tunnels/types"
)

const (
	punchReflexTTL = 60 * time.Second
	punchLocateTTL = 15 * time.Second
	punchLocalTTL  = 10 * time.Second
)

var (
	punchMu        sync.Mutex
	punchDiscoPriv [32]byte
	punchDiscoOK   bool

	punchSem = make(chan struct{}, 32)

	punchLocalMu sync.Mutex
	punchLocalAt time.Time
	punchLocal   map[uuid.UUID]punchDevice

	punchReflexMu sync.Mutex
	punchReflex   = map[uuid.UUID]punchSeen{}

	punchLocateMu sync.Mutex
	punchLocate   = map[uuid.UUID]punchLocateEntry{}
)

type punchDevice struct {
	ip    netip.Addr
	disco [32]byte
}

type punchSeen struct {
	at   netip.AddrPort
	when time.Time
}

type punchLocateEntry struct {
	at  time.Time
	loc types.P2PLocate
	ok  bool
}

func setPunchDisco(priv, pub [32]byte) {
	punchMu.Lock()
	punchDiscoPriv = priv
	punchDiscoOK = true
	punchMu.Unlock()
	_ = pub
}

func enqueuePunch(from string, pkt []byte) {
	select {
	case punchSem <- struct{}{}:
		go func() {
			defer func() { <-punchSem }()
			dispatchPunch(from, pkt)
		}()
	default:
	}
}

func dispatchPunch(fromStr string, pkt []byte) {
	from, err := netip.ParseAddrPort(fromStr)
	if err != nil || !from.IsValid() {
		return
	}
	switch punch.TypeOf(pkt) {
	case punch.TypeReflexReq:
		handleReflex(from, pkt)
	case punch.TypeRendezvous:
		handleRendezvous(from, pkt)
	case punch.TypeForward:
		handleForward(from, pkt)
	}
}

func handleReflex(from netip.AddrPort, pkt []byte) {
	punchMu.Lock()
	priv := punchDiscoPriv
	ok := punchDiscoOK
	punchMu.Unlock()
	if !ok {
		return
	}
	id, pub, err := punch.OpenReflexReq(priv, pkt, time.Now())
	if err != nil {
		return
	}
	dev, found := localPunchDevice(id)
	if !found || dev.disco != pub {
		return
	}
	punchReflexMu.Lock()
	punchReflex[id] = punchSeen{at: from, when: time.Now()}
	punchReflexMu.Unlock()
	sendPunch(from, punch.BuildReflexResp(id, from))
}

func handleRendezvous(from netip.AddrPort, pkt []byte) {
	src, dst, ok := rendezvousIDs(pkt)
	if !ok {
		return
	}
	if _, local := localPunchDevice(src); !local {
		return
	}
	reflex, fresh := reflexOf(src)
	if !fresh || reflex != from {
		return
	}
	srcIP, ok := deviceTunnelIP(src)
	if !ok {
		return
	}
	loc, ok := locateDevice(dst)
	if !ok || loc.WireGuardIP == "" || loc.HomeIP == "" || loc.HomePort == 0 {
		return
	}
	dstIP, err := netip.ParseAddr(loc.WireGuardIP)
	if err != nil {
		return
	}
	if !localAdmitsAllPorts(srcIP, dstIP) {
		INFO("punch: ", srcIP, " is not allowed to punch ", dstIP)
		return
	}
	if loc.Local {
		if !localAdmitsAllPorts(dstIP, srcIP) {
			INFO("punch: ", dstIP, " is not allowed to punch ", srcIP)
			return
		}
		to, fresh := reflexOf(dst)
		if !fresh {
			return
		}
		sendPunch(to, pkt)
		return
	}
	home, err := netip.ParseAddr(loc.HomeIP)
	if err != nil || loc.HomePort < 1 || loc.HomePort > 65535 {
		return
	}
	sendPunch(netip.AddrPortFrom(home, uint16(loc.HomePort)), punch.BuildForward(src, dst, srcIP, pkt))
}

func handleForward(from netip.AddrPort, pkt []byte) {
	if !isMeshPeerIP(from.Addr()) {
		return
	}
	src, dst, srcIP, rendezvous, err := punch.ParseForward(pkt)
	if err != nil || !srcIP.IsValid() {
		return
	}
	if _, local := localPunchDevice(dst); !local {
		return
	}
	dstIP, ok := deviceTunnelIP(dst)
	if !ok || !localAdmitsAllPorts(dstIP, srcIP) {
		INFO("punch: forward to ", dst, " denied by firewall")
		return
	}
	_ = src
	to, fresh := reflexOf(dst)
	if !fresh {
		return
	}
	sendPunch(to, rendezvous)
}

func rendezvousIDs(pkt []byte) (src, dst uuid.UUID, ok bool) {
	if punch.TypeOf(pkt) != punch.TypeRendezvous || len(pkt) < 6+32 {
		return uuid.UUID{}, uuid.UUID{}, false
	}
	copy(src[:], pkt[6:22])
	copy(dst[:], pkt[22:38])
	return src, dst, src != uuid.Nil && dst != uuid.Nil
}

func reflexOf(id uuid.UUID) (netip.AddrPort, bool) {
	punchReflexMu.Lock()
	defer punchReflexMu.Unlock()
	e, ok := punchReflex[id]
	if !ok || time.Since(e.when) > punchReflexTTL {
		return netip.AddrPort{}, false
	}
	return e.at, true
}

func deviceTunnelIP(id uuid.UUID) (netip.Addr, bool) {
	if peerStore == nil {
		return netip.Addr{}, false
	}
	rec, ok := peerStore.Get(id.String())
	if !ok || rec.IP == "" {
		return netip.Addr{}, false
	}
	ip, err := netip.ParseAddr(rec.IP)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip, true
}

func localAdmitsAllPorts(deviceIP, peerIP netip.Addr) bool {
	if d := inspectDevice.Load(); d != nil && !d.firewall.Load() {
		return true
	}
	p, local := fwClassify(deviceIP)
	if !local || p == nil {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.allowAll {
		return true
	}
	ps := p.allowed[peerIP]
	return ps != nil && ps.all
}

func localPunchDevice(id uuid.UUID) (punchDevice, bool) {
	refreshLocalPunchDevices()
	punchLocalMu.Lock()
	defer punchLocalMu.Unlock()
	d, ok := punchLocal[id]
	return d, ok
}

func refreshLocalPunchDevices() {
	punchLocalMu.Lock()
	fresh := time.Since(punchLocalAt) < punchLocalTTL && punchLocal != nil
	punchLocalMu.Unlock()
	if fresh {
		return
	}
	cfg := activeConfig.Load()
	if cfg == nil || httpClient == nil {
		return
	}
	req, err := http.NewRequest(http.MethodGet, trimURL(cfg.ControllerURL)+"/wg/p2p/local", nil)
	if err != nil {
		return
	}
	req.Header.Set("X-WG-KEY", cfg.APIKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var body struct {
		Peers []types.P2PPeer `json:"Peers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return
	}
	next := make(map[uuid.UUID]punchDevice, len(body.Peers))
	for _, p := range body.Peers {
		id, err := uuid.Parse(p.DeviceID)
		if err != nil {
			continue
		}
		ip, err := netip.ParseAddr(p.WireGuardIP)
		if err != nil {
			continue
		}
		disco, err := punch.DecodeKey(p.DiscoKey)
		if err != nil {
			continue
		}
		next[id] = punchDevice{ip: ip, disco: disco}
	}
	punchLocalMu.Lock()
	punchLocal = next
	punchLocalAt = time.Now()
	punchLocalMu.Unlock()
}

func locateDevice(id uuid.UUID) (types.P2PLocate, bool) {
	punchLocateMu.Lock()
	if e, ok := punchLocate[id]; ok && time.Since(e.at) < punchLocateTTL {
		punchLocateMu.Unlock()
		return e.loc, e.ok
	}
	punchLocateMu.Unlock()

	cfg := activeConfig.Load()
	if cfg == nil || httpClient == nil {
		return types.P2PLocate{}, false
	}
	req, err := http.NewRequest(http.MethodGet, trimURL(cfg.ControllerURL)+"/wg/p2p/locate?deviceID="+id.String(), nil)
	if err != nil {
		return types.P2PLocate{}, false
	}
	req.Header.Set("X-WG-KEY", cfg.APIKey)
	resp, err := httpClient.Do(req)
	if err != nil {
		return types.P2PLocate{}, false
	}
	defer resp.Body.Close()
	var loc types.P2PLocate
	ok := resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&loc) == nil && loc.WireGuardIP != ""
	punchLocateMu.Lock()
	punchLocate[id] = punchLocateEntry{at: time.Now(), loc: loc, ok: ok}
	punchLocateMu.Unlock()
	return loc, ok
}

func isMeshPeerIP(ip netip.Addr) bool {
	meshMu.Lock()
	defer meshMu.Unlock()
	for _, p := range meshPeers {
		host, _, err := net.SplitHostPort(p.Endpoint)
		if err != nil {
			continue
		}
		if host == ip.String() || host == ip.Unmap().String() {
			return true
		}
	}
	return false
}

func sendPunch(to netip.AddrPort, pkt []byte) {
	if wgLazyBind == nil || !to.IsValid() || len(pkt) == 0 {
		return
	}
	ep, err := wgLazyBind.ParseEndpoint(to.String())
	if err != nil {
		return
	}
	if err := wgLazyBind.Send([][]byte{pkt}, ep); err != nil {
		WARN("punch send to ", to.String(), ": ", err)
	}
}

func trimURL(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func publishPunchPolicy(src netip.Addr, entries []aclEntry, allowAll bool) {
	if peerStore == nil || httpClient == nil || activeConfig.Load() == nil || !src.Is4() {
		return
	}
	id, ok := peerStore.DeviceIDByIP(src.String())
	if !ok {
		return
	}
	ips := make([]string, 0)
	if !allowAll {
		for _, e := range entries {
			if e.anyHost || e.port != 0 || !e.addr.Is4() {
				continue
			}
			ips = append(ips, e.addr.String())
		}
	}
	body, err := json.Marshal(map[string]any{
		"DeviceID": id,
		"AllowAll": allowAll,
		"AllowIPs": ips,
	})
	if err != nil {
		return
	}
	go postPunchPolicy(body)
}

func postPunchPolicy(body []byte) {
	cfg := activeConfig.Load()
	if cfg == nil || httpClient == nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, trimURL(cfg.ControllerURL)+"/wg/p2p/policy", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("X-WG-KEY", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		WARN("punch policy post: ", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		WARN("punch policy post status ", resp.StatusCode)
	}
}
