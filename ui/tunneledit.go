package ui

import (
	"fmt"
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
		toggle("KillSwitch", "Kill switch", "Blackhole traffic if this tunnel goes down.", form.KillSwitch),
		toggle("DNSBlocking", "DNS blocking", "Apply the resolver's block lists on this tunnel.", form.DNSBlocking),
		toggle("LocalhostNat", "Localhost NAT", "NAT loopback traffic into the tunnel.", form.LocalhostNat),
		toggle("EnableWAN", "WAN routing", "Allow routing to the tunnel's wider network.", form.EnableWAN),
	)

	// Row editors need to nudge the page to re-lay out when a row is added or
	// removed, since a container's own Refresh does not reach its parents.
	var reflow func()
	bump := func() {
		if reflow != nil {
			reflow()
		}
	}

	dnsRows := make([][]string, 0, len(form.DNSServers))
	for _, s := range form.DNSServers {
		dnsRows = append(dnsRows, []string{s})
	}
	dnsEd := newRowEditor("resolver",
		[]fieldCol{{label: "Resolver", placeholder: "1.1.1.1", weight: 1}},
		dnsRows, "No resolvers set — the tunnel keeps the system DNS.", bump)

	routeRows := make([][]string, 0, len(form.Routes))
	for _, r := range form.Routes {
		if r == nil {
			continue
		}
		routeRows = append(routeRows, []string{r.Address, r.Metric, r.Gateway})
	}
	routeEd := newRowEditor("route", []fieldCol{
		{label: "Address", placeholder: "10.0.0.0/24", weight: 2.2},
		{label: "Metric", placeholder: "1", weight: 1},
		{label: "Gateway", placeholder: "optional", weight: 1.8},
	}, routeRows, "No extra routes.", bump)

	netRows := make([][]string, 0, len(form.Networks))
	for _, n := range form.Networks {
		if n == nil {
			continue
		}
		netRows = append(netRows, []string{n.Tag, n.Network, n.Nat})
	}
	netEd := newRowEditor("network", []fieldCol{
		{label: "Tag", placeholder: "lan", weight: 1.2},
		{label: "Network", placeholder: "192.168.1.0/24", weight: 2},
		{label: "NAT", placeholder: "optional", weight: 2},
	}, netRows, "No networks mapped.", bump)

	portStrs := make([]string, 0, len(form.BlockedPorts))
	for _, p := range form.BlockedPorts {
		portStrs = append(portStrs, strconv.Itoa(int(p)))
	}
	ports := kEntry("e.g. 25, 445, 3389", strings.Join(portStrs, ", "))

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
		form.AutoConnect = switches["AutoConnect"].on
		form.AutoReconnect = switches["AutoReconnect"].on
		form.EnableDefaultRoute = switches["EnableDefaultRoute"].on
		form.KillSwitch = switches["KillSwitch"].on
		form.DNSBlocking = switches["DNSBlocking"].on
		form.LocalhostNat = switches["LocalhostNat"].on
		form.EnableWAN = switches["EnableWAN"].on

		form.DNSServers = dnsEd.column(0)

		// Gateway is carried through: the old parser only read Address and
		// Metric, so saving used to erase it from every route.
		form.Routes = nil
		for _, r := range routeEd.values() {
			form.Routes = append(form.Routes, &types.Route{Address: r[0], Metric: r[1], Gateway: r[2]})
		}
		form.Networks = nil
		for _, n := range netEd.values() {
			form.Networks = append(form.Networks, &types.Network{Tag: n[0], Network: n[1], Nat: n[2]})
		}

		form.BlockedPorts = nil
		var bad []string
		for _, p := range splitCSV(ports.Text) {
			n, err := strconv.ParseUint(p, 10, 16)
			if err != nil {
				bad = append(bad, p)
				continue
			}
			form.BlockedPorts = append(form.BlockedPorts, uint16(n))
		}
		if len(bad) > 0 {
			a.fail("Not a valid port: " + strings.Join(bad, ", "))
			return
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
		card("DNS servers", "Resolvers handed to the interface, in order.",
			capWidth(formWidth, dnsEd.object())),
		card("Routes", "Extra routes installed while the tunnel is up.",
			capWidth(z(720), routeEd.object())),
		card("Networks", "Networks reachable through the tunnel, with optional NAT.",
			capWidth(z(720), netEd.object())),
		card("Blocked ports", "Outbound TCP and UDP ports dropped on this tunnel.",
			capWidth(formWidth, field("Ports", ports))),
	)

	// DNSRecords and the WireGuard key are not editable here, but the form is
	// cloned from the live tunnel so they survive a save untouched.
	if n := len(form.DNSRecords); n > 0 {
		cards = append(cards, card("Local DNS records",
			fmt.Sprintf("%d record(s) are attached to this tunnel. Edit them on the Resolver page.", n), nil))
	}

	col := vstack(sp4, cards...)
	reflow = func() { col.Refresh() }

	actions := hstack(sp2, back, save)
	sub := "Interface " + form.IFName
	if connected {
		sub += "  ·  connected"
	}
	return pageShell(form.Tag, sub, actions,
		scrollBodyOf(col))
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
