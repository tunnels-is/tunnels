package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/types"
)

func (a *App) recomputeServerView() {
	var shown []types.Server
	for _, s := range a.servers {
		if filterMatch(a.filterServers, s.Tag, s.IP, countryName(s.Country), s.Country) {
			shown = append(shown, s)
		}
	}
	a.serverView = shown
}

func (a *App) serversPage() fyne.CanvasObject {
	if a.loggedIn() && !a.serversLoaded {
		a.fetchServers(false)
	}
	a.recomputeServerView()

	_, search := searchField("Filter servers", a.filterServers, func(s string) {
		a.filterServers = s
	}, func(s string) {
		a.filterServers = s
		a.reloadCurrent()
	})
	refresh := newIconBtn(theme.ViewRefreshIcon(), kOutline, func() {
		a.note("Refreshing servers…")
		a.fetchServers(true)
	})

	active := 0
	am := a.activeByServer()
	for _, s := range a.serverView {
		if am[s.ID.String()] != nil {
			active++
		}
	}

	sub := fmt.Sprintf("%d available", len(a.serverView))
	switch {
	case a.serversFetching && len(a.serverView) == 0:
		sub = "Loading…"
	case active == 1:
		sub = fmt.Sprintf("%d available · 1 connected", len(a.serverView))
	case active > 1:
		sub = fmt.Sprintf("%d available · %d connected", len(a.serverView), active)
	}

	spec := serverTable()

	if len(a.serverView) == 0 {
		msg, desc := "No servers", "Nothing matched this filter."
		if a.filterServers == "" {
			msg, desc = "No servers available", "Your account has no servers assigned yet."
			if a.serversFetching {
				msg, desc = "Loading servers…", ""
			}
		}
		return pageShellFlush("Servers", sub, hstackFlex(sp2, 0, search, refresh), emptyState(msg, desc))
	}

	a.serverList = newRowList(spec,
		func() int { return len(a.serverView) },
		a.bindServerRow,
	)

	return pageShellFlush("Servers", sub, hstackFlex(sp2, 0, search, refresh),
		tableBody(spec, a.serverList))
}

func serverTable() *tableSpec {
	return &tableSpec{
		actionW: 120,
		cols: []tableCol{
			{label: "SERVER", weight: 2, strong: true},
			{label: "LOCATION", weight: 1.6},
			{label: "ADDRESS", weight: 2, mono: true},
			{label: "TRANSFER", weight: 1.8, mono: true, optional: true},
			{label: "STATUS", weight: 1.2, badge: true},
		},
	}
}

func (a *App) bindServerRow(id widget.ListItemID, row *kRow) {
	if id < 0 || id >= len(a.serverView) {
		return
	}
	s := a.serverView[id]
	at := a.activeByServer()[s.ID.String()]
	on := at != nil

	status, t := "Idle", toneNeutral
	transfer := "—"
	if on {
		status, t = "Connected", toneSuccess
		transfer = "↓ " + at.IngressString() + "  ↑ " + at.EgressString()
	}
	row.SetCells([]string{
		s.Tag,
		countryName(s.Country),
		s.IP + ":" + s.Port,
		transfer,
		status,
	}, on, t)

	row.ghost.SetHidden(true)
	row.iconA.SetHidden(true)
	row.iconB.SetHidden(true)
	if on {
		tun, tag := at, s.Tag
		row.main.Set("Disconnect", kDanger, func() {
			a.confirm("Disconnect", "Disconnect from "+tag+"?", func() { a.disconnectActive(tun) })
		})
	} else {
		srv := s
		row.main.Set("Connect", kSuccess, func() {
			a.confirm("Connect", "Connect to "+srv.Tag+"?", func() { a.connectToServer(srv) })
		})
	}
}
