package wgserver

import (
	"net/netip"
	"sync"
	"testing"
)

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestACL_DefaultOpen(t *testing.T) {
	s := NewACLStore()
	dst := mustAddr(t, "10.0.0.5")
	src := mustAddr(t, "10.0.0.10")
	if !s.Allowed(src, dst) {
		t.Fatal("no policy => default open")
	}
}

func TestACL_AllowSpecific(t *testing.T) {
	s := NewACLStore()
	dst := mustAddr(t, "10.0.0.5")
	allowed := mustAddr(t, "10.0.0.10")
	denied := mustAddr(t, "10.0.0.20")

	s.Set(dst, []netip.Addr{allowed})

	if !s.Allowed(allowed, dst) {
		t.Fatal("allowed src should pass")
	}
	if s.Allowed(denied, dst) {
		t.Fatal("non-allowed src should be denied")
	}
}

func TestACL_EmptyListIsTotalIsolation(t *testing.T) {
	s := NewACLStore()
	dst := mustAddr(t, "10.0.0.5")
	src := mustAddr(t, "10.0.0.10")
	s.Set(dst, nil)
	if s.Allowed(src, dst) {
		t.Fatal("empty allowlist must deny all")
	}
}

func TestACL_ReplaceSemantics(t *testing.T) {
	s := NewACLStore()
	dst := mustAddr(t, "10.0.0.5")
	a := mustAddr(t, "10.0.0.10")
	b := mustAddr(t, "10.0.0.11")

	s.Set(dst, []netip.Addr{a})
	s.Set(dst, []netip.Addr{b}) // replace, not append

	if s.Allowed(a, dst) {
		t.Fatal("replaced policy should not contain a anymore")
	}
	if !s.Allowed(b, dst) {
		t.Fatal("replaced policy should contain b")
	}
}

func TestACL_Clear(t *testing.T) {
	s := NewACLStore()
	dst := mustAddr(t, "10.0.0.5")
	src := mustAddr(t, "10.0.0.10")
	s.Set(dst, nil) // total isolation
	if s.Allowed(src, dst) {
		t.Fatal("setup: should be denied")
	}
	s.Clear(dst)
	if !s.Allowed(src, dst) {
		t.Fatal("clear should revert to default-open")
	}
}

func TestACL_PerDestinationIndependent(t *testing.T) {
	s := NewACLStore()
	dstA := mustAddr(t, "10.0.0.5")
	dstB := mustAddr(t, "10.0.0.6")
	x := mustAddr(t, "10.0.0.10")

	s.Set(dstA, []netip.Addr{x})
	// dstB has no policy.

	if !s.Allowed(x, dstA) {
		t.Fatal("dstA should allow x")
	}
	if !s.Allowed(x, dstB) {
		t.Fatal("dstB should default-open")
	}
	// Random src should be denied for dstA.
	if s.Allowed(mustAddr(t, "10.0.0.99"), dstA) {
		t.Fatal("dstA should not allow unlisted src")
	}
}

func TestACL_IPv6(t *testing.T) {
	s := NewACLStore()
	dst := mustAddr(t, "fd00::5")
	src := mustAddr(t, "fd00::10")
	s.Set(dst, []netip.Addr{src})
	if !s.Allowed(src, dst) {
		t.Fatal("v6 allow failed")
	}
	if s.Allowed(mustAddr(t, "fd00::99"), dst) {
		t.Fatal("v6 deny failed")
	}
}

func TestACL_Snapshot(t *testing.T) {
	s := NewACLStore()
	dst := mustAddr(t, "10.0.0.5")
	s.Set(dst, []netip.Addr{mustAddr(t, "10.0.0.10"), mustAddr(t, "10.0.0.11")})

	snap := s.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 dst, got %d", len(snap))
	}
	if len(snap[dst]) != 2 {
		t.Fatalf("expected 2 srcs, got %d", len(snap[dst]))
	}
}

func TestACL_ConcurrentSafe(t *testing.T) {
	// Catch the obvious race conditions under -race.
	s := NewACLStore()
	dst := mustAddr(t, "10.0.0.5")
	src := mustAddr(t, "10.0.0.10")

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = s.Allowed(src, dst)
				}
			}
		}()
	}
	for i := 0; i < 100; i++ {
		s.Set(dst, []netip.Addr{src})
		s.Clear(dst)
	}
	close(stop)
	wg.Wait()
}
