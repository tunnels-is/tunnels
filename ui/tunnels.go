package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) recomputeTunnelView() {
	a.liveByTag = a.activeByTag()
	var shown []*client.TunnelMeta
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
	byTag := a.liveByTag
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

	cards := make([]fyne.CanvasObject, 0, len(a.tunnelView))
	for _, t := range a.tunnelView {
		if t == nil {
			continue
		}
		cards = append(cards, a.tunnelCard(t))
	}
	return pageShell("Tunnels", sub, actions, wrapBody(cards...))
}

func (a *App) tunnelCard(t *client.TunnelMeta) fyne.CanvasObject {
	at := a.liveByTag[t.Tag]
	srv := a.serverByID(t.ServerID)
	srvLabel := "No server"
	addr := ""
	if srv != nil {
		srvLabel = srv.Tag
		addr = serverWGAddr(srv)
	}
	on := at != nil
	pill, toneName := "Offline", toneNeutral
	if on {
		pill, toneName = "Connected", toneSuccess
	}
	title := t.Tag
	if title == "" {
		title = "Tunnel"
	}

	tun := t
	edit := newIconBtn(theme.DocumentCreateIcon(), kGhost, func() {
		a.editTag = tun.Tag
		a.show(pageTunnelEdit)
	})
	del := newIconBtn(theme.DeleteIcon(), kDanger, func() {
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
	firewall := ghostBtn("Firewall", func() {
		a.peersTag = tun.Tag
		a.show(pageTunnelPeers)
	})

	var connect *kBtn
	if on {
		liveTun := at
		connect = dangerBtn("Disconnect", func() {
			a.confirm("Disconnect", "Disconnect "+tun.Tag+"?", func() { a.disconnectActive(liveTun) })
		})
	} else {
		connect = successBtn("Connect", func() {
			a.confirm("Connect", "Connect "+tun.Tag+"?", func() { a.connectTunnel(tun) })
		})
	}

	facts := []fact{
		{label: "Server", value: srvLabel},
		{label: "Address", value: addr, mono: true},
		{label: "Interface", value: t.IFName, mono: true},
	}
	if on {
		facts = append(facts,
			fact{label: "Download", value: at.IngressString(), mono: true},
			fact{label: "Upload", value: at.EgressString(), mono: true},
		)
	}

	return detailCard(title, hstack(sp2, badge(pill, toneName), edit, del), facts, connect, firewall)
}
