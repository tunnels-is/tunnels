package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/types"
)

func (a *App) recomputeServerView() {
	a.liveByServer = a.activeByServer()
	var shown []types.Server
	for _, s := range a.servers {
		if filterMatch(a.filterServers, s.Tag, s.IP, serverWGAddr(&s), countryName(s.Country), s.Country) {
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
	refresh := newIconBtn(theme.ViewRefreshIcon(), kPrimary, func() {
		a.note("Refreshing servers…")
		a.fetchServers(true)
	})

	active := 0
	am := a.liveByServer
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

	actions := hstackFlex(sp2, 0, search, refresh)

	if len(a.serverView) == 0 {
		msg, desc := "No servers", "Nothing matched this filter."
		if a.filterServers == "" {
			msg, desc = "No servers available", "Your account has no servers assigned yet."
			if a.serversFetching {
				msg, desc = "Loading servers…", ""
			}
		}
		return pageShell("Servers", sub, actions, emptyState(msg, desc))
	}

	cards := make([]fyne.CanvasObject, 0, len(a.serverView))
	for _, s := range a.serverView {
		cards = append(cards, a.serverCard(s))
	}
	return pageShell("Servers", sub, actions, wrapBody(cards...))
}

func (a *App) serverCard(s types.Server) fyne.CanvasObject {
	at := a.liveByServer[s.ID.String()]
	on := at != nil
	pill, status := "Available", toneNeutral
	if on {
		pill, status = "Connected", toneSuccess
	}
	title := s.Tag
	if title == "" {
		title = "Server"
	}

	srv := s
	var action *kBtn
	if on {
		tun, tag := at, s.Tag
		action = dangerBtn("Disconnect", func() {
			a.confirm("Disconnect", "Disconnect from "+tag+"?", func() { a.disconnectActive(tun) })
		})
	} else {
		action = successBtn("Connect", func() {
			a.confirm("Connect", "Connect to "+srv.Tag+"?", func() { a.connectToServer(srv) })
		})
	}

	facts := []fact{
		{label: "Location", value: countryName(s.Country)},
		{label: "Address", value: serverWGAddr(&s), mono: true},
	}
	if on {
		facts = append(facts,
			fact{label: "Download", value: at.IngressString(), mono: true},
			fact{label: "Upload", value: at.EgressString(), mono: true},
		)
	}

	return detailCard(title, badge(pill, status), facts, action)
}
