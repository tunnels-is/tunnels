package client

import (
	"os"
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/tun"
)

type fakeTUN struct {
	closed   bool
	name     string
	events   chan tun.Event
	closeCnt int
}

func newFakeTUN() *fakeTUN {
	return &fakeTUN{name: "tunnels", events: make(chan tun.Event)}
}

func (f *fakeTUN) File() *os.File                         { return nil }
func (f *fakeTUN) Read([][]byte, []int, int) (int, error) { return 0, nil }
func (f *fakeTUN) Write([][]byte, int) (int, error)       { return 0, nil }
func (f *fakeTUN) MTU() (int, error)                      { return 1420, nil }
func (f *fakeTUN) Name() (string, error)                  { return f.name, nil }
func (f *fakeTUN) Events() <-chan tun.Event               { return f.events }
func (f *fakeTUN) BatchSize() int                         { return 1 }
func (f *fakeTUN) Close() error {
	f.closeCnt++
	f.closed = true
	return nil
}

func TestStickyTUN_CloseDoesNotReleaseOS(t *testing.T) {
	inner := newFakeTUN()
	s := newStickyTUN(inner)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if inner.closed {
		t.Fatal("Close() must not destroy the OS TUN")
	}
	if s.CanReuse() {
		t.Fatal("File() is nil so CanReuse must be false")
	}
}

func TestStickyTUN_ReleaseClosesOS(t *testing.T) {
	inner := newFakeTUN()
	s := newStickyTUN(inner)
	_ = s.Close()
	if err := s.Release(); err != nil {
		t.Fatal(err)
	}
	if !inner.closed {
		t.Fatal("Release() must close the OS TUN")
	}
	if s.Release() != nil {
		t.Fatal("second Release must be a no-op")
	}
	if inner.closeCnt != 1 {
		t.Fatalf("inner Close count = %d, want 1", inner.closeCnt)
	}
}

func TestBuildWGIPC_ReplacePeers(t *testing.T) {
	s := buildWGIPC("aa", "bb", "1.2.3.4", "51820")
	for _, want := range []string{
		"replace_peers=true",
		"replace_allowed_ips=true",
		"allowed_ip=0.0.0.0/0",
		"endpoint=1.2.3.4:51820",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("ipc missing %q:\n%s", want, s)
		}
	}
}

func TestWGDeviceAlive_Nil(t *testing.T) {
	if wgDeviceAlive(nil) {
		t.Fatal("nil device is not alive")
	}
}
