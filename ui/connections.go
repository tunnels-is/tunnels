package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) connectionsPage() fyne.CanvasObject {
	var mine []*client.TUN
	for _, t := range a.active {
		if t != nil && t.CR != nil && a.user != nil && t.CR.UserID == a.user.ID {
			mine = append(mine, t)
		}
	}
	if len(mine) == 0 {
		return pageShell("Connections", "Live tunnels", nil,
			emptyState("Nothing connected", "Connect to a server to see live connection details here."))
	}

	cards := make([]fyne.CanvasObject, 0, len(mine))
	for _, ac := range mine {
		ac := ac
		var tun *client.TunnelMETA
		tag := ""
		if ac.CR != nil {
			tag = ac.CR.Tag
			tun = client.FindTunnel(tag)
		}

		rows := []fyne.CanvasObject{
			kvRow("Download", ac.IngressString(), true),
			kvRow("Upload", ac.EgressString(), true),
		}
		if ac.ServerResponse != nil {
			sr := ac.ServerResponse
			rows = append(rows,
				kvRow("Public IP", sr.InterfaceIP, true),
				kvRow("WireGuard IP", sr.WireGuardIP, true),
				kvRow("DNS servers", strings.Join(sr.DNSServers, " "), true),
			)
		}
		if tun != nil {
			rows = append(rows,
				kvRow("Interface", tun.IFName, true),
				kvRow("MTU", fmt.Sprintf("%d", tun.MTU), true),
				kvRow("DNS blocking", boolOn(tun.DNSBlocking), false),
				kvRow("Auto connect", boolOn(tun.AutoConnect), false),
				kvRow("Auto reconnect", boolOn(tun.AutoReconnect), false),
			)
		}

		disc := dangerBtn("Disconnect", func() {
			a.confirm("Disconnect", "Disconnect "+tag+"?", func() { a.disconnectActive(ac) })
		})

		title := tag
		if title == "" {
			title = ac.ID
		}
		cards = append(cards, cardBox(title, "Live tunnel", disc, vstack(0, rows...)))
	}

	return pageShell("Connections", fmtCount(len(mine), "live tunnels"), nil, scrollBody(cards...))
}

func boolOn(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}
