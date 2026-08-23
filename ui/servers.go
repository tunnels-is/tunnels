package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
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

	filter := kSearch("Filter by tag, IP, country…", a.filterServers, func(s string) {
		a.filterServers = s
	}, func(s string) {
		a.filterServers = s
		a.reloadCurrent()
	})
	n := fmt.Sprintf("%d", len(a.serverView))
	active := 0
	am := a.activeByServer()
	for _, s := range a.serverView {
		if am[s.ID.String()] != nil {
			active++
		}
	}
	if active > 0 {
		n = fmt.Sprintf("%d  ·  %d connected", len(a.serverView), active)
	}
	head := pageHeader("Servers", n, kSearchBox(filter))

	a.serverList = widget.NewList(
		func() int { return len(a.serverView) },
		func() fyne.CanvasObject { return newKRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row, ok := obj.(*kRow)
			if !ok {
				return
			}
			a.bindServerRow(id, row)
		},
	)

	return listPage(head, a.serverList)
}

func (a *App) bindServerRow(id widget.ListItemID, row *kRow) {
	if id < 0 || id >= len(a.serverView) {
		return
	}
	s := a.serverView[id]
	at := a.activeByServer()[s.ID.String()]
	on := at != nil
	pill := "Idle"
	if on {
		pill = "Connected"
	}
	row.SetTitleMeta(s.Tag, fmt.Sprintf("%s  ·  %s:%s", countryName(s.Country), s.IP, s.Port), on, pill)
	row.ghost.SetHidden(true)
	row.iconA.SetHidden(true)
	row.iconB.SetHidden(true)
	if on {
		tun := at
		tag := s.Tag
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
