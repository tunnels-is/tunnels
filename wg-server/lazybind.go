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

	// syncDebounce prevents redundant syncs for the same source IP within this window.
	syncDebounce = 30 * time.Second
)

type bufferedPkt struct {
	data []byte
	ep   conn.Endpoint
}

// LazyBind wraps conn.Bind and triggers an immediate SyncPeers() when a
// WireGuard handshake initiation arrives from a source IP that hasn't been
// seen recently. The triggering packet is buffered and re-injected into
// wireguard-go after the sync completes, so the handshake succeeds on the
// first attempt instead of waiting for the client's ~5s retry.
type LazyBind struct {
	inner     conn.Bind
	requeueCh chan *bufferedPkt
	done      chan struct{}
	closeOnce sync.Once

	mu       sync.Mutex
	seenIPs  map[netip.Addr]time.Time
	syncFunc func()
}

func NewLazyBind(inner conn.Bind, syncFunc func()) *LazyBind {
	return &LazyBind{
		inner:     inner,
		requeueCh: make(chan *bufferedPkt, 64),
		done:      make(chan struct{}),
		seenIPs:   make(map[netip.Addr]time.Time),
		syncFunc:  syncFunc,
	}
}

// Open wraps the inner bind's receive functions and appends one extra
// ReceiveFunc that delivers re-injected packets after a sync completes.
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
// new source IPs it triggers a sync, buffers the packet, and drops it from
// the current batch (it will be re-delivered via reinjectReceive).
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
					go b.doSync(pkt)
					continue // drop from batch; re-injected after sync
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
// channel and returns a buffered packet to wireguard-go after a sync completes.
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

// doSync runs SyncPeers and then re-injects the buffered packet.
func (b *LazyBind) doSync(pkt *bufferedPkt) {
	b.syncFunc()
	select {
	case b.requeueCh <- pkt:
	case <-b.done:
	}
}

// shouldSync returns true (and records the IP) if we haven't synced for this
// source IP recently. It also prunes stale entries to bound map growth.
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
