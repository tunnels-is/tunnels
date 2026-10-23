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

func (c *captureTUN) File() *os.File           { return nil }
func (c *captureTUN) MTU() (int, error)        { return 1420, nil }
func (c *captureTUN) Name() (string, error)    { return "cap", nil }
func (c *captureTUN) Events() <-chan tun.Event { return nil }
func (c *captureTUN) Read([][]byte, []int, int) (int, error) {
	return 0, nil
}
func (c *captureTUN) Close() error   { return nil }
func (c *captureTUN) BatchSize() int { return 128 }
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
