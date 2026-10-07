package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/tunnels-is/tunnels/client"
)

func trayLabels(items []*fyne.MenuItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.IsSeparator {
			out = append(out, "---")
			continue
		}
		mark := ""
		if item.Disabled {
			mark = " (off)"
		}
		out = append(out, item.Label+mark)
	}
	return out
}

func liveTun(id, tag string, state client.TunnelState) *client.TUN {
	tun := &client.TUN{ID: id, CR: &client.ConnectionRequest{Tag: tag}}
	tun.SetState(state)
	return tun
}

func TestTrayMenuIdle(t *testing.T) {
	a := &App{}
	menu := a.newTrayMenu()
	got := trayLabels(menu.Items)
	want := []string{"Auto-connect", "Exit"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("menu: %v", got)
	}
	exit := menu.Items[len(menu.Items)-1]
	if !exit.IsQuit || exit.Action == nil || exit.Label != trayExitLabel {
		t.Fatalf("Exit: label %q quit %v", exit.Label, exit.IsQuit)
	}
	auto := menu.Items[len(menu.Items)-2]
	if auto.Label != trayAutoConnectLabel || auto.Disabled || auto.Action == nil {
		t.Fatalf("Auto-connect: label %q disabled %v", auto.Label, auto.Disabled)
	}
}

func TestTrayMenuOnlineRowDisconnectsAndDownRowConnects(t *testing.T) {
	home := liveTun("2", "home", client.TunnelConnected)
	work := liveTun("1", "work", client.TunnelConnecting)
	a := &App{
		active: []*client.TUN{work, home, liveTun("3", "extra", client.TunnelConnected)},
		tunnels: []*client.TunnelMeta{
			{Tag: "office", ServerID: "srv-office"},
			{Tag: "home", ServerID: "srv-home"},
			{Tag: "lab", ServerID: "srv-lab"},
			{Tag: "draft"},
		},
	}
	menu := a.newTrayMenu()
	got := trayLabels(menu.Items)
	want := []string{
		"draft",
		"extra - ONLINE",
		"home - ONLINE",
		"lab",
		"office",
		"work - ONLINE",
		"---",
		"Auto-connect",
		"Exit",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("menu:\n got %v\nwant %v", got, want)
	}
	for _, item := range menu.Items {
		if item.IsSeparator {
			continue
		}
		if item.Action == nil || item.Disabled {
			t.Fatalf("%q must be clickable", item.Label)
		}
	}
}

func TestSyncTrayRebuildsWhenTunnelsChange(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)

	a := &App{}
	a.trayMenu = a.newTrayMenu()
	firstExit := a.trayMenu.Items[len(a.trayMenu.Items)-1]
	a.syncTray()
	if a.trayMenu.Items[len(a.trayMenu.Items)-1] != firstExit {
		t.Fatal("unchanged state rebuilt the menu")
	}

	a.tunnels = []*client.TunnelMeta{{Tag: "office", ServerID: "srv"}}
	a.syncTray()
	got := trayLabels(a.trayMenu.Items)
	want := []string{"office", "---", "Auto-connect", "Exit"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("menu: %v", got)
	}
	if a.trayMenu.Items[len(a.trayMenu.Items)-1] == firstExit {
		t.Fatal("changed state kept the old Exit item")
	}
	if !a.trayMenu.Items[len(a.trayMenu.Items)-1].IsQuit {
		t.Fatal("Exit must stay the quit item after a rebuild")
	}
}

func TestInstallTraySkipsWithoutDesktopSupport(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)
	a := &App{fyneApp: fy, win: fy.NewWindow("t")}
	a.installTray()
	if a.trayMenu != nil {
		t.Fatal("the test driver has no system tray")
	}
}

func TestTrayAutoConnectIdleReturns(t *testing.T) {
	a := &App{}
	a.trayAutoConnect()
}

func TestDisconnectableTunnelsSkipsTeardown(t *testing.T) {
	up := &client.TUN{ID: "up"}
	up.SetState(client.TunnelConnected)
	going := &client.TUN{ID: "going"}
	going.SetState(client.TunnelDisconnecting)
	gone := &client.TUN{ID: "gone"}
	gone.SetState(client.TunnelDisconnected)

	got := disconnectableTunnels([]*client.TUN{nil, up, going, gone})
	if len(got) != 1 || got[0] != up {
		t.Fatalf("got %v, want the connected tunnel", got)
	}
	if len(disconnectableTunnels(nil)) != 0 {
		t.Fatal("nil active list must be idle")
	}
}
