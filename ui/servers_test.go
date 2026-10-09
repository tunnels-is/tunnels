package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func TestServerView_FilterByCountry(t *testing.T) {
	a := &App{
		filterServers: "iceland",
		servers: []types.Server{
			{ID: uuid.New(), Tag: "reykjavik", Country: "IS", IP: "1.2.3.4", WireGuardPort: 51820},
			{ID: uuid.New(), Tag: "london", Country: "GB", IP: "5.6.7.8", WireGuardPort: 51820},
		},
	}
	a.recomputeServerView()
	if len(a.serverView) != 1 || a.serverView[0].Tag != "reykjavik" {
		t.Fatalf("got %#v", a.serverView)
	}
}

func TestServersPage_BuildsCards(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)
	applyZoomTokens(1)

	id := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	a := &App{
		serversLoaded: true,
		user:          &client.User{ID: "user-1"},
		servers: []types.Server{
			{ID: id, Tag: "reykjavik", Country: "IS", IP: "1.2.3.4", WireGuardPort: 51820},
			{ID: uuid.New(), Tag: "london", Country: "GB", IP: "5.6.7.8", WireGuardPort: 51820},
		},
		active: []*client.TUN{{
			CR: &client.ConnectionRequest{ServerID: id.String(), UserID: "user-1"},
		}},
	}
	page := a.serversPage()
	if page == nil {
		t.Fatal("nil page")
	}
	if len(a.serverView) != 2 {
		t.Fatalf("view = %d", len(a.serverView))
	}
	if a.liveByServer[id.String()] == nil {
		t.Fatal("reykjavik should be connected")
	}
}
