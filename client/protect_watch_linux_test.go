//go:build linux

package client

import (
	"encoding/binary"
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

func TestRoutePrefixEqual(t *testing.T) {
	_, host := mustCIDR2("74.63.223.157/32")
	_, other := mustCIDR2("89.147.109.61/32")
	def := &netlink.Route{Gw: net.ParseIP("192.168.1.1"), LinkIndex: 2}
	if routePrefixEqual(def, host) {
		t.Fatal("a default route must not satisfy a host /32")
	}
	match := &netlink.Route{Dst: host, Gw: net.ParseIP("192.168.1.1"), LinkIndex: 2}
	if !routePrefixEqual(match, host) {
		t.Fatal("expected the host /32 to match")
	}
	if routePrefixEqual(match, other) {
		t.Fatal("a different host must not match")
	}
}

func TestIPv4UnicastIfIndexNetworkOrder(t *testing.T) {
	got := ipv4UnicastIfIndex(2)
	var b [4]byte
	binary.NativeEndian.PutUint32(b[:], uint32(got))
	if binary.BigEndian.Uint32(b[:]) != 2 {
		t.Fatalf("IP_UNICAST_IF value %#x is not network-order ifindex 2", got)
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
	_, n := mustCIDR2(s)
	return n
}

func mustCIDR2(s string) (net.IP, *net.IPNet) {
	ip, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return ip, n
}
