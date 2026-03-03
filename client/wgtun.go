package client

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/tun"
)

type chanTUN struct {
	in     chan []byte
	out    chan []byte
	events chan tun.Event
	mtu    int
	done   chan struct{}
}

func newChanTUN(mtu int) *chanTUN {
	ct := &chanTUN{
		in:     make(chan []byte, 64),
		out:    make(chan []byte, 64),
		events: make(chan tun.Event, 1),
		mtu:    mtu,
		done:   make(chan struct{}),
	}
	ct.events <- tun.EventUp
	return ct
}

func (t *chanTUN) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	select {
	case pkt, ok := <-t.in:
		if !ok {
			return 0, os.ErrClosed
		}
		if len(bufs) == 0 || len(bufs[0]) < offset+len(pkt) {
			return 0, fmt.Errorf("wgtun: buffer too small (%d < %d)", len(bufs[0]), offset+len(pkt))
		}
		copy(bufs[0][offset:], pkt)
		sizes[0] = len(pkt)
		return 1, nil
	case <-t.done:
		return 0, os.ErrClosed
	}
}

func (t *chanTUN) Write(bufs [][]byte, offset int) (int, error) {
	for _, buf := range bufs {
		if len(buf) <= offset {
			continue
		}
		pkt := make([]byte, len(buf)-offset)
		copy(pkt, buf[offset:])
		select {
		case t.out <- pkt:
		case <-t.done:
			return 0, os.ErrClosed
		}
	}
	return len(bufs), nil
}

func (t *chanTUN) BatchSize() int           { return 1 }
func (t *chanTUN) File() *os.File           { return nil }
func (t *chanTUN) MTU() (int, error)        { return t.mtu, nil }
func (t *chanTUN) Name() (string, error)    { return "wgtun", nil }
func (t *chanTUN) Events() <-chan tun.Event { return t.events }

func (t *chanTUN) Close() error {
	select {
	case <-t.done:

	default:
		close(t.done)
	}
	return nil
}

func (t *chanTUN) writeEgress(pkt []byte) {
	cp := make([]byte, len(pkt))
	copy(cp, pkt)
	select {
	case t.in <- cp:
	case <-t.done:
	default:
	}
}

func (t *chanTUN) readIngress() ([]byte, bool) {
	select {
	case pkt, ok := <-t.out:
		return pkt, ok
	case <-t.done:
		return nil, false
	}
}

func generateWGPrivKey() (string, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return "", fmt.Errorf("generateWGPrivKey: %w", err)
	}

	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	return base64.StdEncoding.EncodeToString(priv[:]), nil
}

func deriveWGPubKey(privB64 string) (string, error) {
	privBytes, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		return "", fmt.Errorf("deriveWGPubKey: base64: %w", err)
	}
	if len(privBytes) != 32 {
		return "", fmt.Errorf("deriveWGPubKey: expected 32 bytes, got %d", len(privBytes))
	}
	pub, err := curve25519.X25519(privBytes, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("deriveWGPubKey: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pub), nil
}

func wgB64ToHex(b64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("wgB64ToHex: base64: %w", err)
	}
	if len(b) != 32 {
		return "", fmt.Errorf("wgB64ToHex: expected 32 bytes, got %d", len(b))
	}
	return hex.EncodeToString(b), nil
}
