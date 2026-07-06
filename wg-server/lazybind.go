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

	syncDebounce = 30 * time.Second
	maxSeenIPs   = 200_000
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

	serverPriv []byte
	serverPub  []byte

	mu      sync.Mutex
	seenIPs map[netip.Addr]time.Time

	rateMu     sync.Mutex
	ratePerIP  int
	rateWindow map[netip.Addr]*ipRate
}

func NewLazyBind(inner conn.Bind, serverPriv, serverPub []byte, bufferSize, ratePerIP int) *LazyBind {
	return &LazyBind{
		inner:      inner,
		requeueCh:  make(chan *bufferedPkt, bufferSize),
		done:       make(chan struct{}),
		serverPriv: serverPriv,
		serverPub:  serverPub,
		seenIPs:    make(map[netip.Addr]time.Time),
		ratePerIP:  ratePerIP,
		rateWindow: make(map[netip.Addr]*ipRate),
	}
}

func (b *LazyBind) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	fns, actualPort, err := b.inner.Open(port)
	if err != nil {
		return nil, 0, err
	}

	wrapped := make([]conn.ReceiveFunc, len(fns)+1)
	for i, fn := range fns {
		wrapped[i] = b.wrapReceive(fn)
	}
	wrapped[len(fns)] = b.reinjectReceive()
	return wrapped, actualPort, nil
}

// Close closes the underlying UDP bind. It is called by wireguard-go as part of
// normal BindUpdate cycles (e.g. when listen_port is set), not only at final
// shutdown — so it MUST NOT wipe identity material or signal permanent shutdown.
// Use Shutdown() for that.
func (b *LazyBind) Close() error {
	return b.inner.Close()
}

// Shutdown performs the final teardown: signals the reinject goroutine to exit
// and wipes key material from memory. Call this exactly once when the wg-server
// process is exiting.
func (b *LazyBind) Shutdown() {
	b.closeOnce.Do(func() {
		close(b.done)
		zeroBytes(b.serverPriv)
		zeroBytes(b.serverPub)
	})
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
	if !ok || now.Sub(r.start) >= time.Second {
		b.rateWindow[ip] = &ipRate{count: 1, start: now}
		return true
	}
	r.count++
	return r.count <= b.ratePerIP
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
				srcIP := eps[i].DstIP()
				if !b.handshakeRateAllowed(srcIP) {
					WARN("LazyBind: handshake rate limit exceeded for ", srcIP)
					continue
				}
				if b.shouldSync(srcIP) {
					pkt := &bufferedPkt{
						data: make([]byte, len(data)),
						ep:   eps[i],
					}
					copy(pkt.data, data)
					go b.handleInitiation(pkt)
					continue
				}
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

func (b *LazyBind) reinjectReceive() conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		select {
		case pkt := <-b.requeueCh:
			n := copy(bufs[0], pkt.data)
			sizes[0] = n
			eps[0] = pkt.ep
			return 1, nil
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
		// Not encrypted to this server's static key (or malformed). Let
		// wireguard-go handle/drop it.
		b.requeue(pkt)
		return
	}

	// Reconcile authorization with the controller on every (re)connect. An
	// already-open session is not torn down proactively, but any new handshake
	// (initial connect, reconnect, or rekey) re-checks here — so a peer the
	// controller has since revoked cannot re-establish.
	switch reconcilePeer(pubKeyB64) {
	case authAllowed:
		b.requeue(pkt)
	case authDenied:
		INFO("LazyBind: peer no longer authorized, removing → ", pubKeyB64[:12], "…")
		if hexKey, err := b64ToHex(pubKeyB64); err == nil {
			_ = RemovePeer(hexKey)
			addedPeerKeys.Delete(hexKey)
			peerStore.DeleteByPubKey(pubKeyB64)
		}
		// Drop the handshake: the peer is not installed, so replaying it would
		// only fail.
	case authUnknown:
		// Transient controller error — leave peer state untouched and requeue so
		// an already-installed peer's handshake can still complete.
		b.requeue(pkt)
	}
}

func (b *LazyBind) shouldSync(ip netip.Addr) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if t, ok := b.seenIPs[ip]; ok && time.Since(t) < syncDebounce {
		return false
	}

	if len(b.seenIPs) > 1000 {
		for k, t := range b.seenIPs {
			if time.Since(t) >= syncDebounce {
				delete(b.seenIPs, k)
			}
		}
	}

	if len(b.seenIPs) >= maxSeenIPs {
		return false
	}

	b.seenIPs[ip] = time.Now()
	return true
}

func isHandshakeInit(pkt []byte) bool {
	return len(pkt) >= 4 &&
		pkt[0] == msgInitiation &&
		pkt[1] == 0 &&
		pkt[2] == 0 &&
		pkt[3] == 0
}
