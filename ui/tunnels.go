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
		srvTag, addr := "", ""
		if srv != nil {
			srvTag, addr = srv.Tag, serverWGAddr(srv)
		}
		if filterMatch(a.filterTunnels, t.Tag, t.IFName, srvTag, addr) {
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

	spec := tunnelTable()

	if len(a.tunnelView) == 0 {
		msg, desc := "No tunnels", "Nothing matched this filter."
		if a.filterTunnels == "" {
			msg, desc = "No tunnels yet", "Create a tunnel to configure routes, DNS and a firewall."
		}
		return pageShellFlush("Tunnels", sub, actions, emptyState(msg, desc))
	}

	a.tunnelList = newRowList(spec,
		func() int { return len(a.tunnelView) },
		a.bindTunnelRow,
	)
	return pageShellFlush("Tunnels", sub, actions, tableBody(spec, a.tunnelList))
}

func tunnelTable() *tableSpec {
	return &tableSpec{
		actionW: 230,
		cols: []tableCol{
			{label: "TUNNEL", weight: 1.6, strong: true},
			{label: "SERVER", weight: 1.6},
			{label: "ADDRESS", weight: 2, mono: true},
			{label: "INTERFACE", weight: 1.4, mono: true, optional: true},
			{label: "TRANSFER", weight: 1.6, mono: true, optional: true},
		},
	}
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
	addr := "—"
	if srv != nil {
		srvLabel = srv.Tag
		addr = serverWGAddr(srv)
	}
	on := at != nil
	tn := toneNeutral
	transfer := "—"
	if on {
		tn = toneSuccess
		transfer = "↓ " + at.IngressString() + "  ↑ " + at.EgressString()
	}
	row.SetCells([]string{t.Tag, srvLabel, addr, t.IFName, transfer}, on, tn)

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
	row.iconB.SetIconOnly(theme.DeleteIcon(), kDanger, func() {
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
