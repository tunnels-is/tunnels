package client

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/tun"
)

type processingTUN struct {
	tun.Device
	tunnel    atomic.Pointer[TUN]
	egressMu  sync.Mutex
	ingressMu sync.Mutex
}

func newProcessingTUN(d tun.Device, t *TUN) *processingTUN {
	p := &processingTUN{Device: d}
	p.tunnel.Store(t)
	return p
}

func (p *processingTUN) bindTunnel(t *TUN) {
	p.tunnel.Store(t)
}

func (p *processingTUN) tun() *TUN {
	return p.tunnel.Load()
}

func (p *processingTUN) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	defer RecoverAndLog()
	n, err := p.Device.Read(bufs, sizes, offset)
	if err != nil || n == 0 {
		return n, err
	}

	t := p.tun()
	if t == nil {
		return 0, nil
	}

	p.egressMu.Lock()
	defer p.egressMu.Unlock()

	kept := 0
	for i := 0; i < n; i++ {
		packet := bufs[i][offset : offset+sizes[i]]
		if !t.ProcessEgressPacket(&packet) {
			continue
		}
		if kept != i {
			copy(bufs[kept][offset:], packet)
		}
		sizes[kept] = len(packet)
		kept++
	}

	if kept > 0 {
		var total int64
		for i := 0; i < kept; i++ {
			total += int64(sizes[i])
		}
		t.egressBytes.Add(total)
	}

	return kept, nil
}

func (p *processingTUN) Write(bufs [][]byte, offset int) (int, error) {
	defer RecoverAndLog()
	p.ingressMu.Lock()
	defer p.ingressMu.Unlock()

	t := p.tun()
	if t == nil {
		return len(bufs), nil
	}

	var passthrough [][]byte
	var totalBytes int64
	for _, buf := range bufs {
		if len(buf) <= offset {
			continue
		}
		packet := buf[offset:]
		if !t.ProcessIngressPacket(packet) {
			continue
		}
		passthrough = append(passthrough, buf)
		totalBytes += int64(len(packet))
	}

	if len(passthrough) == 0 {
		return len(bufs), nil
	}

	n, err := p.Device.Write(passthrough, offset)
	if err == nil {
		t.ingressBytes.Add(totalBytes)
	}
	return n + (len(bufs) - len(passthrough)), err
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
