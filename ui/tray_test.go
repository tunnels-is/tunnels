package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/tunnels-is/tunnels/client"
)

func TestTrayMenuIdleDisablesDisconnect(t *testing.T) {
	a := &App{}
	menu := a.newTrayMenu()
	if len(menu.Items) != 2 {
		t.Fatalf("items: got %d, want Disconnect and Exit", len(menu.Items))
	}
	disc, exit := menu.Items[0], menu.Items[1]
	if disc.Label != trayDisconnectLabel || !disc.Disabled || disc.Action == nil {
		t.Fatalf("Disconnect: label %q disabled %v action %v", disc.Label, disc.Disabled, disc.Action != nil)
	}
	if exit.Label != trayExitLabel || !exit.IsQuit || exit.Action == nil {
		t.Fatalf("Exit: label %q quit %v action %v", exit.Label, exit.IsQuit, exit.Action != nil)
	}
	if menu.Items[len(menu.Items)-1] != exit {
		t.Fatal("Exit must stay last so Fyne does not append its own Quit")
	}
}

func TestTrayMenuEnablesDisconnectWhileLive(t *testing.T) {
	connected := &client.TUN{ID: "up"}
	connected.SetState(client.TunnelConnected)
	connecting := &client.TUN{ID: "soon"}
	connecting.SetState(client.TunnelConnecting)

	a := &App{active: []*client.TUN{connected, connecting}}
	menu := a.newTrayMenu()
	if menu.Items[0].Disabled {
		t.Fatal("Disconnect must be enabled while a tunnel is up or connecting")
	}
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

func TestSyncTrayTogglesDisconnectWithoutGrowing(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)

	a := &App{}
	a.trayMenu = a.newTrayMenu()
	up := &client.TUN{ID: "up"}
	up.SetState(client.TunnelConnected)
	a.active = []*client.TUN{up}
	a.syncTray()
	if a.trayDisconnect.Disabled {
		t.Fatal("Disconnect must enable when a tunnel is up")
	}
	if len(a.trayMenu.Items) != 2 {
		t.Fatalf("menu grew to %d items", len(a.trayMenu.Items))
	}

	a.active = nil
	a.syncTray()
	if !a.trayDisconnect.Disabled {
		t.Fatal("Disconnect must disable when nothing is up")
	}
	if len(a.trayMenu.Items) != 2 {
		t.Fatalf("menu grew to %d items", len(a.trayMenu.Items))
	}

	a.syncTray()
	if len(a.trayMenu.Items) != 2 {
		t.Fatalf("unchanged sync grew the menu to %d items", len(a.trayMenu.Items))
	}
}

func TestInstallTraySkipsWithoutDesktopSupport(t *testing.T) {
	fy := test.NewApp()
	t.Cleanup(fy.Quit)
	a := &App{fyneApp: fy, win: fy.NewWindow("t")}
	a.installTray()
	if a.trayMenu != nil || a.trayDisconnect != nil {
		t.Fatal("the test driver has no system tray")
	}
}

func TestDisconnectAllIdleReturns(t *testing.T) {
	a := &App{}
	a.disconnectAll()
}
