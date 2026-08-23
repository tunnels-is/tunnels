package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) recomputeTunnelView() {
	var shown []*client.TunnelMETA
	for _, t := range a.tunnels {
		if t == nil {
			continue
		}
		srv := a.serverByID(t.ServerID)
		srvTag, srvIP := "", ""
		if srv != nil {
			srvTag, srvIP = srv.Tag, srv.IP
		}
		if filterMatch(a.filterTunnels, t.Tag, t.IFName, srvTag, srvIP) {
			shown = append(shown, t)
		}
	}
	a.tunnelView = shown
}

func (a *App) tunnelsPage() fyne.CanvasObject {
	if !a.advanced {
		return emptyState("Enable Advanced mode in Settings to manage tunnels.")
	}
	a.fetchServers(false)
	a.recomputeTunnelView()

	filter := kSearch("Filter by tag, interface, server…", a.filterTunnels, func(s string) {
		a.filterTunnels = s
	}, func(s string) {
		a.filterTunnels = s
		a.reloadCurrent()
	})
	create := primaryBtn("Create", func() {
		go func() {
			_, err := client.CreateTunnel()
			a.uiDo(func() {
				if err != nil {
					a.fail(err.Error())
					return
				}
				a.refreshState()
				a.note("Tunnel created")
				a.reloadCurrent()
			})
		}()
	})
	head := pageHeader("Tunnels", "", kSearchBox(filter), hspace(8), create)

	a.tunnelList = widget.NewList(
		func() int { return len(a.tunnelView) },
		func() fyne.CanvasObject { return newKRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row, ok := obj.(*kRow)
			if !ok {
				return
			}
			a.bindTunnelRow(id, row)
		},
	)
	return listPage(head, a.tunnelList)
}

func (a *App) bindTunnelRow(id widget.ListItemID, row *kRow) {
	if id < 0 || id >= len(a.tunnelView) {
		return
	}
	t := a.tunnelView[id]
	if t == nil {
		return
	}
	at := a.activeByTag()[t.Tag]
	srv := a.serverByID(t.ServerID)
	srvLabel := "No server"
	if srv != nil {
		srvLabel = srv.Tag + "  ·  " + srv.IP
	}
	on := at != nil
	pill := "Idle"
	meta := srvLabel + "  ·  " + t.IFName
	if on {
		pill = "Connected"
		meta = "↓ " + at.IngressString() + "  ↑ " + at.EgressString() + "  ·  " + meta
	}
	row.SetTitleMeta(t.Tag, meta, on, pill)
	tun := t
	if on {
		live := at
		row.main.Set("Disconnect", kDanger, func() {
			a.confirm("Disconnect", "Disconnect "+tun.Tag+"?", func() { a.disconnectActive(live) })
		})
	} else {
		row.main.Set("Connect", kSuccess, func() {
			a.confirm("Connect", "Connect "+tun.Tag+"?", func() { a.connectTunnel(tun) })
		})
	}
	row.ghost.Set("Firewall", kGhost, func() {
		a.peersTag = tun.Tag
		a.show(pageTunnelPeers)
	})
	row.ghost.SetHidden(false)
	row.iconA.SetIconOnly(theme.DocumentCreateIcon(), kGhost, func() {
		a.editTag = tun.Tag
		a.show(pageTunnelEdit)
	})
	row.iconB.SetIconOnly(theme.DeleteIcon(), kDanger, func() {
		a.confirm("Delete", "Delete tunnel "+tun.Tag+"?", func() {
			if err := client.DeleteTunnel(tun.Tag); err != nil {
				a.fail(err.Error())
				return
			}
			a.refreshState()
			a.note("Tunnel deleted")
			a.reloadCurrent()
		})
	})
}
