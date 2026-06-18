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

// setupFW (re)initializes the package firewall tables for a test. The cleaner
// goroutine is intentionally not started here.
func setupFW(t *testing.T, subnet4, subnet6 string) {
	t.Helper()
	if err := initPeerList(subnet4, subnet6); err != nil {
		t.Fatal(err)
	}
}

func entry(t *testing.T, ip string) *peer {
	t.Helper()
	p, ok := fwClassify(mustAddr(t, ip))
	if !ok || p == nil {
		t.Fatalf("no peer entry for %s", ip)
	}
	return p
}

func flowLen(p *peer) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.flows)
}

func TestInitPeerList_RejectsLargeSubnet(t *testing.T) {
	if err := initPeerList("10.0.0.0/8", ""); err == nil {
		t.Fatal("subnet larger than /16 must be rejected")
	}
	if err := initPeerList("10.0.0.0/24", ""); err != nil {
		t.Fatalf("/24 should be accepted: %v", err)
	}
}

func TestV4Offset(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	for _, tc := range []struct {
		ip   string
		want uint32
		ok   bool
	}{
		{"10.0.0.0", 0, true},
		{"10.0.0.1", 1, true},
		{"10.0.0.255", 255, true},
		{"10.0.1.0", 0, false}, // outside /24
		{"8.8.8.8", 0, false},
	} {
		off, ok := v4Offset(mustAddr(t, tc.ip))
		if ok != tc.ok || (ok && off != tc.want) {
			t.Fatalf("%s: got (%d,%v) want (%d,%v)", tc.ip, off, ok, tc.want, tc.ok)
		}
	}
}

func TestResetPeer_DefaultDeny(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	p := entry(t, "10.0.0.5")
	if p.allowedContains(mustAddr(t, "10.0.0.10")) {
		t.Fatal("fresh peer must allow nobody")
	}
}

func TestSetAllowed_ReplaceAndClear(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	p := entry(t, "10.0.0.5")

	p.setAllowed([]netip.Addr{mustAddr(t, "10.0.0.10")}, false)
	if !p.allowedContains(mustAddr(t, "10.0.0.10")) {
		t.Fatal("allowed src should be present")
	}

	p.setAllowed([]netip.Addr{mustAddr(t, "10.0.0.11")}, false) // replace
	if p.allowedContains(mustAddr(t, "10.0.0.10")) {
		t.Fatal("replace-set must drop the old entry")
	}
	if !p.allowedContains(mustAddr(t, "10.0.0.11")) {
		t.Fatal("replace-set must contain the new entry")
	}

	p.setAllowed(nil, false) // clear
	if p.allowedContains(mustAddr(t, "10.0.0.11")) {
		t.Fatal("empty list must clear the allowlist")
	}
}

func TestSetAllowed_AllowAll(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	p := entry(t, "10.0.0.5")

	// allow-all permits any source, even one not on the (empty) list.
	p.setAllowed(nil, true)
	if !p.allowedContains(mustAddr(t, "10.0.0.99")) {
		t.Fatal("allow-all must permit any source")
	}
	if !p.allowedContains(mustAddr(t, "10.88.0.1")) {
		t.Fatal("allow-all must permit a cross-server source too")
	}

	// turning allow-all off (replace-set) reverts to the explicit list.
	p.setAllowed([]netip.Addr{mustAddr(t, "10.0.0.10")}, false)
	if p.allowedContains(mustAddr(t, "10.0.0.99")) {
		t.Fatal("clearing allow-all must revert to the explicit list")
	}
	if !p.allowedContains(mustAddr(t, "10.0.0.10")) {
		t.Fatal("explicit entry should remain after allow-all is cleared")
	}
}

func TestResetPeer_AllowAllDefaultsOff(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	if entry(t, "10.0.0.5").allowedContains(mustAddr(t, "10.0.0.99")) {
		t.Fatal("a fresh peer must not allow-all by default")
	}
}

func TestFlow_TouchAndMatch(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	p := entry(t, "10.0.0.5")

	k := flowKey{remote: mustAddr(t, "10.0.0.10"), rport: 22, lport: 1000, proto: 6}
	if p.flowMatch(k) {
		t.Fatal("untracked flow must not match")
	}
	p.touchFlow(k)
	if !p.flowMatch(k) {
		t.Fatal("tracked flow must match")
	}
	other := flowKey{remote: mustAddr(t, "10.0.0.10"), rport: 23, lport: 1000, proto: 6}
	if p.flowMatch(other) {
		t.Fatal("a different flow key must not match")
	}
}

func TestCleanFlows_AgesIdleKeepsActive(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	p := entry(t, "10.0.0.5")

	idle := flowKey{remote: mustAddr(t, "10.0.0.10"), rport: 1, lport: 1, proto: 6}
	active := flowKey{remote: mustAddr(t, "10.0.0.11"), rport: 2, lport: 2, proto: 6}
	p.touchFlow(idle)
	p.touchFlow(active)

	cleanFlows() // first pass: both seen since creation -> snapshot, keep
	if flowLen(p) != 2 {
		t.Fatalf("after first clean want 2 flows, got %d", flowLen(p))
	}

	p.touchFlow(active) // only 'active' sees traffic
	cleanFlows()        // idle unchanged -> dropped; active changed -> kept
	if flowLen(p) != 1 {
		t.Fatalf("after second clean want 1 flow, got %d", flowLen(p))
	}
	if !p.flowMatch(active) {
		t.Fatal("active flow must survive")
	}
}

func TestResetPeer_WipesPriorState(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	p1 := entry(t, "10.0.0.5")
	p1.setAllowed([]netip.Addr{mustAddr(t, "10.0.0.10")}, false)
	p1.touchFlow(flowKey{remote: mustAddr(t, "10.0.0.11"), rport: 1, lport: 1, proto: 6})

	resetPeer("10.0.0.5") // IP reuse / reconnect
	p2 := entry(t, "10.0.0.5")
	if p2 == p1 {
		t.Fatal("reset must install a fresh entry")
	}
	if p2.allowedContains(mustAddr(t, "10.0.0.10")) {
		t.Fatal("reset must wipe the allowlist")
	}
	if flowLen(p2) != 0 {
		t.Fatal("reset must wipe tracked flows")
	}
}

func TestDualStack_SharesEntry(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "fd00::/64")
	resetPeer("10.0.0.5", "fd00::5")
	p4, _ := fwClassify(mustAddr(t, "10.0.0.5"))
	p6, _ := fwClassify(mustAddr(t, "fd00::5"))
	if p4 == nil || p4 != p6 {
		t.Fatal("v4 and v6 addresses of one device must share a peer entry")
	}
}

func TestV6Map_CleanupOnReuse(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "fd00::/64")
	resetPeer("10.0.0.5", "fd00::aaaa") // device A
	resetPeer("10.0.0.5", "fd00::bbbb") // device B reuses the v4 IP

	if p, _ := fwClassify(mustAddr(t, "fd00::aaaa")); p != nil {
		t.Fatal("old device's v6 map entry must be reclaimed on v4 reuse")
	}
	if p, _ := fwClassify(mustAddr(t, "fd00::bbbb")); p == nil {
		t.Fatal("new device's v6 entry must be present")
	}
}

func TestPeerListSnapshot(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	entry(t, "10.0.0.5").setAllowed([]netip.Addr{mustAddr(t, "10.0.0.10"), mustAddr(t, "10.0.0.11")}, false)
	resetPeer("10.0.0.6") // no allowlist -> excluded from snapshot

	snap := peerListSnapshot()
	if len(snap) != 1 {
		t.Fatalf("want 1 peer with a policy, got %d", len(snap))
	}
	if len(snap[mustAddr(t, "10.0.0.5")]) != 2 {
		t.Fatalf("want 2 allowed srcs, got %d", len(snap[mustAddr(t, "10.0.0.5")]))
	}
}

func TestL4Ports(t *testing.T) {
	l4 := []byte{0x00, 0x16, 0x30, 0x39} // sport 22, dport 12345
	for _, proto := range []byte{6, protoUDP} {
		sp, dp := l4Ports(proto, l4)
		if sp != 22 || dp != 12345 {
			t.Fatalf("proto %d: got %d/%d", proto, sp, dp)
		}
	}
	if sp, dp := l4Ports(1 /*ICMP*/, l4); sp != 0 || dp != 0 {
		t.Fatalf("ICMP should report 0/0, got %d/%d", sp, dp)
	}
	if sp, dp := l4Ports(6, []byte{0x00}); sp != 0 || dp != 0 {
		t.Fatalf("short l4 should report 0/0, got %d/%d", sp, dp)
	}
}

func TestPeer_ConcurrentSafe(t *testing.T) {
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	p := entry(t, "10.0.0.5")

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			k := flowKey{remote: mustAddr(t, "10.0.0.10"), rport: uint16(n), lport: 1, proto: 6}
			for {
				select {
				case <-stop:
					return
				default:
					p.touchFlow(k)
					p.flowMatch(k)
					p.allowedContains(mustAddr(t, "10.0.0.20"))
				}
			}
		}(i)
	}
	for i := 0; i < 200; i++ {
		p.setAllowed([]netip.Addr{mustAddr(t, "10.0.0.20")}, false)
		cleanFlows()
	}
	close(stop)
	wg.Wait()
}
