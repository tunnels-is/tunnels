//go:build unix

package client

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"testing"
	"time"

	"golang.zx2c4.com/wireguard/tun"
)

type pipeTUN struct {
	r *os.File
}

// File ...
func (p *pipeTUN) File() *os.File { return p.r }

// Write ...
func (p *pipeTUN) Write([][]byte, int) (int, error) { return 0, nil }

// MTU ...
func (p *pipeTUN) MTU() (int, error) { return 1420, nil }

// Name ...
func (p *pipeTUN) Name() (string, error) { return "pipe", nil }

// Events ...
func (p *pipeTUN) Events() <-chan tun.Event { return nil }

// Close ...
func (p *pipeTUN) Close() error { return p.r.Close() }

// BatchSize ...
func (p *pipeTUN) BatchSize() int { return 1 }

// Read ...
func (p *pipeTUN) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	n, err := p.r.Read(bufs[0][offset:])
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	sizes[0] = n
	return 1, nil
}

// TestInjectWakesIdleRead ...
func TestInjectWakesIdleRead(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pr.Close()
		pw.Close()
	})
	tun := &TUN{punchInject: make(chan []byte, 4)}
	ensurePunchWake(tun)
	pt := newProcessingTUN(&pipeTUN{r: pr}, tun)
	got := make(chan []byte, 1)
	go func() {
		bufs := [][]byte{make([]byte, 256)}
		sizes := []int{0}
		n, err := pt.Read(bufs, sizes, 16)
		if err != nil || n != 1 {
			got <- nil
			return
		}
		got <- append([]byte(nil), bufs[0][16:16+sizes[0]]...)
	}()

	payload := []byte{9, 8, 7, 6}
	tun.punchInject <- payload
	signalPunchInject(tun)

	select {
	case pkt := <-got:
		if !bytes.Equal(pkt, payload) {
			t.Fatalf("read %v", pkt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("injected introduction did not wake an idle tunnel read")
	}
}

// TestDevicePacketWhilePunchAwake ...
func TestDevicePacketWhilePunchAwake(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pr.Close()
		pw.Close()
	})
	tun := &TUN{punchInject: make(chan []byte, 4)}
	ensurePunchWake(tun)
	pt := newProcessingTUN(&pipeTUN{r: pr}, tun)
	got := make(chan []byte, 1)
	go func() {
		bufs := [][]byte{make([]byte, 256)}
		sizes := []int{0}
		n, err := pt.Read(bufs, sizes, 16)
		if err != nil || n != 1 {
			got <- nil
			return
		}
		got <- append([]byte(nil), bufs[0][16:16+sizes[0]]...)
	}()

	pkt := make([]byte, 28)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], 28)
	pkt[9] = 17
	pkt[16], pkt[17], pkt[18], pkt[19] = 10, 0, 1, 2
	if _, err := pw.Write(pkt); err != nil {
		t.Fatal(err)
	}

	select {
	case gotPkt := <-got:
		if !bytes.Equal(gotPkt, pkt) {
			t.Fatalf("read %v", gotPkt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("tunnel packet was not delivered while punch wake was armed")
	}
}
