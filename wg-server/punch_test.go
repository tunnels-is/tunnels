package wgserver

import (
	"net/netip"
	"testing"
)

func TestLocalAdmitsAllPorts(t *testing.T) {
	inspectDevice.Store(nil)
	setupFW(t, "10.0.0.0/24", "")
	resetPeer("10.0.0.5")
	p := entry(t, "10.0.0.5")
	peer := netip.MustParseAddr("10.0.1.8")
	self := netip.MustParseAddr("10.0.0.5")

	if localAdmitsAllPorts(self, peer) {
		t.Fatal("empty allowlist must not admit a direct path")
	}
	p.setAllowed([]aclEntry{{addr: peer, port: 9090}}, false)
	if localAdmitsAllPorts(self, peer) {
		t.Fatal("a single port must not admit a direct path")
	}
	p.setAllowed([]aclEntry{{addr: peer}}, false)
	if !localAdmitsAllPorts(self, peer) {
		t.Fatal("a bare IP allow must admit every port")
	}
	p.setAllowed(nil, true)
	if !localAdmitsAllPorts(self, peer) {
		t.Fatal("allow-all must admit a direct path")
	}
}
