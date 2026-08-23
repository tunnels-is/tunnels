package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func (a *App) tunnelEditPage() fyne.CanvasObject {
	meta := client.FindTunnel(a.editTag)
	if meta == nil {
		return container.NewVBox(
			ghostBtn("Back", func() { a.show(pageTunnels) }),
			muted(`Tunnel "`+a.editTag+`" not found.`),
		)
	}
	form := client.CloneTunnelMETA(meta)
	if form.DNSServers == nil {
		form.DNSServers = []string{}
	}
	if form.Routes == nil {
		form.Routes = []*types.Route{}
	}
	if form.Networks == nil {
		form.Networks = []*types.Network{}
	}

	connected := a.activeByTag()[form.Tag] != nil

	tag := widget.NewEntry()
	tag.SetText(form.Tag)
	ifname := widget.NewEntry()
	ifname.SetText(form.IFName)
	mtu := widget.NewEntry()
	mtu.SetText(strconv.Itoa(int(form.MTU)))
	txq := widget.NewEntry()
	txq.SetText(strconv.Itoa(int(form.TxQueueLen)))

	srvOpts := []string{"None"}
	srvIDs := map[string]string{"None": ""}
	sel := "None"
	for _, s := range a.servers {
		label := s.Tag + " (" + s.IP + ")"
		srvOpts = append(srvOpts, label)
		srvIDs[label] = s.ID.String()
		if s.ID.String() == form.ServerID {
			sel = label
		}
	}
	server := widget.NewSelect(srvOpts, nil)
	server.SetSelected(sel)

	checks := map[string]*widget.Check{}
	addCheck := func(key, label string, v bool) *widget.Check {
		c := bindCheck(label, v, nil)
		checks[key] = c
		return c
	}

	dns := widget.NewMultiLineEntry()
	dns.SetText(strings.Join(form.DNSServers, "\n"))
	dns.SetMinRowsVisible(3)

	routes := widget.NewMultiLineEntry()
	var rlines []string
	for _, r := range form.Routes {
		if r == nil {
			continue
		}
		rlines = append(rlines, strings.TrimSpace(r.Address+" "+r.Metric))
	}
	routes.SetText(strings.Join(rlines, "\n"))
	routes.SetMinRowsVisible(3)

	nets := widget.NewMultiLineEntry()
	var nlines []string
	for _, n := range form.Networks {
		if n == nil {
			continue
		}
		nlines = append(nlines, strings.TrimSpace(n.Tag+" "+n.Network+" "+n.Nat))
	}
	nets.SetText(strings.Join(nlines, "\n"))
	nets.SetMinRowsVisible(3)

	save := primaryBtn("Save", func() {
		form.Tag = strings.TrimSpace(tag.Text)
		form.IFName = strings.TrimSpace(ifname.Text)
		form.ServerID = srvIDs[server.Selected]
		if n, err := strconv.Atoi(strings.TrimSpace(mtu.Text)); err == nil {
			form.MTU = int32(n)
		}
		if n, err := strconv.Atoi(strings.TrimSpace(txq.Text)); err == nil {
			form.TxQueueLen = int32(n)
		}
		form.DNSBlocking = checks["DNSBlocking"].Checked
		form.LocalhostNat = checks["LocalhostNat"].Checked
		form.AutoReconnect = checks["AutoReconnect"].Checked
		form.AutoConnect = checks["AutoConnect"].Checked
		form.EnableDefaultRoute = checks["EnableDefaultRoute"].Checked
		form.EnableWAN = checks["EnableWAN"].Checked
		form.DNSServers = splitLines(dns.Text)
		form.Routes = nil
		for _, line := range splitLines(routes.Text) {
			p := strings.Fields(line)
			r := &types.Route{}
			if len(p) > 0 {
				r.Address = p[0]
			}
			if len(p) > 1 {
				r.Metric = p[1]
			}
			form.Routes = append(form.Routes, r)
		}
		form.Networks = nil
		for _, line := range splitLines(nets.Text) {
			p := strings.Fields(line)
			n := &types.Network{}
			if len(p) > 0 {
				n.Tag = p[0]
			}
			if len(p) > 1 {
				n.Network = p[1]
			}
			if len(p) > 2 {
				n.Nat = p[2]
			}
			form.Networks = append(form.Networks, n)
		}
		if err := client.SaveTunnel(form, a.editTag); err != nil {
			a.fail(err.Error())
			return
		}
		a.refreshState()
		a.note("Tunnel saved")
		a.show(pageTunnels)
	})
	if connected {
		save.Disable()
	}

	warn := fyne.CanvasObject(widget.NewLabel(""))
	if connected {
		warn = wrapLabel("This tunnel is connected — disconnect it before saving changes.")
	}

	return pageScroll(
		toolbar(widget.NewLabel(a.editTag),
			ghostBtn("Back", func() { a.show(pageTunnels) }),
			ghostBtn("Cancel", func() { a.show(pageTunnels) }),
			save,
		),
		warn,
		card("General", "Identity, server and transport settings.", container.NewVBox(
			labeled("Tag", tag),
			labeled("Interface", ifname),
			labeled("Server", server),
			labeled("MTU", mtu),
			labeled("TX queue length", txq),
		)),
		card("Features", "Behaviour of this tunnel while connected.", container.NewVBox(
			addCheck("DNSBlocking", "DNS blocking", form.DNSBlocking),
			addCheck("LocalhostNat", "Localhost NAT", form.LocalhostNat),
			addCheck("AutoReconnect", "Auto reconnect", form.AutoReconnect),
			addCheck("AutoConnect", "Auto connect", form.AutoConnect),
			addCheck("EnableDefaultRoute", "Default route", form.EnableDefaultRoute),
			addCheck("EnableWAN", "WAN routing", form.EnableWAN),
		)),
		card("DNS servers", "One resolver per line.", dns),
		card("Routes", "One per line: ADDRESS METRIC", routes),
		card("Networks", "One per line: TAG NETWORK NAT", nets),
	)
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
