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

	// maxConcurrentHandshakes caps the number of in-flight handleInitiation
	// goroutines (each may cost an X25519 + a controller round-trip). Excess
	// handshake packets are dropped; the client retries.
	maxConcurrentHandshakes = 128

	// deniedKeyTTL debounces controller lookups for a pubkey the controller
	// just rejected — a flood of handshakes for a revoked/unknown key would
	// otherwise translate 1:1 into controller requests. A re-enabled peer is
	// delayed at most this long.
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

	// closeCh is recreated on each Open and closed by Close. The reinject
	// ReceiveFunc selects on it so it returns net.ErrClosed when the bind is
	// closed during a BindUpdate (e.g. when listen_port is set) — otherwise that
	// routine would never stop and wireguard-go's BindUpdate would deadlock
	// waiting for it.
	closeMu sync.Mutex
	closeCh chan struct{}

	serverPriv []byte
	serverPub  []byte

	mu      sync.Mutex
	seenIPs map[netip.Addr]time.Time

	rateMu     sync.Mutex
	ratePerIP  int
	rateWindow map[netip.Addr]*ipRate

	// handshakeSem bounds concurrent handleInitiation goroutines (global
	// handshake budget, not just per-IP).
	handshakeSem chan struct{}

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
		seenIPs:      make(map[netip.Addr]time.Time),
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

// Close closes the underlying UDP bind. It is called by wireguard-go as part of
// normal BindUpdate cycles (e.g. when listen_port is set), not only at final
// shutdown — so it MUST NOT wipe identity material or signal permanent shutdown.
// Use Shutdown() for that.
func (b *LazyBind) Close() error {
	// Unblock the reinject ReceiveFunc for this bind generation so it returns
	// net.ErrClosed and wireguard-go's BindUpdate can finish waiting on it.
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
	if ok {
		if now.Sub(r.start) >= time.Second {
			r.count = 1
			r.start = now
			return true
		}
		r.count++
		return r.count <= b.ratePerIP
	}

	// New IP: sweep expired windows opportunistically and hard-cap the map
	// (mirrors seenIPs) so a spoofed-source flood cannot grow it without bound.
	if len(b.rateWindow) > 1000 {
		for k, v := range b.rateWindow {
			if now.Sub(v.start) >= time.Second {
				delete(b.rateWindow, k)
			}
		}
	}
	if len(b.rateWindow) >= maxSeenIPs {
		return false
	}
	b.rateWindow[ip] = &ipRate{count: 1, start: now}
	return true
}

// recentlyDenied reports whether the controller rejected this pubkey within
// deniedKeyTTL, so repeated handshakes for a revoked/unknown key don't each
// trigger a controller lookup.
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
	if len(b.deniedKeys) >= maxDeniedKeys {
		return
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
				// Cheap mac1 gate before any rate bookkeeping, X25519, goroutine
				// spawn, or controller call: drop handshake-init packets not
				// genuinely addressed to this server (junk/spoofed floods).
				if !validMAC1(data, b.serverPub) {
					continue
				}
				srcIP := eps[i].DstIP()
				if !b.handshakeRateAllowed(srcIP) {
					WARN("LazyBind: handshake rate limit exceeded for ", srcIP)
					continue
				}
				if b.shouldSync(srcIP) {
					// Global concurrency budget: past the cap the handshake is
					// dropped (client retries) instead of spawning an unbounded
					// number of goroutines.
					select {
					case b.handshakeSem <- struct{}{}:
						pkt := &bufferedPkt{
							data: make([]byte, len(data)),
							ep:   eps[i],
						}
						copy(pkt.data, data)
						go func() {
							defer func() { <-b.handshakeSem }()
							b.handleInitiation(pkt)
						}()
					default:
						WARN("LazyBind: handshake concurrency limit reached, dropping packet")
					}
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

func (b *LazyBind) reinjectReceive(closeCh chan struct{}) conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		select {
		case pkt := <-b.requeueCh:
			n := copy(bufs[0], pkt.data)
			sizes[0] = n
			eps[0] = pkt.ep
			return 1, nil
		case <-closeCh:
			// This bind generation was closed (BindUpdate/Close) — stop so
			// wireguard-go's receive-routine WaitGroup can drain.
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
		// Not encrypted to this server's static key (or malformed). Let
		// wireguard-go handle/drop it.
		b.requeue(pkt)
		return
	}

	// A pubkey the controller rejected moments ago: drop without another
	// controller round-trip (request-amplification guard).
	if b.recentlyDenied(pubKeyB64) {
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
		b.noteDenied(pubKeyB64)
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
