package client

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
)

// TestDirectPeersSnapshot ...
func TestDirectPeersSnapshot(t *testing.T) {
	if got := (*TUN)(nil).DirectPeers(); got != nil {
		t.Fatalf("nil tunnel returned %#v", got)
	}
	tun := &TUN{}
	if got := tun.DirectPeers(); got != nil {
		t.Fatalf("empty tunnel returned %#v", got)
	}

	first := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	second := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	tun.punchDirect = map[[4]byte]punchInstall{
		{10, 0, 1, 3}: {id: first, via: netip.MustParseAddrPort("192.0.2.10:51820")},
		{10, 0, 0, 4}: {id: second, via: netip.MustParseAddrPort("192.0.2.4:40000")},
	}
	got := tun.DirectPeers()
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].TunnelIP != "10.0.0.4" || got[0].DeviceID != second.String() || got[0].Endpoint != "192.0.2.4:40000" {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].TunnelIP != "10.0.1.3" || got[1].DeviceID != first.String() || got[1].Endpoint != "192.0.2.10:51820" {
		t.Fatalf("second = %+v", got[1])
	}
}
