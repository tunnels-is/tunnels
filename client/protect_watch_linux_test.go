//go:build linux

package client

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestPickPhysicalIPv4Default_SkipsTunnelAndBlackhole(t *testing.T) {
	tunIdx, wifiIdx := 8, 2
	routes := []netlink.Route{
		{LinkIndex: tunIdx, Gw: net.ParseIP("10.0.0.13"), Priority: 0},
		{Type: unix.RTN_BLACKHOLE, Priority: 50, Dst: mustCIDR("0.0.0.0/0")},
		{LinkIndex: wifiIdx, Gw: net.ParseIP("192.168.2.1"), Priority: 600},
	}
	got := pickPhysicalIPv4Default(routes, map[int]struct{}{tunIdx: {}}, map[string]struct{}{"10.0.0.13": {}})
	if got == nil {
		t.Fatal("expected wifi default")
	}
	if got.LinkIndex != wifiIdx || got.Gw.String() != "192.168.2.1" {
		t.Fatalf("got link %d gw %v", got.LinkIndex, got.Gw)
	}
}

func TestPickPhysicalIPv4Default_EmptyWhenOnlyTunnel(t *testing.T) {
	routes := []netlink.Route{
		{LinkIndex: 8, Gw: net.ParseIP("10.0.0.13"), Priority: 0},
	}
	got := pickPhysicalIPv4Default(routes, map[int]struct{}{8: {}}, map[string]struct{}{"10.0.0.13": {}})
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestIPv4RouteMatches(t *testing.T) {
	gw := net.ParseIP("192.168.2.1").To4()
	r := &netlink.Route{LinkIndex: 2, Gw: gw}
	if !ipv4RouteMatches(r, 2, gw) {
		t.Fatal("expected match")
	}
	if ipv4RouteMatches(r, 3, gw) {
		t.Fatal("wrong ifindex must not match")
	}
	if ipv4RouteMatches(r, 2, net.ParseIP("192.168.2.2").To4()) {
		t.Fatal("wrong gw must not match")
	}
}

func TestHostProtectRouteRequired_SkipsOnLink(t *testing.T) {
	if hostProtectRouteRequired("127.0.0.1") {
		t.Fatal("loopback is on-link; watcher must not demand a /32")
	}
	if !hostProtectRouteRequired("1.2.3.4") {
		t.Fatal("public address should still get a pinned /32")
	}
	if hostProtectRouteRequired("") {
		t.Fatal("empty host is not a required route")
	}
}

func TestIsIPv4DefaultRoute(t *testing.T) {
	if !isIPv4DefaultRoute(&netlink.Route{}) {
		t.Fatal("nil Dst should be default")
	}
	if !isIPv4DefaultRoute(&netlink.Route{Dst: mustCIDR("0.0.0.0/0")}) {
		t.Fatal("0.0.0.0/0 should be default")
	}
	if isIPv4DefaultRoute(&netlink.Route{Dst: mustCIDR("74.63.223.157/32")}) {
		t.Fatal("/32 must not be treated as default")
	}
}

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}
