package wgserver

import (
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/conn"
)

const (
	msgInitiation byte = 1

	maxRateWindowIPs = 200_000

	maxConcurrentHandshakes = 128

	deniedKeyTTL  = 10 * time.Second
	maxDeniedKeys = 65_536
)

type bufferedPkt struct {
	data []byte
	ep   conn.Endpoint
}

type ipRate struct {
	count int
	start time.Time
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

	rateMu     sync.Mutex
	ratePerIP  int
	rateWindow map[netip.Addr]*ipRate

	handshakeSem chan struct{}
	handshakeWG  sync.WaitGroup

	deniedMu   sync.Mutex
	deniedKeys map[string]time.Time
}

func NewLazyBind(inner conn.Bind, serverPriv, serverPub []byte, bufferSize, ratePerIP int) *LazyBind {
	return &LazyBind{
		inner:        inner,
		requeueCh:    make(chan *bufferedPkt, bufferSize),
		done:         make(chan struct{}),
		serverPriv:   serverPriv,
		serverPub:    serverPub,
		ratePerIP:    ratePerIP,
		rateWindow:   make(map[netip.Addr]*ipRate),
		handshakeSem: make(chan struct{}, maxConcurrentHandshakes),
		deniedKeys:   make(map[string]time.Time),
	}
}

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

func (b *LazyBind) Shutdown() {
	b.closeOnce.Do(func() {
		close(b.done)
	})
}

func (b *LazyBind) WipeKeys() {
	b.handshakeWG.Wait()
	zeroBytes(b.serverPriv)
	zeroBytes(b.serverPub)
}

func (b *LazyBind) SetMark(mark uint32) error                     { return b.inner.SetMark(mark) }
func (b *LazyBind) Send(bufs [][]byte, ep conn.Endpoint) error    { return b.inner.Send(bufs, ep) }
func (b *LazyBind) ParseEndpoint(s string) (conn.Endpoint, error) { return b.inner.ParseEndpoint(s) }
func (b *LazyBind) BatchSize() int                                { return b.inner.BatchSize() }

func (b *LazyBind) handshakeRateAllowed(ip netip.Addr) bool {
	b.rateMu.Lock()
	defer b.rateMu.Unlock()

	now := time.Now()
	r, ok := b.rateWindow[ip]
	if ok {
		if now.Sub(r.start) >= time.Second {
			r.count = 1
			r.start = now
			return true
		}
		r.count++
		return r.count <= b.ratePerIP
	}

	if len(b.rateWindow) > 1000 {
		for k, v := range b.rateWindow {
			if now.Sub(v.start) >= time.Second {
				delete(b.rateWindow, k)
			}
		}
	}
	if len(b.rateWindow) >= maxRateWindowIPs {
		return false
	}
	b.rateWindow[ip] = &ipRate{count: 1, start: now}
	return true
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
			if isHandshakeInit(data) {

				if !validMAC1(data, b.serverPub) {
					continue
				}
				srcIP := eps[i].DstIP()
				if !b.handshakeRateAllowed(srcIP) {
					WARN("LazyBind: handshake rate limit exceeded for ", srcIP)
					continue
				}

				select {
				case b.handshakeSem <- struct{}{}:
					pkt := &bufferedPkt{
						data: make([]byte, len(data)),
						ep:   eps[i],
					}
					copy(pkt.data, data)

					b.handshakeWG.Add(1)
					go func() {
						defer b.handshakeWG.Done()
						defer func() { <-b.handshakeSem }()
						b.handleInitiation(pkt)
					}()
				default:
					WARN("LazyBind: handshake concurrency limit reached, dropping packet")
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
		WARN("LazyBind: requeue buffer full, dropping handshake packet")
	}
}

func (b *LazyBind) handleInitiation(pkt *bufferedPkt) {
	pubKeyB64, ok := tryDecryptInitiator(pkt.data, b.serverPriv, b.serverPub)
	if !ok {

		b.requeue(pkt)
		return
	}

	if b.recentlyDenied(pubKeyB64) {
		return
	}

	switch reconcilePeer(pubKeyB64) {
	case authAllowed:
		b.requeue(pkt)
	case authDenied:
		b.noteDenied(pubKeyB64)
		INFO("LazyBind: peer no longer authorized, removing → ", pubKeyB64[:12], "…")
		if hexKey, err := b64ToHex(pubKeyB64); err == nil {
			_ = RemovePeer(hexKey)
			addedPeerKeys.Delete(hexKey)
			peerStore.DeleteByPubKey(pubKeyB64)
		}

	case authUnknown:

		b.requeue(pkt)
	}
}

func isHandshakeInit(pkt []byte) bool {
	return len(pkt) >= 4 &&
		pkt[0] == msgInitiation &&
		pkt[1] == 0 &&
		pkt[2] == 0 &&
		pkt[3] == 0
}
