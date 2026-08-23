package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
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
		return pageScroll(card("", "", muted("No active connections")))
	}
	cards := []fyne.CanvasObject{widget.NewLabel("Connections")}
	for _, ac := range mine {
		ac := ac
		var tun *client.TunnelMETA
		tag := ""
		if ac.CR != nil {
			tag = ac.CR.Tag
			tun = client.FindTunnel(tag)
		}
		rows := []fyne.CanvasObject{}
		if tun != nil {
			rows = append(rows,
				infoRow("Tag", tun.Tag),
				infoRow("Interface", tun.IFName),
				infoRow("MTU", fmt.Sprintf("%d", tun.MTU)),
				infoRow("DNS blocking", boolOn(tun.DNSBlocking)),
				infoRow("DNS servers", strings.Join(tun.DNSServers, " ")),
				infoRow("Auto connect", boolOn(tun.AutoConnect)),
				infoRow("Auto re-connect", boolOn(tun.AutoReconnect)),
			)
		}
		if ac.CR != nil {
			rows = append(rows, infoRow("User ID", ac.CR.UserID))
		}
		rows = append(rows,
			infoRow("Download", ac.IngressString()),
			infoRow("Upload", ac.EgressString()),
		)
		if ac.ServerResponse != nil {
			sr := ac.ServerResponse
			rows = append(rows,
				infoRow("Public IP", sr.InterfaceIP),
				infoRow("WireGuard IP", sr.WireGuardIP),
				infoRow("DNS servers", strings.Join(sr.DNSServers, " ")),
			)
		}
		disc := dangerBtn("Disconnect", func() {
			a.confirm("Disconnect", "Disconnect "+tag+"?", func() { a.disconnectActive(ac) })
		})
		title := tag
		if title == "" {
			title = ac.ID
		}
		cards = append(cards, card(title, "", container.NewVBox(append(rows, disc)...)))
	}
	return pageScroll(cards...)
}

func boolOn(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}
