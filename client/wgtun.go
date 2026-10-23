package client

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/curve25519"
	"golang.zx2c4.com/wireguard/tun"
)

// Ingress is handed to the TUN a few packets at a time. Writing each one is a
// syscall, which dominated receive CPU. Hold a short batch so the TUN GRO path
// can merge a flow into one large write. 1ms keeps interactive packets moving.
const (
	ingressBatchPackets = 32
	ingressBatchBytes   = 48 * 1024
	ingressBatchWait    = time.Millisecond
	ingressBufCap       = 65535 + 64
	ingressFreeMax      = 96
)

type processingTUN struct {
	tun.Device
	tunnel    atomic.Pointer[TUN]
	egressMu  sync.Mutex
	ingressMu sync.Mutex

	batchMu      sync.Mutex
	pending      [][]byte
	pendingBytes int
	batchOff     int
	free         [][]byte
	flushing     bool
	armed        bool
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
	t := p.tun()
	if t == nil {
		p.ingressMu.Unlock()
		return len(bufs), nil
	}

	owned := make([][]byte, 0, len(bufs))
	var ownedBytes int
	var direct [][]byte
	var directBytes int
	for _, buf := range bufs {
		if len(buf) <= offset {
			continue
		}
		packet := buf[offset:]
		if !t.ProcessIngressPacket(packet) {
			continue
		}
		need := offset + len(packet)
		if need > ingressBufCap {
			direct = append(direct, buf)
			directBytes += len(packet)
			continue
		}
		dst := p.takeBuf()
		if cap(dst) < need {
			dst = make([]byte, need, ingressBufCap)
		}
		dst = dst[:need]
		copy(dst[offset:], packet)
		owned = append(owned, dst)
		ownedBytes += len(packet)
	}
	p.ingressMu.Unlock()

	if len(direct) > 0 {
		if _, err := p.Device.Write(direct, offset); err != nil {
			ERROR("tunnel ingress write: ", err)
		} else {
			t.ingressBytes.Add(int64(directBytes))
		}
	}
	if len(owned) == 0 {
		return len(bufs), nil
	}
	p.enqueueIngress(owned, ownedBytes, offset)
	return len(bufs), nil
}

func (p *processingTUN) takeBuf() []byte {
	p.batchMu.Lock()
	n := len(p.free)
	if n > 0 {
		b := p.free[n-1]
		p.free = p.free[:n-1]
		p.batchMu.Unlock()
		return b[:0]
	}
	p.batchMu.Unlock()
	return make([]byte, 0, ingressBufCap)
}

func (p *processingTUN) enqueueIngress(owned [][]byte, n, offset int) {
	p.batchMu.Lock()
	if len(p.pending) > 0 && p.batchOff != offset {
		p.batchMu.Unlock()
		p.flushIngress()
		p.batchMu.Lock()
	}
	p.batchOff = offset
	p.pending = append(p.pending, owned...)
	p.pendingBytes += n
	full := len(p.pending) >= ingressBatchPackets || p.pendingBytes >= ingressBatchBytes
	if !full && !p.armed {
		p.armed = true
		time.AfterFunc(ingressBatchWait, p.flushIngress)
	}
	p.batchMu.Unlock()
	if full {
		p.flushIngress()
	}
}

func (p *processingTUN) flushIngress() {
	defer RecoverAndLog()
	p.batchMu.Lock()
	if p.flushing {
		p.batchMu.Unlock()
		return
	}
	p.flushing = true
	for {
		bufs := p.pending
		off := p.batchOff
		nBytes := p.pendingBytes
		p.pending = nil
		p.pendingBytes = 0
		p.armed = false
		if len(bufs) == 0 {
			p.flushing = false
			p.batchMu.Unlock()
			return
		}
		p.batchMu.Unlock()

		_, err := p.Device.Write(bufs, off)
		if err == nil {
			if t := p.tun(); t != nil {
				t.ingressBytes.Add(int64(nBytes))
			}
		} else {
			ERROR("tunnel ingress write: ", err)
		}

		p.batchMu.Lock()
		for _, b := range bufs {
			if cap(b) >= ingressBufCap && len(p.free) < ingressFreeMax {
				p.free = append(p.free, b[:0])
			}
		}
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
