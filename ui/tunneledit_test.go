package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/tunnels-is/tunnels/types"
)

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
		{Domain: "b.example", Wildcard: true, TXT: []string{"hello"}},
	}
	rows := tunnelDNSRows(recs, nil, nil)
	if len(rows) != 2 {
		t.Fatalf("nil hole: got %d rows, want 2", len(rows))
	}
}
