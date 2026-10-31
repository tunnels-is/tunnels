package wgserver

import (
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/punch"
	"golang.zx2c4.com/wireguard/conn"
)

const (
	msgInitiation byte = 1

	rateSlotCount = 4096

	maxConcurrentHandshakes = 128
	maxKnownHandshakes      = 32

	deniedKeyTTL  = 10 * time.Second
	maxDeniedKeys = 65_536
)

type bufferedPkt struct {
	data []byte
	ep   conn.Endpoint
}

type rateSlot struct {
	key   [16]byte
	sec   int64
	count int
}

type LazyBind struct {
	inner     conn.Bind
	requeueCh chan *bufferedPkt
	done      chan struct{}
	closeOnce sync.Once

	closeMu sync.Mutex
	closeCh chan struct{}

	serverPriv []byte
	serverPub  []byte

	nowFn     func() time.Time
	rateMu    sync.Mutex
	ratePerIP int
	rateSlots [rateSlotCount]rateSlot

	handshakeSem chan struct{}
	knownSem     chan struct{}
	handshakeWG  sync.WaitGroup

	cookieMu       sync.Mutex
	cookieSecret   [32]byte
	cookieSecretAt time.Time
	cookieSec      int64
	cookieSent     int

	deniedMu   sync.Mutex
	deniedKeys map[string]time.Time
}

func newLazyBind(inner conn.Bind, serverPriv, serverPub []byte, bufferSize, ratePerIP int) *LazyBind {
	return &LazyBind{
		inner:        inner,
		requeueCh:    make(chan *bufferedPkt, bufferSize),
		done:         make(chan struct{}),
		serverPriv:   serverPriv,
		serverPub:    serverPub,
		ratePerIP:    ratePerIP,
		handshakeSem: make(chan struct{}, maxConcurrentHandshakes),
		knownSem:     make(chan struct{}, maxKnownHandshakes),
		deniedKeys:   make(map[string]time.Time),
	}
}

// Open ...
func (b *LazyBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	fns, actualPort, err := b.inner.Open(port)
	if err != nil {
		return nil, 0, err
	}

	b.closeMu.Lock()
	b.closeCh = make(chan struct{})
	closeCh := b.closeCh
	b.closeMu.Unlock()

	wrapped := make([]conn.ReceiveFunc, len(fns)+1)
	for i, fn := range fns {
		wrapped[i] = b.wrapReceive(fn)
	}
	wrapped[len(fns)] = b.reinjectReceive(closeCh)
	return wrapped, actualPort, nil
}

// Close ...
func (b *LazyBind) Close() error {

	b.closeMu.Lock()
	if b.closeCh != nil {
		select {
		case <-b.closeCh:
		default:
			close(b.closeCh)
		}
	}
	b.closeMu.Unlock()
	return b.inner.Close()
}

// Shutdown ...
func (b *LazyBind) Shutdown() {
	b.closeOnce.Do(func() {
		close(b.done)
	})
}

// WipeKeys ...
func (b *LazyBind) WipeKeys() {
	b.handshakeWG.Wait()
	zeroBytes(b.serverPriv)
	zeroBytes(b.serverPub)
	b.cookieMu.Lock()
	zeroBytes(b.cookieSecret[:])
	b.cookieMu.Unlock()
}

// SetMark ...
func (b *LazyBind) SetMark(mark uint32) error { return b.inner.SetMark(mark) }

// Send ...
func (b *LazyBind) Send(bufs [][]byte, ep conn.Endpoint) error { return b.inner.Send(bufs, ep) }

// ParseEndpoint ...
func (b *LazyBind) ParseEndpoint(s string) (conn.Endpoint, error) { return b.inner.ParseEndpoint(s) }

// BatchSize ...
func (b *LazyBind) BatchSize() int { return b.inner.BatchSize() }

func (b *LazyBind) now() time.Time {
	if b.nowFn != nil {
		return b.nowFn()
	}
	return time.Now()
}

func ratePrefixKey(ip netip.Addr) ([16]byte, bool) {
	var key [16]byte
	if !ip.IsValid() {
		return key, false
	}
	ip = ip.Unmap()
	if ip.Is4() {
		addr := ip.As4()
		key[0] = 4
		key[1] = addr[0]
		key[2] = addr[1]
		key[3] = addr[2]
		return key, true
	}
	if ip.Is6() {
		addr := ip.As16()
		key[0] = 6
		copy(key[1:9], addr[:8])
		return key, true
	}
	return key, false
}

func rateSlotIndex(key [16]byte) uint32 {
	h := uint32(2166136261)
	for _, c := range key {
		h ^= uint32(c)
		h *= 16777619
	}
	return h % rateSlotCount
}

func (b *LazyBind) handshakeRateAllowed(ip netip.Addr) bool {
	key, ok := ratePrefixKey(ip)
	if !ok {
		return false
	}
	limit := b.ratePerIP
	if limit <= 0 {
		limit = 100
	}
	slot := &b.rateSlots[rateSlotIndex(key)]
	now := b.now().Unix()

	b.rateMu.Lock()
	defer b.rateMu.Unlock()
	if slot.sec == now && slot.key != key {
		return false
	}
	if slot.sec != now || slot.key != key {
		slot.key = key
		slot.sec = now
		slot.count = 1
		return true
	}
	slot.count++
	return slot.count <= limit
}

func (b *LazyBind) recentlyDenied(pubKeyB64 string) bool {
	b.deniedMu.Lock()
	defer b.deniedMu.Unlock()
	t, ok := b.deniedKeys[pubKeyB64]
	if !ok {
		return false
	}
	if time.Since(t) >= deniedKeyTTL {
		delete(b.deniedKeys, pubKeyB64)
		return false
	}
	return true
}

func (b *LazyBind) noteDenied(pubKeyB64 string) {
	b.deniedMu.Lock()
	defer b.deniedMu.Unlock()
	if len(b.deniedKeys) > 1000 {
		for k, t := range b.deniedKeys {
			if time.Since(t) >= deniedKeyTTL {
				delete(b.deniedKeys, k)
			}
		}
	}

	for len(b.deniedKeys) >= maxDeniedKeys {
		for k := range b.deniedKeys {
			delete(b.deniedKeys, k)
			break
		}
	}
	b.deniedKeys[pubKeyB64] = time.Now()
}

func (b *LazyBind) wrapReceive(inner conn.ReceiveFunc) conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		n, err := inner(bufs, sizes, eps)
		if err != nil {
			return n, err
		}

		out := 0
		for i := 0; i < n; i++ {
			data := bufs[i][:sizes[i]]
			if punch.Is(data) {
				pkt := make([]byte, len(data))
				copy(pkt, data)
				from := ""
				if eps[i] != nil {
					from = eps[i].DstToString()
				}
				enqueuePunch(from, pkt)
				continue
			}
			if isHandshakeInit(data) {
				b.takeHandshake(data, eps[i])
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

func (b *LazyBind) reinjectReceive(closeCh chan struct{}) conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		select {
		case pkt := <-b.requeueCh:
			n := copy(bufs[0], pkt.data)
			sizes[0] = n
			eps[0] = pkt.ep
			return 1, nil
		case <-closeCh:

			return 0, net.ErrClosed
		case <-b.done:
			return 0, net.ErrClosed
		}
	}
}

func (b *LazyBind) requeue(pkt *bufferedPkt) {
	select {
	case b.requeueCh <- pkt:
	case <-b.done:
	default:
		slowWarn.do("requeue-full", "LazyBind: requeue buffer full, dropping handshake packet")
	}
}

func (b *LazyBind) handshakeBusy() bool {
	return len(b.handshakeSem) >= cap(b.handshakeSem)
}

func (b *LazyBind) takeHandshake(data []byte, ep conn.Endpoint) {
	if ep == nil || !validMAC1(data, b.serverPub) {
		return
	}
	if !b.handshakeRateAllowed(ep.DstIP()) {
		slowWarn.do("handshake-rate", "LazyBind: handshake rate limit exceeded for ", ep.DstIP())
		return
	}
	if b.handshakeBusy() || !b.offerHandshake(data, ep, true) {
		b.cookieOrKnown(data, ep)
	}
}

func (b *LazyBind) cookieOrKnown(data []byte, ep conn.Endpoint) {
	src := ep.DstToBytes()
	if !b.validMAC2(data, src) {
		reply, err := b.writeCookieReply(data, src)
		if err != nil {
			slowWarn.do("cookie", "LazyBind: cookie reply: ", err)
			return
		}
		if err := b.Send([][]byte{reply}, ep); err != nil {
			slowWarn.do("cookie", "LazyBind: cookie send: ", err)
		}
		return
	}
	if !b.offerHandshake(data, ep, false) {
		slowWarn.do("handshake-slots", "LazyBind: handshake concurrency limit reached, dropping packet")
	}
}

func (b *LazyBind) offerHandshake(data []byte, ep conn.Endpoint, allowLookup bool) bool {
	sem := b.handshakeSem
	if !allowLookup {
		sem = b.knownSem
	}
	select {
	case sem <- struct{}{}:
	default:
		return false
	}
	pkt := &bufferedPkt{
		data: append([]byte(nil), data...),
		ep:   ep,
	}
	b.handshakeWG.Add(1)
	go func() {
		defer b.handshakeWG.Done()
		defer func() { <-sem }()
		b.handleInitiation(pkt, allowLookup)
	}()
	return true
}

func (b *LazyBind) handleInitiation(pkt *bufferedPkt, allowLookup bool) {
	pubKeyB64, ok := tryDecryptInitiator(pkt.data, b.serverPriv, b.serverPub)
	if !ok {
		slowWarn.do("bad-initiation", "LazyBind: dropping initiation that failed the timestamp check")
		return
	}

	if b.recentlyDenied(pubKeyB64) {
		return
	}

	if hexKey, err := b64ToHex(pubKeyB64); err == nil {
		if _, installed := installedPeerAddrs.Load(hexKey); installed {
			b.requeue(pkt)
			b.refreshInstalled(pubKeyB64)
			return
		}
	}
	if !allowLookup {
		return
	}

	switch reconcilePeer(pubKeyB64) {
	case authAllowed:
		b.requeue(pkt)
	case authDenied:
		b.noteDenied(pubKeyB64)
		INFO("LazyBind: peer no longer authorized, removing → ", pubKeyB64[:12], "…")
	case authUnknown:
		b.requeue(pkt)
	}
}

func (b *LazyBind) refreshInstalled(pubKeyB64 string) {
	if !claimPeerRefresh(pubKeyB64, time.Now()) {
		return
	}
	go func() {
		if reconcilePeer(pubKeyB64) == authDenied {
			b.noteDenied(pubKeyB64)
			INFO("LazyBind: peer no longer authorized, removing → ", pubKeyB64[:12], "…")
		}
	}()
}

func isHandshakeInit(pkt []byte) bool {
	return len(pkt) >= 4 &&
		pkt[0] == msgInitiation &&
		pkt[1] == 0 &&
		pkt[2] == 0 &&
		pkt[3] == 0
}
