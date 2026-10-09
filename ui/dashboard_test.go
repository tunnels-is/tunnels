package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func TestStageCentersShortContent(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)
	applyZoomTokens(1)

	child := vspace(z(40))
	box := fyne.NewContainerWithLayout(&stageLayout{w: z(200)}, child)
	box.Resize(fyne.NewSize(z(800), z(400)))

	if child.Position().Y < z(160) || child.Position().Y > z(200) {
		t.Fatalf("vertical centre y=%v", child.Position().Y)
	}
	if child.Position().X < z(280) || child.Position().X > z(320) {
		t.Fatalf("horizontal centre x=%v", child.Position().X)
	}
	if child.Size().Width < z(190) || child.Size().Width > z(210) {
		t.Fatalf("column width=%v", child.Size().Width)
	}
}

func TestStageTopsTallContent(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)
	applyZoomTokens(1)

	child := vspace(z(500))
	box := fyne.NewContainerWithLayout(&stageLayout{w: z(200)}, child)
	box.Resize(fyne.NewSize(z(800), z(200)))
	if child.Position().Y != 0 {
		t.Fatalf("tall content y=%v, want 0", child.Position().Y)
	}
}

func TestDashboardPageBuilds(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)
	applyZoomTokens(1)

	id := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	base := App{
		serversLoaded: true,
		servers: []types.Server{{
			ID: id, Tag: "reykjavik", Country: "IS", IP: "1.2.3.4", Port: "443",
		}},
	}

	empty := base
	if empty.dashboardPage() == nil {
		t.Fatal("empty dashboard")
	}

	probing := base
	probing.probing = true
	if probing.dashboardPage() == nil {
		t.Fatal("probing dashboard")
	}

	found := base
	found.probedOnce = true
	found.probeResults = []client.ServerProbe{{
		Tag: "reykjavik", IP: "1.2.3.4", Country: "IS", ServerID: id.String(),
		Latency: 18 * time.Millisecond, OK: true,
	}}
	if found.dashboardPage() == nil {
		t.Fatal("closest dashboard")
	}
	if got := dashNameSize(); got < fsTitle+8 {
		t.Fatalf("dashboard name size %v, title is %v", got, fsTitle)
	}

	live := base
	live.bwRange = 60
	live.user = &client.User{ID: "user-1"}
	live.active = []*client.TUN{{
		CR: &client.ConnectionRequest{Tag: "home", UserID: "user-1", ServerID: id.String()},
	}}
	if live.dashboardPage() == nil {
		t.Fatal("live dashboard")
	}
}
