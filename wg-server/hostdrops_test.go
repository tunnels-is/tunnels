package wgserver

import (
	"net"
	"slices"
	"testing"
)

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func attached(cidr, link string) attachedPrefix {
	return attachedPrefix{IPNet: mustCIDR(cidr), LinkName: link}
}

func TestDropCIDRsFromAttached_SkipsOverlayAndDefault(t *testing.T) {
	v4, v6 := dropCIDRsFromAttached(
		[]attachedPrefix{
			attached("0.0.0.0/0", "eth0"),
			attached("10.0.1.5/24", "eth0"),
			attached("172.17.0.1/16", "docker0"),
			attached("10.201.0.1/16", "wg0"),
			attached("10.202.0.1/16", "wg0mesh"),
			attached("127.0.0.1/8", "lo"),
			attached("::/0", "eth0"),
			attached("2001:db8::1/64", "eth0"),
			attached("fd00::1/64", "wg0"),
			attached("fe80::1/64", "eth0"),
		},
		[]string{"wg0", "wg0mesh"},
		[]*net.IPNet{mustCIDR("10.201.0.0/16"), mustCIDR("fd00::/64")},
	)
	for _, want := range []string{"10.0.1.0/24", "172.17.0.0/16", "127.0.0.0/8"} {
		if !slices.Contains(v4, want) {
			t.Errorf("v4 missing %s: %v", want, v4)
		}
	}
	for _, not := range []string{"0.0.0.0/0", "10.201.0.0/16", "10.202.0.0/16"} {
		if slices.Contains(v4, not) {
			t.Errorf("v4 should not contain overlay/default %s: %v", not, v4)
		}
	}
	if !slices.Contains(v6, "2001:db8::/64") {
		t.Errorf("v6 missing LAN prefix: %v", v6)
	}
	if slices.Contains(v6, "fd00::/64") {
		t.Errorf("v6 leaked overlay: %v", v6)
	}
	if !slices.Contains(v6, "fe80::/64") {
		t.Errorf("v6 missing link-local: %v", v6)
	}
}

func TestDropCIDRsFromAttached_EmptySkipKeepsOverlay(t *testing.T) {
	v4, _ := dropCIDRsFromAttached(
		[]attachedPrefix{attached("10.201.0.1/16", "wg0")},
		nil,
		nil,
	)
	if !slices.Contains(v4, "10.201.0.0/16") {
		t.Fatalf("without skip, overlay addr is a host prefix: %v", v4)
	}
}

func TestMergeCIDRs_DropsMoreSpecificAndDefault(t *testing.T) {
	got := mergeCIDRs(
		[]string{"169.254.0.0/16", "10.0.1.0/24"},
		[]string{"169.254.169.254/32", "10.0.1.5/32", "0.0.0.0/0", "not-a-cidr", "10.0.1.0/24"},
	)
	if slices.Contains(got, "169.254.169.254/32") || slices.Contains(got, "10.0.1.5/32") {
		t.Fatalf("specific prefixes should be covered: %v", got)
	}
	if slices.Contains(got, "0.0.0.0/0") {
		t.Fatalf("default route must not be a drop: %v", got)
	}
	for _, want := range []string{"169.254.0.0/16", "10.0.1.0/24"} {
		if !slices.Contains(got, want) {
			t.Fatalf("missing %s: %v", want, got)
		}
	}
}

func TestPrefixInOverlay_MoreSpecificInside(t *testing.T) {
	overlay := []*net.IPNet{mustCIDR("10.201.0.0/16")}
	if !prefixInOverlay(mustCIDR("10.201.4.0/24"), overlay) {
		t.Fatal("inner overlay prefix should be skipped")
	}
	if prefixInOverlay(mustCIDR("10.0.1.0/24"), overlay) {
		t.Fatal("unrelated LAN should not be overlay")
	}
	if prefixInOverlay(mustCIDR("10.0.0.0/8"), overlay) {
		t.Fatal("wider net must not be treated as overlay")
	}
}

func TestEgressDropCIDRs_AlwaysHasRFCAndFabric(t *testing.T) {
	cfg := &Config{
		WireGuardSubnet: "10.201.0.0/16",
		WireGuardIface:  "wg0-does-not-exist",
		InternetIface:   "eth0",
	}
	v4, v6, err := egressDropCIDRs(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"169.254.0.0/16", "127.0.0.0/8"} {
		if !slices.Contains(v4, want) {
			t.Errorf("v4 missing %s: %v", want, v4)
		}
	}
	if !cidrsCover(v4, "168.63.129.16") || !cidrsCover(v4, "100.100.100.200") {
		t.Errorf("v4 fabric not covered: %v", v4)
	}
	if !slices.Contains(v6, "fe80::/10") && !cidrsCover(v6, "fe80::a9fe:a9fe") {
		t.Errorf("v6 missing link-local: %v", v6)
	}
	if !cidrsCover(v6, "::1") || !cidrsCover(v6, "fd00:ec2::254") {
		t.Errorf("v6 fabric/loopback not covered: %v", v6)
	}
}

func cidrsCover(cidrs []string, ip string) bool {
	p := net.ParseIP(ip)
	if p == nil {
		return false
	}
	for _, s := range cidrs {
		_, n, err := net.ParseCIDR(s)
		if err == nil && n.Contains(p) {
			return true
		}
	}
	return false
}

func TestCollectHostDropCIDRs_IncludesLoopbackExcludesDefault(t *testing.T) {
	v4, _, err := collectHostDropCIDRs(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	hasLoopback := false
	for _, s := range v4 {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			t.Fatal(err)
		}
		if n.Contains(net.ParseIP("127.0.0.1")) {
			hasLoopback = true
		}
		if s == "0.0.0.0/0" {
			t.Fatalf("default route leaked into drops: %v", v4)
		}
	}
	if !hasLoopback {
		t.Fatalf("expected a loopback prefix in host drops, got %v", v4)
	}
}
