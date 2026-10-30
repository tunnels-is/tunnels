package client

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	neturl "net/url"
	"runtime/debug"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/punch"
	wgconn "golang.zx2c4.com/wireguard/conn"
	wgdevice "golang.zx2c4.com/wireguard/device"
)

type punchDatagram struct {
	from string
	data []byte
}

type DirectPeer struct {
	DeviceID string
	TunnelIP string
	Endpoint string
}

// DirectPeers ...
func (t *TUN) DirectPeers() []DirectPeer {
	if t == nil {
		return nil
	}
	t.punchMu.Lock()
	defer t.punchMu.Unlock()
	if len(t.punchDirect) == 0 {
		return nil
	}
	out := make([]DirectPeer, 0, len(t.punchDirect))
	for ip, inst := range t.punchDirect {
		out = append(out, DirectPeer{
			DeviceID: inst.id.String(),
			TunnelIP: netip.AddrFrom4(ip).String(),
			Endpoint: inst.via.String(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TunnelIP == out[j].TunnelIP {
			return out[i].DeviceID < out[j].DeviceID
		}
		return out[i].TunnelIP < out[j].TunnelIP
	})
	return out
}

type punchInstall struct {
	id    uuid.UUID
	wgPub [32]byte
	via   netip.AddrPort
}

type punchBind struct {
	inner   wgconn.Bind
	tun     atomic.Pointer[TUN]
	closed  atomic.Bool
	fwOff   atomic.Bool
	retryMu sync.Mutex
	retry   map[uuid.UUID]punchInstall

	loopMu  sync.Mutex
	looping bool
}

func newPunchBind(inner wgconn.Bind, tun *TUN) wgconn.Bind {
	if tun.punchIn == nil {
		tun.punchIn = make(chan punchDatagram, 64)
	}
	if tun.punchWant == nil {
		tun.punchWant = make(chan [4]byte, 16)
	}
	if tun.punchInject == nil {
		tun.punchInject = make(chan []byte, 16)
	}
	ensurePunchWake(tun)
	b := &punchBind{inner: inner}
	b.tun.Store(tun)
	return b
}

// Open ...
func (b *punchBind) Open(port uint16) ([]wgconn.ReceiveFunc, uint16, error) {
	fns, actual, err := b.inner.Open(port)
	if err != nil {
		return nil, 0, err
	}
	b.closed.Store(false)
	b.ensureLoop()
	out := make([]wgconn.ReceiveFunc, len(fns))
	for i, fn := range fns {
		out[i] = b.wrap(fn)
	}
	return out, actual, nil
}

func (b *punchBind) ensureLoop() {
	b.loopMu.Lock()
	defer b.loopMu.Unlock()
	if b.looping {
		return
	}
	b.looping = true
	go b.loop()
}

func (b *punchBind) wrap(inner wgconn.ReceiveFunc) wgconn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []wgconn.Endpoint) (int, error) {
		n, err := inner(bufs, sizes, eps)
		if err != nil || n == 0 {
			return n, err
		}
		tun := b.tun.Load()
		out := 0
		for i := 0; i < n; i++ {
			data := bufs[i][:sizes[i]]
			if punch.Is(data) && tun != nil && tun.punchIn != nil {
				pkt := make([]byte, len(data))
				copy(pkt, data)
				from := ""
				if eps[i] != nil {
					from = eps[i].DstToString()
				}
				select {
				case tun.punchIn <- punchDatagram{from: from, data: pkt}:
				default:
				}
				continue
			}
			if out != i {
				copy(bufs[out][:sizes[i]], data)
				sizes[out] = sizes[i]
				eps[out] = eps[i]
			}
			out++
		}
		return out, nil
	}
}

// Close ...
func (b *punchBind) Close() error {
	b.closed.Store(true)
	return b.inner.Close()
}

// SetMark ...
func (b *punchBind) SetMark(mark uint32) error { return b.inner.SetMark(mark) }

// Send ...
func (b *punchBind) Send(bufs [][]byte, ep wgconn.Endpoint) error {
	return b.inner.Send(bufs, ep)
}

// ParseEndpoint ...
func (b *punchBind) ParseEndpoint(s string) (wgconn.Endpoint, error) {
	return b.inner.ParseEndpoint(s)
}

// BatchSize ...
func (b *punchBind) BatchSize() int { return b.inner.BatchSize() }

func (b *punchBind) loop() {
	panicked := false
	defer func() {
		b.loopMu.Lock()
		b.looping = false
		again := !panicked && !b.closed.Load()
		b.loopMu.Unlock()
		if again {
			b.ensureLoop()
		}
	}()
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			ERROR("punch loop: ", r, "\n", string(debug.Stack()))
		}
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var eng *punch.Engine
	var refreshed time.Time
	DEBUG("punch: loop started")
	for !b.closed.Load() {
		tun := b.tun.Load()
		if tun == nil || tun.punchIn == nil || tun.punchWant == nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if eng == nil {
			eng = b.buildEngine(tun)
			if eng != nil {
				DEBUG("punch: engine ready for ", tun.deviceID)
			}
		}
		select {
		case pkt := <-tun.punchIn:
			if eng == nil {
				continue
			}
			from, err := netip.ParseAddrPort(pkt.from)
			if err != nil {
				continue
			}
			b.apply(tun, eng, eng.OnPacket(from, pkt.data, time.Now()))
		case <-tun.punchWant:
			if eng != nil {
				tun = b.tun.Load()
				if tun != nil {
					tun.drainPunchWants(eng)
				}
			}
		case <-tick.C:
			tun = b.tun.Load()
			if tun == nil || eng == nil {
				continue
			}
			b.apply(tun, eng, b.pushLocalPolicy(tun, eng))
			if refreshed.IsZero() || time.Since(refreshed) > 15*time.Second {
				b.refreshWG(tun, eng)
				b.apply(tun, eng, b.pushLocalPolicy(tun, eng))
				refreshed = time.Now()
			}
			tun.drainPunchWants(eng)
			if tun.wgDevice != nil {
				b.apply(tun, eng, eng.Poll(time.Now()))
				b.retryRemoves(tun)
			}
		}
	}
}

func (b *punchBind) buildEngine(tun *TUN) *punch.Engine {
	if tun.deviceID == "" {
		return nil
	}
	dev, err := loadLocalDevice(tun.deviceID)
	if err != nil || dev == nil || dev.WireGuardPrivKey == "" || dev.WireGuardIP == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(dev.WireGuardPrivKey)
	if err != nil || len(raw) != 32 {
		return nil
	}
	var wgPriv, wgPub, disco [32]byte
	copy(wgPriv[:], raw)
	disco = punch.DiscoPrivateFor(wgPriv)
	if _, err = punch.Public(disco); err != nil {
		return nil
	}
	wgRaw, err := base64.StdEncoding.DecodeString(dev.WireGuardPubKey)
	if err != nil || len(wgRaw) != 32 {
		derived, derr := deriveWGPubKey(dev.WireGuardPrivKey)
		if derr != nil {
			return nil
		}
		wgRaw, _ = base64.StdEncoding.DecodeString(derived)
	}
	if len(wgRaw) != 32 {
		return nil
	}
	copy(wgPub[:], wgRaw)
	ip, err := netip.ParseAddr(dev.WireGuardIP)
	if err != nil {
		return nil
	}
	id, err := uuid.Parse(tun.deviceID)
	if err != nil {
		return nil
	}
	eng := punch.NewEngine(id, disco, wgPub, ip)
	meta := tun.meta.Load()
	eng.SetDiscoverMTU(meta == nil || meta.MTU == 0)
	return eng
}

func (b *punchBind) pushLocalPolicy(tun *TUN, eng *punch.Engine) punch.Result {
	m := tun.meta.Load()
	var entries []string
	allowAll := false
	if m != nil {
		entries = m.AllowedHosts
		allowAll = m.AllowAll
	}
	return eng.SetLocalPolicy(b.fwOff.Load(), allowAll, entries)
}

func (b *punchBind) refreshWG(tun *TUN, eng *punch.Engine) {
	cr := tun.CR
	if cr == nil || cr.Server == nil || tun.deviceID == "" {
		return
	}
	cfg := b.fetchWG(tun)
	if cfg != nil {
		b.fwOff.Store(!cfg.EnableFirewall)
		if cfg.DiscoKey != "" {
			if key, err := punch.DecodeKey(cfg.DiscoKey); err == nil {
				eng.SetServerDisco(key)
			}
		}
	}
}

func (b *punchBind) fetchWG(tun *TUN) *wgServerConfig {
	cr := tun.CR
	dev, err := loadLocalDevice(tun.deviceID)
	if err != nil || dev == nil || dev.WireGuardPubKey == "" {
		return nil
	}
	url := cr.Server.GetURL("/client/wg/config") + "?serverID=" + neturl.QueryEscape(tun.metaServer()) + "&pubKey=" + neturl.QueryEscape(dev.WireGuardPubKey)
	body, code, err := SendRequestToURL(nil, "GET", url, nil, 8000, cr.Server.ValidateCertificate, cr.Server.CertificatePath, authHeaders(cr))
	if err != nil || code != 200 {
		DEBUG("punch: wg config ", code, " ", err)
		return nil
	}
	cfg := new(wgServerConfig)
	if json.Unmarshal(body, cfg) != nil {
		return nil
	}
	return cfg
}

func authHeaders(cr *ConnectionRequest) map[string]string {
	return map[string]string{
		"X-Device-Token": cr.DeviceToken,
		"X-UID":          cr.UserID,
	}
}

func (t *TUN) metaServer() string {
	if t.CR != nil && t.CR.ServerID != "" {
		return t.CR.ServerID
	}
	if m := t.meta.Load(); m != nil {
		return m.ServerID
	}
	return ""
}

func (b *punchBind) apply(tun *TUN, eng *punch.Engine, res punch.Result) {
	if tun == nil {
		return
	}
	cr := tun.CR
	serverAddr := ""
	if cr != nil && tun.ServerResponse != nil && tun.ServerResponse.WireGuardPort != "" {
		serverAddr = cr.ServerIP + ":" + tun.ServerResponse.WireGuardPort
	}
	for _, d := range res.Sends {
		if d.Tunnel {
			b.injectIntro(tun, d)
			continue
		}
		to := d.To.String()
		if d.Server {
			to = serverAddr
		}
		if to == "" {
			DEBUG("punch: drop send, no address")
			continue
		}
		ep, err := b.ParseEndpoint(to)
		if err != nil {
			DEBUG("punch: bad endpoint ", to, ": ", err)
			continue
		}
		err = b.sendOne(d.Data, ep, d.MTU)
		if err != nil && d.MTU && eng != nil && isMsgTooBig(err) {
			b.apply(tun, eng, eng.MTUUnsent(d.Peer, time.Now()))
			continue
		}
		if err != nil {
			DEBUG("punch: send to ", to, ": ", err)
		}
	}
	if tun.wgDevice == nil {
		return
	}
	for _, up := range res.Up {
		if !up.Peer.WGIP.Is4() || !up.Via.IsValid() {
			continue
		}
		ip := up.Peer.WGIP.As4()
		inst := punchInstall{id: up.Peer.DeviceID, wgPub: up.Peer.WGPub, via: up.Via}
		if err := applyDirectIPC(tun.wgDevice, ip, inst); err != nil {
			ERROR("punch install ", up.Peer.WGIP.String(), ": ", err)
			continue
		}
		tun.punchMu.Lock()
		if tun.punchDirect == nil {
			tun.punchDirect = map[[4]byte]punchInstall{}
		}
		tun.punchDirect[ip] = inst
		tun.punchMu.Unlock()
		b.clearRetry(up.Peer.DeviceID)
		INFO("punch: direct path to ", up.Peer.WGIP.String(), " via ", up.Via.String())
	}
	for _, id := range res.Down {
		b.removeDirect(tun, id)
	}
	for _, m := range res.MTU {
		tun.notePeerMTU(m)
	}
}

func (b *punchBind) sendOne(data []byte, ep wgconn.Endpoint, df bool) error {
	if df {
		restore := setBindDontFrag(b.inner)
		defer restore()
	}
	return b.inner.Send([][]byte{data}, ep)
}

func applyDirectIPC(dev *wgdevice.Device, ip [4]byte, inst punchInstall) error {
	conf := fmt.Sprintf(
		"public_key=%s\nendpoint=%s\nreplace_allowed_ips=true\nallowed_ip=%s/32\npersistent_keepalive_interval=10\n\n",
		hex.EncodeToString(inst.wgPub[:]), inst.via.String(), netip.AddrFrom4(ip).String(),
	)
	return applyWGIPC(dev, conf)
}

func (b *punchBind) removeDirect(tun *TUN, id uuid.UUID) {
	tun.punchMu.Lock()
	var inst punchInstall
	var ip [4]byte
	found := false
	for k, v := range tun.punchDirect {
		if v.id == id {
			inst = v
			ip = k
			found = true
			break
		}
	}
	dev := tun.wgDevice
	tun.punchMu.Unlock()
	if !found {
		b.clearRetry(id)
		return
	}
	if dev != nil {
		conf := fmt.Sprintf("public_key=%s\nremove=true\n\n", hex.EncodeToString(inst.wgPub[:]))
		if err := applyWGIPC(dev, conf); err == nil {
			deleted := false
			tun.punchMu.Lock()
			if cur, ok := tun.punchDirect[ip]; ok && cur.id == id {
				delete(tun.punchDirect, ip)
				deleted = true
			}
			tun.punchMu.Unlock()
			if deleted {
				tun.clearPeerMTU(ip)
			}
			b.clearRetry(id)
			INFO("punch: direct path down ", netip.AddrFrom4(ip).String())
			return
		}
	}
	if b.rememberRetry(id, inst) {
		ERROR("punch: remove failed for ", netip.AddrFrom4(ip).String())
	}
}

func (t *TUN) reinstallDirectPeers() {
	t.punchMu.Lock()
	if len(t.punchDirect) == 0 || t.wgDevice == nil {
		t.punchMu.Unlock()
		return
	}
	snap := make(map[[4]byte]punchInstall, len(t.punchDirect))
	for k, v := range t.punchDirect {
		snap[k] = v
	}
	dev := t.wgDevice
	t.punchMu.Unlock()
	for ip, inst := range snap {
		if err := applyDirectIPC(dev, ip, inst); err != nil {
			ERROR("punch reinstall ", netip.AddrFrom4(ip).String(), ": ", err)
		}
	}
}

func (b *punchBind) rememberRetry(id uuid.UUID, inst punchInstall) bool {
	b.retryMu.Lock()
	defer b.retryMu.Unlock()
	if b.retry == nil {
		b.retry = map[uuid.UUID]punchInstall{}
	}
	if _, ok := b.retry[id]; ok {
		return false
	}
	b.retry[id] = inst
	return true
}

func (b *punchBind) clearRetry(id uuid.UUID) {
	b.retryMu.Lock()
	delete(b.retry, id)
	b.retryMu.Unlock()
}

func (b *punchBind) retryRemoves(tun *TUN) {
	b.retryMu.Lock()
	if len(b.retry) == 0 {
		b.retryMu.Unlock()
		return
	}
	ids := make([]uuid.UUID, 0, len(b.retry))
	for id := range b.retry {
		ids = append(ids, id)
	}
	b.retryMu.Unlock()
	for _, id := range ids {
		b.removeDirect(tun, id)
	}
}

func (b *punchBind) injectIntro(tun *TUN, d punch.Datagram) {
	msg, err := punch.ParseIntro(d.Data)
	if err != nil || !msg.WGIP.Is4() || !d.To.Addr().Is4() {
		return
	}
	pkt, err := punch.WrapIPv4UDP(msg.WGIP, d.To.Addr(), punch.IntroPort, punch.IntroPort, d.Data)
	if err != nil || tun.punchInject == nil {
		return
	}
	select {
	case tun.punchInject <- pkt:
		signalPunchInject(tun)
	default:
		DEBUG("punch: intro queue full")
	}
}

func (t *TUN) takePunchIntro(packet []byte) bool {
	if t == nil || t.punchIn == nil {
		return false
	}
	src, srcPort, payload, ok := punch.UnwrapIPv4UDP(packet, punch.IntroPort)
	if !ok || punch.TypeOf(payload) != punch.TypeIntro {
		return false
	}
	from := netip.AddrPortFrom(src, srcPort)
	select {
	case t.punchIn <- punchDatagram{from: from.String(), data: append([]byte(nil), payload...)}:
	default:
	}
	return true
}

func ensurePunchWake(tun *TUN) {
	if tun == nil || tun.punchPoke != nil {
		return
	}
	tun.punchPoke = make(chan struct{}, 1)
}

func signalPunchInject(tun *TUN) {
	if tun == nil || tun.punchPoke == nil {
		return
	}
	select {
	case tun.punchPoke <- struct{}{}:
	default:
	}
}

func (t *TUN) drainInject(bufs [][]byte, sizes []int, offset int) int {
	if t == nil || t.punchInject == nil {
		return 0
	}
	n := 0
	for n < len(bufs) {
		var pkt []byte
		select {
		case pkt = <-t.punchInject:
		default:
			return n
		}
		if offset+len(pkt) > len(bufs[n]) {
			continue
		}
		copy(bufs[n][offset:], pkt)
		sizes[n] = len(pkt)
		n++
	}
	return n
}

func (t *TUN) notePunchWant(ip [4]byte) {
	if t.punchWant == nil {
		return
	}
	t.punchMu.Lock()
	if t.punchPending == nil {
		t.punchPending = map[[4]byte]struct{}{}
	}
	_, existed := t.punchPending[ip]
	t.punchPending[ip] = struct{}{}
	t.punchMu.Unlock()
	if existed {
		return
	}
	select {
	case t.punchWant <- ip:
	default:
	}
}

func (t *TUN) drainPunchWants(eng *punch.Engine) {
	if eng == nil {
		return
	}
	t.punchMu.Lock()
	pending := t.punchPending
	t.punchPending = nil
	t.punchMu.Unlock()
	for ip := range pending {
		eng.NoteTraffic(netip.AddrFrom4(ip))
	}
}

func (t *TUN) punchAllowsIngress(src [4]byte) bool {
	t.punchMu.Lock()
	_, direct := t.punchDirect[src]
	t.punchMu.Unlock()
	if !direct {
		return true
	}
	m := t.meta.Load()
	fw := t.ServerResponse != nil && t.ServerResponse.EnableFirewall
	allowAll := m != nil && m.AllowAll
	var entries []string
	if m != nil {
		entries = m.AllowedHosts
	}
	return punch.AdmitsAllPorts(!fw, allowAll, entries, netip.AddrFrom4(src))
}
