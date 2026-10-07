package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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
	form := client.CloneTunnelMeta(meta)
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

	// Records stay on the cloned form until Save changes. reflow is assigned
	// once the scroll exists, so the first render skips it. A domain with no
	// address is a route; the two cards are two views of the same list.
	recBody := container.NewStack()
	routeBody := container.NewStack()
	var renderDNS func()
	renderDNS = func() {
		recBody.Objects = []fyne.CanvasObject{settingList(tunnelDNSRows(form.DNSRecords, func(i int, rec *types.DNSRecord) {
			a.editTunnelDNSRecord(rec, func(cp types.DNSRecord) {
				if i >= 0 && i < len(form.DNSRecords) {
					form.DNSRecords[i] = &cp
				}
				renderDNS()
			})
		}, func(i int, name string) {
			a.confirm("Delete record", "Delete DNS record "+name+"?", func() {
				if i >= 0 && i < len(form.DNSRecords) {
					form.DNSRecords = append(form.DNSRecords[:i], form.DNSRecords[i+1:]...)
				}
				renderDNS()
			})
		})...)}
		routeBody.Objects = []fyne.CanvasObject{settingList(tunnelDNSRouteRows(form.DNSRecords, func(i int, rec *types.DNSRecord) {
			a.editTunnelDNSRoute(rec, func(cp types.DNSRecord) {
				if i >= 0 && i < len(form.DNSRecords) {
					form.DNSRecords[i] = &cp
				}
				renderDNS()
			})
		}, func(i int, name string) {
			a.confirm("Remove domain", "Stop routing "+name+" through this tunnel?", func() {
				if i >= 0 && i < len(form.DNSRecords) {
					form.DNSRecords = append(form.DNSRecords[:i], form.DNSRecords[i+1:]...)
				}
				renderDNS()
			})
		})...)}
		recBody.Refresh()
		routeBody.Refresh()
		if reflow != nil {
			reflow()
		}
	}
	renderDNS()
	addRoute := outlineBtn("Add domain", func() {
		a.editTunnelDNSRoute(nil, func(cp types.DNSRecord) {
			form.DNSRecords = append(form.DNSRecords, &cp)
			renderDNS()
		})
	}).withIcon(theme.ContentAddIcon()).small()
	addRecord := outlineBtn("Add record", func() {
		a.editTunnelDNSRecord(&types.DNSRecord{Domain: "yourdomain.com", IP: []string{"127.0.0.1"}, Wildcard: true}, func(cp types.DNSRecord) {
			form.DNSRecords = append(form.DNSRecords, &cp)
			renderDNS()
		})
	}).withIcon(theme.ContentAddIcon()).small()

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
		cards = append(cards, fullRow(notice("This tunnel is connected. Disconnect it before saving changes.", toneWarning)))
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
		cardBox("DNS routing",
			"Send lookups for specific domains through this tunnel. Saved with the tunnel.",
			addRoute, routeBody),
		cardBox("DNS records",
			"Fixed answers for names on this tunnel. Saved with the tunnel.",
			addRecord, recBody),
		card("Routes", "Extra routes installed while the tunnel is up.",
			capWidth(z(720), routeEd.object())),
		card("Networks", "Networks reachable through the tunnel, with optional NAT.",
			capWidth(z(720), netEd.object())),
		card("Blocked ports", "Outbound TCP and UDP ports dropped on this tunnel.",
			capWidth(formWidth, field("Ports", ports))),
	)

	flow := scrollFlow(cards...)
	body := scrollBodyOf(flow)
	// Refresh the scroll too: the flow's height changes when a row is added,
	// and the scroller only remeasures when it is refreshed itself.
	reflow = func() {
		flow.Refresh()
		body.Refresh()
	}

	actions := hstack(sp2, back, save)
	sub := "Interface " + form.IFName
	if connected {
		sub += "  ·  connected"
	}
	return pageShell(form.Tag, sub, actions, body)
}

// dnsRecordIsRoute is a domain sent through the tunnel. An address or TXT
// value makes it a fixed answer instead.
func dnsRecordIsRoute(r *types.DNSRecord) bool {
	return r != nil && len(r.IP) == 0 && len(r.TXT) == 0
}

// tunnelDNSRows is the list body for a tunnel's fixed DNS answers.
// index is the slot in the form slice, so a nil hole is not shown and is
// not renumbered out from under edit and delete. Routes are left out; they
// have their own list.
func tunnelDNSRows(records []*types.DNSRecord, onEdit func(int, *types.DNSRecord), onDelete func(int, string)) []fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(records))
	for i, r := range records {
		i, r := i, r
		if r == nil || dnsRecordIsRoute(r) {
			continue
		}
		name := r.Domain
		if name == "" {
			name = "unnamed"
		}
		title := []fyne.CanvasObject{text(name, fsBody, pal().Content, false)}
		if r.Wildcard {
			title = append(title, badge("wildcard", tonePrimary))
		}
		target := strings.Join(r.IP, ", ")
		if txt := strings.Join(r.TXT, ", "); txt != "" {
			if target != "" {
				target += "  ·  " + txt
			} else {
				target = txt
			}
		}
		left := vstack(1, hstack(sp2, title...), monoText(target, fsSmall, pal().Muted))
		edit := newIconBtn(theme.DocumentCreateIcon(), kGhost, func() { onEdit(i, r) }).small()
		del := newIconBtn(theme.DeleteIcon(), kGhost, func() { onDelete(i, name) }).small()
		rows = append(rows, insetEach(sp2, 0, sp2, 0, splitRow(left, hstack(sp1, edit, del))))
	}
	if len(rows) == 0 {
		rows = append(rows, emptyRow("No records on this tunnel."))
	}
	return rows
}

// tunnelDNSRouteRows lists domains whose lookups are forwarded through the
// tunnel. index matches the shared DNSRecords slice.
func tunnelDNSRouteRows(records []*types.DNSRecord, onEdit func(int, *types.DNSRecord), onDelete func(int, string)) []fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(records))
	for i, r := range records {
		i, r := i, r
		if !dnsRecordIsRoute(r) {
			continue
		}
		name := r.Domain
		if name == "" {
			name = "unnamed"
		}
		edit := newIconBtn(theme.DocumentCreateIcon(), kGhost, func() { onEdit(i, r) }).small()
		del := newIconBtn(theme.DeleteIcon(), kGhost, func() { onDelete(i, name) }).small()
		row := splitRow(text(name, fsBody, pal().Content, false), hstack(sp1, edit, del))
		rows = append(rows, insetEach(sp2, 0, sp2, 0, row))
	}
	if len(rows) == 0 {
		rows = append(rows, emptyRow("No domains are routed through this tunnel."))
	}
	return rows
}

// editTunnelDNSRoute asks for a domain and stores it as a route: no address,
// wildcard on, so the name and everything under it is forwarded.
func (a *App) editTunnelDNSRoute(rec *types.DNSRecord, onSave func(types.DNSRecord)) {
	current := ""
	if rec != nil {
		current = rec.Domain
	}
	domain := kEntry("example.com", current)
	form := container.New(fixedLayout{w: z(420)}, vstack(sp3,
		hint("Lookups for this domain, and any name under it, are sent through this tunnel. The query goes to the tunnel's DNS servers from the tunnel address, so the lookup travels inside the tunnel."),
		field("Domain", domain),
	))
	d := dialog.NewCustomConfirm("Route domain", "Save", "Cancel", form, func(ok bool) {
		if !ok || onSave == nil {
			return
		}
		name := strings.TrimSuffix(strings.TrimSpace(domain.Text), ".")
		if name == "" {
			a.fail("A domain is required")
			return
		}
		onSave(types.DNSRecord{Domain: name, Wildcard: true})
	}, a.win)
	d.Resize(fyne.NewSize(z(460), z(320)))
	d.Show()
}

// editTunnelDNSRecord edits one fixed answer in memory. The tunnel file is
// written only when the page's Save changes button runs.
func (a *App) editTunnelDNSRecord(rec *types.DNSRecord, onSave func(types.DNSRecord)) {
	if rec == nil {
		rec = &types.DNSRecord{}
	}
	domain := kEntry("yourdomain.com", rec.Domain)
	ips := kMultiline(strings.Join(rec.IP, "\n"), 3)
	txt := kMultiline(strings.Join(rec.TXT, "\n"), 3)
	wild := bindCheck("Match subdomains (wildcard)", rec.Wildcard, nil)
	form := container.New(fixedLayout{w: z(420)}, vstack(sp3,
		field("Domain", domain),
		fieldWith("IP addresses", "One per line.", ips),
		fieldWith("TXT records", "One per line.", txt),
		wild,
	))
	d := dialog.NewCustomConfirm("DNS record", "Save", "Cancel", form, func(ok bool) {
		if !ok || onSave == nil {
			return
		}
		cp := types.DNSRecord{
			Domain:   strings.TrimSpace(domain.Text),
			Wildcard: wild.Checked,
			IP:       splitLines(ips.Text),
			TXT:      splitLines(txt.Text),
		}
		if cp.Domain == "" {
			a.fail("A domain is required")
			return
		}
		onSave(cp)
	}, a.win)
	d.Resize(fyne.NewSize(z(480), z(480)))
	d.Show()
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
