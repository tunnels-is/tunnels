package client

import (
	"encoding/binary"
	"os"
	"sync"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/tun"
)

type captureTUN struct {
	mu     sync.Mutex
	writes int
	pkts   int
}

// File ...
func (c *captureTUN) File() *os.File { return nil }

// MTU ...
func (c *captureTUN) MTU() (int, error) { return 1420, nil }

// Name ...
func (c *captureTUN) Name() (string, error) { return "cap", nil }

// Events ...
func (c *captureTUN) Events() <-chan tun.Event { return nil }

// Read ...
func (c *captureTUN) Read([][]byte, []int, int) (int, error) {
	return 0, nil
}

// Close ...
func (c *captureTUN) Close() error { return nil }

// BatchSize ...
func (c *captureTUN) BatchSize() int { return 128 }

// Write ...
func (c *captureTUN) Write(bufs [][]byte, _ int) (int, error) {
	c.mu.Lock()
	c.writes++
	c.pkts += len(bufs)
	c.mu.Unlock()
	return len(bufs), nil
}

func (c *captureTUN) snapshot() (writes, pkts int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writes, c.pkts
}

func tcpIngressBuf(offset int) []byte {
	buf := make([]byte, offset+40)
	buf[offset] = 0x45
	binary.BigEndian.PutUint16(buf[offset+2:], 40)
	buf[offset+9] = 6
	return buf
}

// TestIngressWriteBatches ...
func TestIngressWriteBatches(t *testing.T) {
	capDev := &captureTUN{}
	pt := newProcessingTUN(capDev, &TUN{})
	for i := 0; i < 40; i++ {
		if _, err := pt.Write([][]byte{tcpIngressBuf(16)}, 16); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(30 * time.Millisecond)
	writes, pkts := capDev.snapshot()
	if pkts != 40 {
		t.Fatalf("packets %d, want 40", pkts)
	}
	if writes < 1 || writes >= 40 {
		t.Fatalf("writes %d, want a batch smaller than one write per packet", writes)
	}
}
