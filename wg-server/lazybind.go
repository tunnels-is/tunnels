package main

import (
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/conn"
)

const (
	// msgInitiation is WireGuard message type 1 (handshake initiation).
	msgInitiation byte = 1

	// syncDebounce prevents redundant work for the same source IP within this window.
	syncDebounce = 30 * time.Second
)

type bufferedPkt struct {
	data []byte
	ep   conn.Endpoint
}

// LazyBind wraps conn.Bind and intercepts WireGuard handshake initiation
// packets. When an initiation arrives it:
//  1. Decrypts the encrypted_static field (Noise Xk) to recover the
//     initiator's static public key — no controller call needed.
//  2. Looks up the pubkey in the local peer store.
//  3. If found, calls AddPeer immediately so the handshake succeeds on the
//     first attempt.
//  4. Falls back to a full SyncPeers if the pubkey is not in the store
//     (genuinely unknown peer).
//
// The triggering packet is always buffered and re-injected after the lookup
// so wireguard-go sees it after the peer is configured.
type LazyBind struct {
	inner      conn.Bind
	requeueCh  chan *bufferedPkt
	done       chan struct{}
	closeOnce  sync.Once

	serverPriv []byte // raw 32-byte Curve25519 private key
	serverPub  []byte // raw 32-byte Curve25519 public key

	mu      sync.Mutex
	seenIPs map[netip.Addr]time.Time

	fallbackSync func() // full SyncPeers, used when peer not in store
}

func NewLazyBind(inner conn.Bind, serverPriv, serverPub []byte, fallbackSync func()) *LazyBind {
	return &LazyBind{
		inner:        inner,
		requeueCh:    make(chan *bufferedPkt, 64),
		done:         make(chan struct{}),
		serverPriv:   serverPriv,
		serverPub:    serverPub,
		seenIPs:      make(map[netip.Addr]time.Time),
		fallbackSync: fallbackSync,
	}
}

// Open wraps the inner bind's receive functions and appends one extra
// ReceiveFunc that delivers re-injected packets after a lookup completes.
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

func (b *LazyBind) Close() error {
	err := b.inner.Close()
	b.closeOnce.Do(func() { close(b.done) })
	return err
}

func (b *LazyBind) SetMark(mark uint32) error                     { return b.inner.SetMark(mark) }
func (b *LazyBind) Send(bufs [][]byte, ep conn.Endpoint) error    { return b.inner.Send(bufs, ep) }
func (b *LazyBind) ParseEndpoint(s string) (conn.Endpoint, error) { return b.inner.ParseEndpoint(s) }
func (b *LazyBind) BatchSize() int                                 { return b.inner.BatchSize() }

// wrapReceive intercepts incoming packets. For handshake initiations from
// source IPs not seen recently it decrypts the initiator identity, adds the
// peer if known, then re-injects the packet.
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
				if b.shouldSync(srcIP) {
					pkt := &bufferedPkt{
						data: make([]byte, len(data)),
						ep:   eps[i],
					}
					copy(pkt.data, data)
					go b.handleInitiation(pkt)
					continue // drop from batch; re-injected after peer is added
				}
			}
			// Compact the output slice in-place.
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

// reinjectReceive is an extra ReceiveFunc goroutine that blocks on the requeue
// channel and returns a buffered packet to wireguard-go after a peer is added.
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

// handleInitiation decrypts the initiator's pubkey from the handshake packet,
// looks it up in the peer store, and calls AddPeer directly.
// Falls back to a full SyncPeers if the peer is not found in the local store.
func (b *LazyBind) handleInitiation(pkt *bufferedPkt) {
	INFO("LazyBind: handshake initiation received, attempting identity decrypt")

	pubKeyB64, ok := tryDecryptInitiator(pkt.data, b.serverPriv, b.serverPub)
	if !ok {
		INFO("LazyBind: decrypt failed (wrong server key?), falling back to full sync")
		b.fallbackSync()
		select {
		case b.requeueCh <- pkt:
		case <-b.done:
		}
		return
	}

	INFO("LazyBind: decrypted initiator pubkey=", pubKeyB64[:12], "…")

	rec, found := peerStore.GetByPubKey(pubKeyB64)
	if !found {
		INFO("LazyBind: pubkey not in peer store, attempting targeted assign from controller")
		if assignAndAdd(pubKeyB64) {
			INFO("LazyBind: targeted assign succeeded, re-injecting handshake")
			select {
			case b.requeueCh <- pkt:
			case <-b.done:
			}
			return
		}
		INFO("LazyBind: targeted assign failed, falling back to full sync")
		b.fallbackSync()
		select {
		case b.requeueCh <- pkt:
		case <-b.done:
		}
		return
	}

	INFO("LazyBind: peer found in store → ip=", rec.IP, " calling AddPeer")
	hexKey, err := b64ToHex(pubKeyB64)
	if err != nil {
		WARN("LazyBind: b64ToHex failed: ", err)
	} else if err := AddPeer(hexKey, rec.IP+"/32"); err != nil {
		WARN("LazyBind: AddPeer failed: ", err)
	} else {
		INFO("LazyBind: AddPeer OK → peer=", pubKeyB64[:12], "… ip=", rec.IP, " re-injecting handshake")
	}

	select {
	case b.requeueCh <- pkt:
	case <-b.done:
	}
}

// shouldSync returns true (and records the IP) if we haven't processed an
// initiation from this source IP recently.
func (b *LazyBind) shouldSync(ip netip.Addr) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if t, ok := b.seenIPs[ip]; ok && time.Since(t) < syncDebounce {
		return false
	}

	// Prune entries whose debounce window has expired.
	if len(b.seenIPs) > 1000 {
		for k, t := range b.seenIPs {
			if time.Since(t) >= syncDebounce {
				delete(b.seenIPs, k)
			}
		}
	}

	b.seenIPs[ip] = time.Now()
	return true
}

// isHandshakeInit reports whether pkt is a WireGuard handshake initiation
// (message type 1, little-endian uint32 in bytes 0–3).
func isHandshakeInit(pkt []byte) bool {
	return len(pkt) >= 4 &&
		pkt[0] == msgInitiation &&
		pkt[1] == 0 &&
		pkt[2] == 0 &&
		pkt[3] == 0
}
