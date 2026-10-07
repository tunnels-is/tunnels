package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/tunnels-is/tunnels/client"
)

const (
	trayDisconnectLabel = "Disconnect"
	trayExitLabel       = "Exit"
)

// installTray registers the status icon. Left click shows the window.
// Right click offers Disconnect and Exit. SetSystemTrayWindow also hides
// the window on close; surviveWindowClose runs after this and keeps that
// hide, including when the display itself goes away.
//
// The title-bar minimize button stays with the compositor. On Wayland,
// xdg-shell does not report iconify, so that click cannot hide to the tray.
func (a *App) installTray() {
	if a.fyneApp == nil || a.win == nil {
		return
	}
	desk, ok := a.fyneApp.(desktop.App)
	if !ok {
		return
	}
	ensureTrayTitle(a.fyneApp)
	a.trayMenu = a.newTrayMenu()
	desk.SetSystemTrayMenu(a.trayMenu)
	if icon := a.fyneApp.Icon(); icon != nil {
		desk.SetSystemTrayIcon(icon)
	}
	desk.SetSystemTrayWindow(a.win)
}

// ensureTrayTitle gives the status icon a name. A plain go build does not
// embed FyneApp.toml, so Fyne would otherwise label the icon with the app id.
func ensureTrayTitle(fy fyne.App) {
	meta := fy.Metadata()
	if meta.Name != "" {
		return
	}
	meta.Name = appName
	if meta.ID == "" {
		meta.ID = appID
	}
	app.SetMetadata(meta)
}

// newTrayMenu builds Disconnect, then Exit. Exit is last and marked quit so
// Fyne does not append a second Quit item that skips client shutdown.
func (a *App) newTrayMenu() *fyne.Menu {
	disconnect := fyne.NewMenuItem(trayDisconnectLabel, a.disconnectAll)
	disconnect.Disabled = len(disconnectableTunnels(a.active)) == 0
	a.trayDisconnect = disconnect

	exit := fyne.NewMenuItem(trayExitLabel, a.shutdown)
	exit.IsQuit = true
	return fyne.NewMenu("", disconnect, exit)
}

// syncTray enables Disconnect while a tunnel can be torn down. The title
// pump calls this about once a second, so the menu is rebuilt only when
// that flag changes.
func (a *App) syncTray() {
	item := a.trayDisconnect
	if item == nil {
		return
	}
	disabled := len(disconnectableTunnels(a.active)) == 0
	if item.Disabled == disabled {
		return
	}
	item.Disabled = disabled
	if a.trayMenu != nil {
		a.trayMenu.Refresh()
	}
}

// disconnectableTunnels is every live tunnel the tray can still tear down.
// Tunnels already disconnecting are left out so the item can go idle.
func disconnectableTunnels(active []*client.TUN) []*client.TUN {
	var out []*client.TUN
	for _, tun := range active {
		if tun == nil {
			continue
		}
		switch tun.GetState() {
		case client.TunnelDisconnecting, client.TunnelDisconnected:
			continue
		default:
			out = append(out, tun)
		}
	}
	return out
}
