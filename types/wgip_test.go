package types

import (
	"net"
	"net/netip"
	"testing"
)

func TestIPv4Gateway_MasksThenIncrements(t *testing.T) {
	_, n, err := net.ParseCIDR("10.0.0.5/22")
	if err != nil {
		t.Fatal(err)
	}
	got := IPv4Gateway(n)
	if got.String() != "10.0.0.1" {
		t.Fatalf("gateway = %s, want 10.0.0.1", got)
	}

	_, n24, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if g := IPv4Gateway(n24).String(); g != "10.0.0.1" {
		t.Fatalf(" /24 gateway = %s, want 10.0.0.1", g)
	}
}

func TestIPv4Broadcast(t *testing.T) {
	_, n, err := net.ParseCIDR("10.9.9.0/30")
	if err != nil {
		t.Fatal(err)
	}
	if b := IPv4Broadcast(n).String(); b != "10.9.9.3" {
		t.Fatalf("broadcast = %s, want 10.9.9.3", b)
	}
	_, n22, err := net.ParseCIDR("10.0.0.0/22")
	if err != nil {
		t.Fatal(err)
	}
	if b := IPv4Broadcast(n22).String(); b != "10.0.3.255" {
		t.Fatalf(" /22 broadcast = %s, want 10.0.3.255", b)
	}
}

func TestIsReservedWireGuardIPv4(t *testing.T) {
	_, n, err := net.ParseCIDR("10.9.9.0/30")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]bool{
		"10.9.9.0": true,  // network
		"10.9.9.1": true,  // gateway
		"10.9.9.2": false, // only usable host
		"10.9.9.3": true,  // broadcast
		"8.8.8.8":  true,  // outside
	}
	for ip, want := range cases {
		if got := IsReservedWireGuardIPv4(n, net.ParseIP(ip)); got != want {
			t.Errorf("%s reserved=%v, want %v", ip, got, want)
		}
	}

	_, n22, err := net.ParseCIDR("10.0.0.0/22")
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"10.0.0.0", "10.0.0.1", "10.0.0.255", "10.0.1.0", "10.0.3.255"} {
		if !IsReservedWireGuardIPv4(n22, net.ParseIP(ip)) {
			t.Errorf("%s should be reserved in /22", ip)
		}
	}
	if IsReservedWireGuardIPv4(n22, net.ParseIP("10.0.0.2")) {
		t.Fatal("10.0.0.2 must be usable")
	}
}

func TestIsReservedWireGuardIPv6(t *testing.T) {
	p := netip.MustParsePrefix("fd00::/64")
	if !IsReservedWireGuardIPv6(p, netip.MustParseAddr("fd00::")) {
		t.Fatal("network reserved")
	}
	if !IsReservedWireGuardIPv6(p, netip.MustParseAddr("fd00::1")) {
		t.Fatal("gateway reserved")
	}
	if IsReservedWireGuardIPv6(p, netip.MustParseAddr("fd00::2")) {
		t.Fatal("::2 must be usable")
	}
}

func TestValidIfaceName(t *testing.T) {
	if !ValidIfaceName("eth0") || !ValidIfaceName("wg0") {
		t.Fatal("common names")
	}
	if ValidIfaceName("") || ValidIfaceName("+") || ValidIfaceName("eth+") {
		t.Fatal("invalid names must fail")
	}
}

func TestValidateDeviceWireGuardAddrs(t *testing.T) {
	if err := ValidateDeviceWireGuardAddrs("10.0.0.0/24", "", "10.0.0.1", ""); err == nil {
		t.Fatal("server IP must be rejected")
	}
	if err := ValidateDeviceWireGuardAddrs("10.0.0.0/24", "", "10.0.0.10", ""); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDeviceWireGuardAddrs("10.0.0.0/24", "fd00::/64", "10.0.0.10", "fd00::1"); err == nil {
		t.Fatal("server IPv6 must be rejected")
	}
}
