package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/tunnels-is/tunnels/types"
)

func TestParseBlockedPorts(t *testing.T) {
	ports, bad := parseBlockedPorts([]string{"25", "443", "25", "0", "65536", "nope", "80"})
	if len(bad) != 3 || bad[0] != "0" || bad[1] != "65536" || bad[2] != "nope" {
		t.Fatalf("bad = %#v", bad)
	}
	if len(ports) != 3 || ports[0] != 25 || ports[1] != 443 || ports[2] != 80 {
		t.Fatalf("ports = %#v", ports)
	}
}

func TestTunnelDNSRowsSkipsNil(t *testing.T) {
	a := test.NewApp()
	t.Cleanup(a.Quit)
	applyZoomTokens(1)
	setLiveTheme(themeTunnelsDark)

	if rows := tunnelDNSRows(nil, nil, nil); len(rows) != 1 {
		t.Fatalf("empty list: got %d rows, want the empty state", len(rows))
	}

	recs := []*types.DNSRecord{
		{Domain: "a.example", IP: []string{"10.0.0.1"}},
		nil,
		{Domain: "routed.example", Wildcard: true},
		{Domain: "b.example", Wildcard: true, TXT: []string{"hello"}},
	}
	rows := tunnelDNSRows(recs, nil, nil)
	if len(rows) != 2 {
		t.Fatalf("nil hole: got %d rows, want 2", len(rows))
	}
	routes := tunnelDNSRouteRows(recs, nil, nil)
	if len(routes) != 1 {
		t.Fatalf("routes: got %d rows, want 1", len(routes))
	}
	if len(tunnelDNSRouteRows(nil, nil, nil)) != 1 {
		t.Fatal("empty routes should show the empty state")
	}
}
