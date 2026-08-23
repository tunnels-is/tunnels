package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func (a *App) tunnelEditPage() fyne.CanvasObject {
	back := outlineBtn("Back", func() { a.show(pageTunnels) }).withIcon(theme.NavigateBackIcon())

	meta := client.FindTunnel(a.editTag)
	if meta == nil {
		return pageShell("Tunnel", a.editTag, back,
			emptyState("Tunnel not found", `No tunnel named "`+a.editTag+`" exists any more.`))
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

	tag := kEntry("tunnel name", form.Tag)
	ifname := kEntry("interface name", form.IFName)
	mtu := kEntry("1420", strconv.Itoa(int(form.MTU)))
	txq := kEntry("3000", strconv.Itoa(int(form.TxQueueLen)))

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
	server := kSelect(srvOpts, sel, nil)

	switches := map[string]*kSwitch{}
	toggle := func(key, title, desc string, v bool) fyne.CanvasObject {
		s := newSwitch(v, nil)
		switches[key] = s
		return settingRow(title, desc, s)
	}
	features := settingList(
		toggle("AutoConnect", "Auto connect", "Bring this tunnel up when the app starts.", form.AutoConnect),
		toggle("AutoReconnect", "Auto reconnect", "Re-establish the tunnel if it drops.", form.AutoReconnect),
		toggle("EnableDefaultRoute", "Default route", "Send all traffic through this tunnel.", form.EnableDefaultRoute),
		toggle("DNSBlocking", "DNS blocking", "Apply the resolver's block lists on this tunnel.", form.DNSBlocking),
		toggle("LocalhostNat", "Localhost NAT", "NAT loopback traffic into the tunnel.", form.LocalhostNat),
		toggle("EnableWAN", "WAN routing", "Allow routing to the tunnel's wider network.", form.EnableWAN),
	)

	dns := kMultiline(strings.Join(form.DNSServers, "\n"), 3)

	var rlines []string
	for _, r := range form.Routes {
		if r == nil {
			continue
		}
		rlines = append(rlines, strings.TrimSpace(r.Address+" "+r.Metric))
	}
	routes := kMultiline(strings.Join(rlines, "\n"), 4)

	var nlines []string
	for _, n := range form.Networks {
		if n == nil {
			continue
		}
		nlines = append(nlines, strings.TrimSpace(n.Tag+" "+n.Network+" "+n.Nat))
	}
	nets := kMultiline(strings.Join(nlines, "\n"), 4)

	save := primaryBtn("Save changes", func() {
		form.Tag = strings.TrimSpace(tag.Text)
		form.IFName = strings.TrimSpace(ifname.Text)
		form.ServerID = srvIDs[server.Selected]
		if n, err := strconv.Atoi(strings.TrimSpace(mtu.Text)); err == nil {
			form.MTU = int32(n)
		}
		if n, err := strconv.Atoi(strings.TrimSpace(txq.Text)); err == nil {
			form.TxQueueLen = int32(n)
		}
		form.DNSBlocking = switches["DNSBlocking"].on
		form.LocalhostNat = switches["LocalhostNat"].on
		form.AutoReconnect = switches["AutoReconnect"].on
		form.AutoConnect = switches["AutoConnect"].on
		form.EnableDefaultRoute = switches["EnableDefaultRoute"].on
		form.EnableWAN = switches["EnableWAN"].on
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

	cards := []fyne.CanvasObject{}
	if connected {
		cards = append(cards, notice("This tunnel is connected. Disconnect it before saving changes.", toneWarning))
	}
	cards = append(cards,
		card("General", "Identity, server and transport.",
			formRows(
				formPair(field("Name", tag), field("Interface", ifname)),
				field("Server", server),
				formPair(field("MTU", mtu), field("TX queue length", txq)),
			)),
		card("Behaviour", "What this tunnel does while connected.", features),
		card("DNS servers", "One resolver per line.", dns),
		card("Routes", "One per line, as ADDRESS METRIC.", routes),
		card("Networks", "One per line, as TAG NETWORK NAT.", nets),
	)

	actions := hstack(sp2, back, save)
	sub := "Interface " + form.IFName
	if connected {
		sub += "  ·  connected"
	}
	return pageShell(form.Tag, sub, actions, scrollBody(cards...))
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
