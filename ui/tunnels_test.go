package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func TestTunnelView_FilterByServerTag(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := &App{
		filterTunnels: "iceland",
		servers: []types.Server{{
			ID: id, Tag: "iceland", IP: "1.2.3.4", WireGuardPort: 51820,
		}},
		tunnels: []*client.TunnelMeta{
			{Tag: "home", ServerID: id.String(), IFName: "tun0"},
			{Tag: "work", ServerID: "other", IFName: "tun1"},
		},
	}
	a.recomputeTunnelView()
	if len(a.tunnelView) != 1 || a.tunnelView[0].Tag != "home" {
		t.Fatalf("got %#v", a.tunnelView)
	}
}

func TestTunnelsPage_BuildsCards(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)
	applyZoomTokens(1)

	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	a := &App{
		advanced:      true,
		serversLoaded: true,
		user:          &client.User{ID: "user-1"},
		servers: []types.Server{{
			ID: id, Tag: "iceland", IP: "1.2.3.4", WireGuardPort: 51820,
		}},
		tunnels: []*client.TunnelMeta{
			{Tag: "home", ServerID: id.String(), IFName: "tun0"},
			{Tag: "work", IFName: "tun1"},
		},
		active: []*client.TUN{{
			CR: &client.ConnectionRequest{Tag: "home", UserID: "user-1"},
		}},
	}
	page := a.tunnelsPage()
	if page == nil {
		t.Fatal("nil page")
	}
	if len(a.tunnelView) != 2 {
		t.Fatalf("view = %d", len(a.tunnelView))
	}
	if a.liveByTag["home"] == nil || a.liveByTag["work"] != nil {
		t.Fatalf("live = %#v", a.liveByTag)
	}
}
