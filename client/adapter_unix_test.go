//go:build freebsd || linux || openbsd

package client

import "testing"

func TestIPv4HostOnLink(t *testing.T) {
	if !ipv4HostOnLink("127.0.0.1/32") {
		t.Fatal("loopback /32 should be on-link")
	}
	if !ipv4HostOnLink("127.0.0.1") {
		t.Fatal("loopback host should be on-link")
	}
	if ipv4HostOnLink("0.0.0.0/0") {
		t.Fatal("default route is not an on-link host")
	}
	if ipv4HostOnLink("default") {
		t.Fatal("default keyword is not on-link")
	}
	if ipv4HostOnLink("1.2.3.4/32") {
		t.Fatal("public address should not be treated as on-link")
	}
}
