package ui

import (
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

const (
	trayAutoConnectLabel = "Auto-connect"
	trayOnlineSuffix     = " - ONLINE"
	trayExitLabel        = "Exit"
)

// installTray registers the status icon. Left click shows the window.
// Right click lists each tunnel, then Auto-connect and Exit.
// SetSystemTrayWindow also hides the window on close; surviveWindowClose
// runs after this and keeps that hide, including when the display itself
// goes away.
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

// newTrayMenu builds the status icon menu. Exit is last and marked quit so
// Fyne does not append a second Quit item that skips client shutdown.
func (a *App) newTrayMenu() *fyne.Menu {
	items, sig := a.trayItems()
	a.traySig = sig
	return fyne.NewMenu("", items...)
}

// syncTray rebuilds the menu when a tunnel comes up, goes down, or the
// saved list changes. The title pump calls this about once a second, so
// an unchanged signature leaves the open menu alone.
func (a *App) syncTray() {
	if a.trayMenu == nil {
		return
	}
	items, sig := a.trayItems()
	if sig == a.traySig {
		return
	}
	a.traySig = sig
	a.trayMenu.Items = items
	a.trayMenu.Refresh()
}

// trayItems is the right-click menu.
// Each saved tunnel is one row. A live tunnel reads "name - ONLINE" and a
// click disconnects it. A tunnel that is down shows its name and a click
// connects it. Auto-connect sits where Disconnect used to, just above Exit,
// and always brings up the default tunnel.
func (a *App) trayItems() ([]*fyne.MenuItem, string) {
	rows := a.trayTunnelRows()
	var items []*fyne.MenuItem
	var sig strings.Builder
	for _, row := range rows {
		label := trayTunnelLabel(row.tag, row.online)
		tag := row.tag
		items = append(items, fyne.NewMenuItem(label, func() { a.trayToggleTunnel(tag) }))
		sig.WriteString("t\t")
		sig.WriteString(label)
		sig.WriteByte('\n')
	}
	if len(rows) > 0 {
		items = append(items, fyne.NewMenuItemSeparator())
		sig.WriteString("sep\n")
	}

	items = append(items, fyne.NewMenuItem(trayAutoConnectLabel, a.trayAutoConnect))
	sig.WriteString("auto\n")

	exit := fyne.NewMenuItem(trayExitLabel, a.shutdown)
	exit.IsQuit = true
	items = append(items, exit)
	sig.WriteString("exit\n")
	return items, sig.String()
}

type trayTunnelRow struct {
	tag    string
	online bool
}

// trayTunnelRows is every saved tunnel, plus any live tunnel that has no
// saved row yet, so it can still be disconnected. Names are alphabetical.
func (a *App) trayTunnelRows() []trayTunnelRow {
	live := map[string]*client.TUN{}
	for _, tun := range disconnectableTunnels(a.active) {
		tag := tunnelDisconnectTag(tun)
		if tag == "" {
			tag = a.tunnelLabel(tun)
		}
		if tag == "" || tag == "tunnel" {
			continue
		}
		live[tag] = tun
	}

	seen := map[string]struct{}{}
	var rows []trayTunnelRow
	for _, meta := range a.tunnels {
		if meta == nil || meta.Tag == "" {
			continue
		}
		if _, ok := seen[meta.Tag]; ok {
			continue
		}
		seen[meta.Tag] = struct{}{}
		_, online := live[meta.Tag]
		rows = append(rows, trayTunnelRow{tag: meta.Tag, online: online})
	}
	for tag := range live {
		if _, ok := seen[tag]; ok {
			continue
		}
		rows = append(rows, trayTunnelRow{tag: tag, online: true})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].tag < rows[j].tag
	})
	return rows
}

func trayTunnelLabel(tag string, online bool) string {
	if online {
		return tag + trayOnlineSuffix
	}
	return tag
}

// trayToggleTunnel disconnects a live tunnel and connects one that is down.
func (a *App) trayToggleTunnel(tag string) {
	for _, tun := range disconnectableTunnels(a.active) {
		got := tunnelDisconnectTag(tun)
		if got == "" {
			got = a.tunnelLabel(tun)
		}
		if got == tag {
			a.disconnectActive(tun)
			return
		}
	}
	a.connectTunnelByTag(tag)
}

// trayAutoConnect connects the way the dashboard Connect button does.
// A probe that already ran is reused. Otherwise the server list is measured
// and the closest one is passed to connectToServer.
func (a *App) trayAutoConnect() {
	if srv, ok := a.closestServer(); ok {
		a.connectToServer(srv)
		return
	}
	if a.user == nil || a.user.DeviceToken == nil {
		a.fail("You are not logged in")
		if a.win != nil {
			a.win.Show()
		}
		a.show(pageLogin)
		return
	}
	a.note("Finding closest server...")
	known := append([]types.Server(nil), a.servers...)
	go func() {
		list := known
		var fetchErr error
		if len(list) == 0 {
			list, fetchErr = a.fetchServerList()
		}
		var probed []client.ServerProbe
		if fetchErr == nil && len(list) > 0 {
			probed = client.ProbeServers(list)
		}
		a.uiDo(func() {
			if fetchErr != nil {
				a.fail(fetchErr.Error())
				return
			}
			if len(list) == 0 {
				a.fail("Unable to find servers")
				return
			}
			if len(known) == 0 {
				a.servers = list
				a.serversLoaded = true
			}
			a.probeResults = probed
			a.probeAt = time.Now()
			a.probedOnce = true
			srv, ok := a.closestServer()
			if !ok {
				a.fail("No server answered a ping.")
				return
			}
			a.connectToServer(srv)
		})
	}()
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
