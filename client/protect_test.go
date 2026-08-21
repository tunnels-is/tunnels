package client

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewProtectBindWrapsWhenIfIndexSet(t *testing.T) {
	plain := newProtectBind(0)
	if _, ok := plain.(*ifaceProtectBind); ok {
		t.Fatal("ifIndex 0 should not wrap the default bind")
	}
	wrapped := newProtectBind(7)
	b, ok := wrapped.(*ifaceProtectBind)
	if !ok {
		t.Fatal("ifIndex > 0 should wrap the bind")
	}
	if b.ifIndex != 7 {
		t.Fatalf("ifIndex = %d, want 7", b.ifIndex)
	}
}

func TestParseRouteGetGateway(t *testing.T) {
	out := `
   route to: default
destination: default
       mask: default
    gateway: 192.168.2.1
  interface: en0
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING,IFSCOPE,IFREF>
 recvpipe  sendpipe  ssthresh  rtt,msec    rttvar  hopcount      mtu     expire
       0         0         0         0         0         0      1500         0
`
	got := parseRouteGetGateway(out)
	if got == nil || got.String() != "192.168.2.1" {
		t.Fatalf("got %v", got)
	}
	if parseRouteGetGateway("gateway: fe80::1") != nil {
		t.Fatal("IPv6 gateway should be ignored")
	}
}

func TestRegisterProtectHostDedupes(t *testing.T) {
	tun := &TUN{}
	registerProtectHost(tun, "74.63.223.157")
	registerProtectHost(tun, "74.63.223.157")
	registerProtectHost(tun, "not-an-ip")
	registerProtectHost(tun, "2001:db8::1")
	if len(tun.protectHosts) != 1 || tun.protectHosts[0] != "74.63.223.157" {
		t.Fatalf("protectHosts = %v", tun.protectHosts)
	}
}

func TestBuildWGIPCIncludesFwMarkReplacePeersAndEndpoint(t *testing.T) {
	s := buildWGIPC("aa", "bb", "74.63.223.157", "51820")
	wantMark := fmt.Sprintf("fwmark=%d", wgProtectMark)
	if !strings.Contains(s, wantMark) {
		t.Fatalf("fwmark missing from UAPI config:\n%s", s)
	}
	if !strings.Contains(s, "replace_peers=true") {
		t.Fatalf("replace_peers missing from UAPI config (needed for sticky in-place replace):\n%s", s)
	}
	if !strings.Contains(s, "replace_allowed_ips=true") {
		t.Fatalf("replace_allowed_ips missing from UAPI config:\n%s", s)
	}
	if !strings.Contains(s, "endpoint=74.63.223.157:51820") {
		t.Fatalf("endpoint missing from UAPI config:\n%s", s)
	}
	if !strings.Contains(s, "allowed_ip=0.0.0.0/0") {
		t.Fatalf("allowed_ip missing from UAPI config:\n%s", s)
	}
}
