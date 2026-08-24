package ui

import (
	"fmt"

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
		return pageShell("Tunnels", "", nil,
			emptyState("Advanced mode required", "Turn on Advanced mode in Settings to manage tunnels."))
	}
	a.fetchServers(false)
	a.recomputeTunnelView()

	_, search := searchField("Filter tunnels", a.filterTunnels, func(s string) {
		a.filterTunnels = s
	}, func(s string) {
		a.filterTunnels = s
		a.reloadCurrent()
	})
	create := primaryBtn("New tunnel", func() {
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
	}).withIcon(theme.ContentAddIcon())
	actions := hstackFlex(sp2, 0, search, create)

	live := 0
	byTag := a.activeByTag()
	for _, t := range a.tunnelView {
		if byTag[t.Tag] != nil {
			live++
		}
	}
	sub := fmt.Sprintf("%d configured", len(a.tunnelView))
	if live > 0 {
		sub = fmt.Sprintf("%d configured · %d up", len(a.tunnelView), live)
	}

	if len(a.tunnelView) == 0 {
		msg, desc := "No tunnels", "Nothing matched this filter."
		if a.filterTunnels == "" {
			msg, desc = "No tunnels yet", "Create a tunnel to configure routes, DNS and a firewall."
		}
		return pageShell("Tunnels", sub, actions, emptyState(msg, desc))
	}

	a.tunnelList = newRowList(
		func() int { return len(a.tunnelView) },
		a.bindTunnelRow,
	)
	return pageShell("Tunnels", sub, actions, listBody(a.tunnelList))
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
	srvLabel := "no server"
	if srv != nil {
		srvLabel = srv.Tag + "  ·  " + srv.IP
	}
	on := at != nil
	pill, tn := "", toneNeutral
	meta := srvLabel + "  ·  " + t.IFName
	if on {
		pill, tn = "Up", toneSuccess
		meta = "↓ " + at.IngressString() + "   ↑ " + at.EgressString() + "  ·  " + meta
	}
	row.SetRow(t.Tag, meta, on, pill, tn)

	tun := t
	if on {
		liveTun := at
		row.main.Set("Disconnect", kDanger, func() {
			a.confirm("Disconnect", "Disconnect "+tun.Tag+"?", func() { a.disconnectActive(liveTun) })
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
	row.iconB.SetIconOnly(theme.DeleteIcon(), kGhost, func() {
		a.confirm("Delete tunnel", "Delete tunnel "+tun.Tag+"?", func() {
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
