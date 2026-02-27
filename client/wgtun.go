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

// chanTUN is a channel-based tun.Device that bridges the tunnels packet
// processing loops with the wireguard-go device. No kernel TUN is created;
// wireguard-go uses it purely as a read/write seam for plaintext packets.
type chanTUN struct {
	in     chan []byte   // egress: ReadFromTunnelInterface → WG Read()
	out    chan []byte   // ingress: WG Write() → ReadFromServeTunnel
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

// Read is called by the wireguard-go device to retrieve plaintext egress
// packets that should be encrypted and sent to the peer.
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

// Write is called by the wireguard-go device to deliver plaintext ingress
// packets that have been received from the peer and decrypted.
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
		// already closed
	default:
		close(t.done)
	}
	return nil
}

// writeEgress is called by ReadFromTunnelInterface after ProcessEgressPacket.
// The packet is copied so the caller's buffer can be reused immediately.
// Drops silently if the channel is full (congestion drop).
func (t *chanTUN) writeEgress(pkt []byte) {
	cp := make([]byte, len(pkt))
	copy(cp, pkt)
	select {
	case t.in <- cp:
	case <-t.done:
	default:
	}
}

// readIngress is called by ReadFromServeTunnel to receive a decrypted packet
// delivered by wireguard-go. Returns (nil, false) when the TUN is closed.
func (t *chanTUN) readIngress() ([]byte, bool) {
	select {
	case pkt, ok := <-t.out:
		return pkt, ok
	case <-t.done:
		return nil, false
	}
}

// generateWGPrivKey generates a random Curve25519 private key and returns it
// base64-encoded (standard encoding, 44 chars).
func generateWGPrivKey() (string, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return "", fmt.Errorf("generateWGPrivKey: %w", err)
	}
	// Clamp per RFC 7748
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	return base64.StdEncoding.EncodeToString(priv[:]), nil
}

// deriveWGPubKey derives the Curve25519 public key from a base64-encoded
// private key and returns it base64-encoded.
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

// wgB64ToHex converts a base64-encoded 32-byte WireGuard key to lowercase hex,
// as required by the wireguard-go UAPI IPC protocol.
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
