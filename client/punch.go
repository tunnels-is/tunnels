package client

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	neturl "net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/punch"
	"github.com/tunnels-is/tunnels/types"
	wgconn "golang.zx2c4.com/wireguard/conn"
	wgdevice "golang.zx2c4.com/wireguard/device"
)

type punchDatagram struct {
	from string
	data []byte
}

// punchInstall is one direct WireGuard peer this tunnel added. The public
// key is kept so a revoke can remove that peer without wiping the server peer.
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
}

func newPunchBind(inner wgconn.Bind, tun *TUN) wgconn.Bind {
	if tun.punchIn == nil {
		tun.punchIn = make(chan punchDatagram, 64)
	}
	if tun.punchWant == nil {
		tun.punchWant = make(chan [4]byte, 16)
	}
	b := &punchBind{inner: inner}
	b.tun.Store(tun)
	go b.loop()
	return b
}

func (b *punchBind) Open(port uint16) ([]wgconn.ReceiveFunc, uint16, error) {
	fns, actual, err := b.inner.Open(port)
	if err != nil {
		return nil, 0, err
	}
	out := make([]wgconn.ReceiveFunc, len(fns))
	for i, fn := range fns {
		out[i] = b.wrap(fn)
	}
	return out, actual, nil
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

func (b *punchBind) Close() error {
	b.closed.Store(true)
	return b.inner.Close()
}
func (b *punchBind) SetMark(mark uint32) error { return b.inner.SetMark(mark) }
func (b *punchBind) Send(bufs [][]byte, ep wgconn.Endpoint) error {
	return b.inner.Send(bufs, ep)
}
func (b *punchBind) ParseEndpoint(s string) (wgconn.Endpoint, error) {
	return b.inner.ParseEndpoint(s)
}
func (b *punchBind) BatchSize() int { return b.inner.BatchSize() }

func (b *punchBind) loop() {
	defer RecoverAndLog()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var eng *punch.Engine
	var refreshed time.Time
	for !b.closed.Load() {
		tun := b.tun.Load()
		if tun == nil || tun.punchIn == nil || tun.punchWant == nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if eng == nil {
			eng = b.buildEngine(tun)
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
			b.apply(tun, eng.OnPacket(from, pkt.data, time.Now()))
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
			b.apply(tun, b.pushLocalPolicy(tun, eng))
			if refreshed.IsZero() || time.Since(refreshed) > 15*time.Second {
				b.refreshDirectory(tun, eng)
				b.apply(tun, b.pushLocalPolicy(tun, eng))
				refreshed = time.Now()
			}
			tun.drainPunchWants(eng)
			if tun.wgDevice != nil {
				b.apply(tun, eng.Poll(time.Now()))
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
	pub, err := punch.Public(disco)
	if err != nil {
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
	b.registerDisco(tun, punch.EncodeKey(pub))
	return punch.NewEngine(id, disco, wgPub, ip)
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

func (b *punchBind) refreshDirectory(tun *TUN, eng *punch.Engine) {
	cr := tun.CR
	if cr == nil || cr.Server == nil || tun.deviceID == "" {
		return
	}
	if dev, err := loadLocalDevice(tun.deviceID); err == nil && dev != nil {
		if disco := discoPublicB64(dev.WireGuardPrivKey); disco != "" {
			b.registerDisco(tun, disco)
		}
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
	peers := b.fetchPeers(tun)
	if peers != nil {
		b.apply(tun, eng.SetPeers(peers))
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
		return nil
	}
	cfg := new(wgServerConfig)
	if json.Unmarshal(body, cfg) != nil {
		return nil
	}
	return cfg
}

func (b *punchBind) fetchPeers(tun *TUN) []punch.Peer {
	cr := tun.CR
	url := cr.Server.GetURL("/client/p2p/peers") + "?deviceID=" + tun.deviceID
	body, code, err := SendRequestToURL(nil, "GET", url, nil, 8000, cr.Server.ValidateCertificate, cr.Server.CertificatePath, authHeaders(cr))
	if err != nil || code != 200 {
		return nil
	}
	var resp struct {
		Peers []types.P2PPeer `json:"Peers"`
	}
	if json.Unmarshal(body, &resp) != nil {
		return nil
	}
	out := make([]punch.Peer, 0, len(resp.Peers))
	for _, p := range resp.Peers {
		id, err := uuid.Parse(p.DeviceID)
		if err != nil {
			continue
		}
		disco, err := punch.DecodeKey(p.DiscoKey)
		if err != nil {
			continue
		}
		wg, err := punch.DecodeKey(p.WireGuardKey)
		if err != nil {
			continue
		}
		ip, err := netip.ParseAddr(p.WireGuardIP)
		if err != nil {
			continue
		}
		out = append(out, punch.Peer{DeviceID: id, DiscoPub: disco, WGPub: wg, WGIP: ip})
	}
	return out
}

func (b *punchBind) registerDisco(tun *TUN, disco string) {
	cr := tun.CR
	if cr == nil || cr.Server == nil {
		return
	}
	body := map[string]string{"DeviceID": tun.deviceID, "DiscoKey": disco}
	_, _, _ = SendRequestToURL(nil, "POST", cr.Server.GetURL("/client/device/disco"), body, 8000, cr.Server.ValidateCertificate, cr.Server.CertificatePath, authHeaders(cr))
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

func (b *punchBind) apply(tun *TUN, res punch.Result) {
	if tun == nil {
		return
	}
	cr := tun.CR
	serverAddr := ""
	if cr != nil && tun.ServerResponse != nil && tun.ServerResponse.WireGuardPort != "" {
		serverAddr = cr.ServerIP + ":" + tun.ServerResponse.WireGuardPort
	}
	for _, d := range res.Sends {
		to := d.To.String()
		if d.Server {
			to = serverAddr
		}
		if to == "" {
			continue
		}
		ep, err := b.ParseEndpoint(to)
		if err != nil {
			continue
		}
		_ = b.Send([][]byte{d.Data}, ep)
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
			tun.punchMu.Lock()
			if cur, ok := tun.punchDirect[ip]; ok && cur.id == id {
				delete(tun.punchDirect, ip)
			}
			tun.punchMu.Unlock()
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

func discoPublicB64(wgPrivB64 string) string {
	raw, err := base64.StdEncoding.DecodeString(wgPrivB64)
	if err != nil || len(raw) != 32 {
		return ""
	}
	var priv [32]byte
	copy(priv[:], raw)
	pub, err := punch.Public(punch.DiscoPrivateFor(priv))
	if err != nil {
		return ""
	}
	return punch.EncodeKey(pub)
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
