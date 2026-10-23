package client

import (
	"testing"
	"time"

	wgconn "golang.zx2c4.com/wireguard/conn"
)

// punchStubBind is the smallest Bind that lets Open/Close run without a socket.
type punchStubBind struct{}

func (punchStubBind) Open(port uint16) ([]wgconn.ReceiveFunc, uint16, error) {
	return nil, port, nil
}
func (punchStubBind) Close() error                                  { return nil }
func (punchStubBind) SetMark(uint32) error                          { return nil }
func (punchStubBind) Send([][]byte, wgconn.Endpoint) error          { return nil }
func (punchStubBind) ParseEndpoint(string) (wgconn.Endpoint, error) { return nil, nil }
func (punchStubBind) BatchSize() int                                { return 1 }

func TestPunchBindSurvivesWireGuardRebind(t *testing.T) {
	prevS := STATE.Load()
	prevC := CONFIG.Load()
	t.Cleanup(func() {
		STATE.Store(prevS)
		CONFIG.Store(prevC)
	})
	STATE.Store(&State{})
	CONFIG.Store(&Config{})

	b := newPunchBind(punchStubBind{}, &TUN{}).(*punchBind)
	if _, _, err := b.Open(0); err != nil {
		t.Fatal(err)
	}
	waitPunchLoop(t, b, true)

	// WireGuard's first Up, and every later route change, does Close then Open
	// on this same bind. Close used to retire the loop for good.
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if !b.closed.Load() {
		t.Fatal("close did not pause punch")
	}
	if _, _, err := b.Open(0); err != nil {
		t.Fatal(err)
	}
	if b.closed.Load() {
		t.Fatal("rebind left punch paused")
	}
	time.Sleep(1200 * time.Millisecond)
	if b.closed.Load() {
		t.Fatal("punch stayed paused after rebind")
	}
	waitPunchLoop(t, b, true)

	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	waitPunchLoop(t, b, false)
}

func waitPunchLoop(t *testing.T, b *punchBind, want bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b.loopMu.Lock()
		got := b.looping
		b.loopMu.Unlock()
		if got == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	b.loopMu.Lock()
	got := b.looping
	b.loopMu.Unlock()
	t.Fatalf("punch loop running=%v, want %v", got, want)
}
