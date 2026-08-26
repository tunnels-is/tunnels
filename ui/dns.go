package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func (a *App) dnsPage() fyne.CanvasObject {
	if !a.advanced {
		return pageShell("Resolver", "", nil,
			emptyState("Advanced mode required", "Turn on Advanced mode in Settings to manage DNS."))
	}
	cfg := a.config
	if cfg == nil {
		return pageShell("Resolver", "", nil, emptyState("Config not loaded", ""))
	}

	ip := kEntry("127.0.0.1", cfg.DNSServerIP)
	port := kEntry("53", cfg.DNSServerPort)
	dns1 := kEntry("1.1.1.1", cfg.DNS1Default)
	dns2 := kEntry("8.8.8.8", cfg.DNS2Default)
	saveDNS := primaryBtn("Save resolver", func() {
		ipv, portv := strings.TrimSpace(ip.Text), strings.TrimSpace(port.Text)
		d1, d2 := strings.TrimSpace(dns1.Text), strings.TrimSpace(dns2.Text)
		a.updateConfig("Saving resolver", func(c *client.Config) {
			c.DNSServerIP, c.DNSServerPort = ipv, portv
			c.DNS1Default, c.DNS2Default = d1, d2
		}, func() {
			a.note("Resolver saved")
			a.rebuild()
		})
	})

	behaviour := settingList(
		toggleRow("Secure DNS", "Resolve upstream queries over HTTPS.",
			cfg.DNSOverHTTPS, func(bool) { a.toggleConfig("DNSOverHTTPS") }),
		toggleRow("Dynamic encryption", "Pick the DoH endpoint automatically.",
			cfg.DNSHTTPSAutomatic, func(bool) { a.toggleConfig("DNSHTTPSAutomatic") }),
		toggleRow("Log blocked queries", "",
			cfg.LogBlockedDomains, func(bool) { a.toggleConfig("LogBlockedDomains") }),
		toggleRow("Log every query", "Verbose. Records all resolutions, not just blocks.",
			cfg.LogAllDomains, func(bool) { a.toggleConfig("LogAllDomains") }),
		toggleRow("Collect statistics", "Needed for the Statistics page.",
			cfg.DNSstats, func(bool) { a.toggleConfig("DNSstats") }),
	)

	// Local records.
	recRows := []fyne.CanvasObject{}
	for i, r := range cfg.DNSRecords {
		i, r := i, r
		if r == nil {
			continue
		}
		name := r.Domain
		badges := []fyne.CanvasObject{}
		if r.Wildcard {
			badges = append(badges, badge("wildcard", tonePrimary))
		}
		edit := newIconBtn(theme.DocumentCreateIcon(), kGhost, func() { a.editDNSRecord(i, r) }).small()
		del := newIconBtn(theme.DeleteIcon(), kGhost, func() {
			a.confirm("Delete record", "Delete DNS record "+name+"?", func() {
				a.updateConfig("Removing record", func(c *client.Config) {
					list := append([]*types.DNSRecord(nil), c.DNSRecords...)
					if i >= 0 && i < len(list) {
						list = append(list[:i], list[i+1:]...)
					}
					c.DNSRecords = list
				}, a.rebuild)
			})
		}).small()

		target := strings.Join(r.IP, ", ")
		if target == "" {
			target = strings.Join(r.TXT, ", ")
		}
		left := vstack(1,
			hstack(sp2, append([]fyne.CanvasObject{text(name, fsBody, pal().Content, false)}, badges...)...),
			monoText(target, fsSmall, pal().Muted),
		)
		recRows = append(recRows, insetEach(sp2, 0, sp2, 0, splitRow(left, hstack(sp1, edit, del))))
	}
	if len(recRows) == 0 {
		recRows = append(recRows, emptyRow("No local records configured."))
	}
	addRecord := outlineBtn("Add record", func() {
		a.editDNSRecord(-1, &types.DNSRecord{Domain: "yourdomain.com", IP: []string{"127.0.0.1"}, Wildcard: true})
	}).withIcon(theme.ContentAddIcon()).small()

	return pageShell("Resolver", "Local DNS resolver, records and filter lists", nil, scrollBody(
		card("Server", "Where the resolver listens, and the upstream fallbacks.",
			formRows(
				formPair(field("Listen IP", ip), field("Port", port)),
				formPair(field("Primary upstream", dns1), field("Backup upstream", dns2)),
				vspace(sp1),
				hstack(0, saveDNS),
			)),
		card("Behaviour", "Encryption, logging and statistics.", behaviour),
		cardBox("Local records", "A and TXT records answered by this client.", addRecord,
			settingList(recRows...)),
		a.dnsListCard("Block lists", "Domains from these lists are refused.", "DNSBlockLists", cfg.DNSBlockLists, "blocklist"),
		a.dnsListCard("Allow lists", "Domains here always resolve, even if a block list contains them.", "DNSWhiteLists", cfg.DNSWhiteLists, "whitelist"),
	))
}

func (a *App) dnsListCard(title, desc, key string, lists []*client.BlockList, kind string) fyne.CanvasObject {
	update := outlineBtn("Update", func() {
		a.note("Updating lists…")
		go func() {
			if kind == "blocklist" {
				client.UpdateBlockLists()
			} else {
				client.UpdateWhiteLists()
			}
			a.uiDo(func() {
				a.refreshState()
				a.note("Lists updated")
				a.rebuild()
			})
		}()
	}).small()
	create := outlineBtn("Add list", func() {
		a.editDNSList(key, -1, &client.BlockList{Tag: "new-list", URL: "https://example.com/list.txt", Enabled: true})
	}).withIcon(theme.ContentAddIcon()).small()
	enableAll := ghostBtn("All on", func() { a.setAllLists(key, true) }).small()
	disableAll := ghostBtn("All off", func() { a.setAllLists(key, false) }).small()
	actions := hstack(sp1, enableAll, disableAll, update, create)

	rows := []fyne.CanvasObject{}
	for i, l := range lists {
		i, l := i, l
		if l == nil {
			continue
		}
		custom := strings.EqualFold(l.Tag, "custom")
		// No rebuild on success: the switch already shows the new state, and
		// re-rendering the page under the cursor was half of what made this
		// feel like an interruption.
		toggle := newSwitch(l.Enabled, func(v bool) {
			a.updateConfig("Applying list change", func(c *client.Config) {
				src := c.DNSBlockLists
				if key == "DNSWhiteLists" {
					src = c.DNSWhiteLists
				}
				if i < len(src) && src[i] != nil {
					cp := *src[i]
					cp.Enabled = v
					src[i] = &cp
				}
				if key == "DNSWhiteLists" {
					c.DNSWhiteLists = src
				} else {
					c.DNSBlockLists = src
				}
			}, nil)
		})

		right := []fyne.CanvasObject{}
		if custom {
			right = append(right, ghostBtn("Edit domains", func() { a.editCustomList(kind) }).small())
		} else {
			right = append(right,
				newIconBtn(theme.DocumentCreateIcon(), kGhost, func() { a.editDNSList(key, i, l) }).small(),
				newIconBtn(theme.DeleteIcon(), kGhost, func() {
					a.confirm("Delete list", "Delete "+l.Tag+"?", func() { a.deleteListItem(key, i) })
				}).small(),
			)
		}
		right = append(right, hspace(sp1), toggle)

		left := vstack(1,
			text(l.Tag, fsBody, pal().Content, false),
			text(fmt.Sprintf("%d domains", l.Count), fsSmall, pal().Faint, false),
		)
		rows = append(rows, insetEach(sp2, 0, sp2, 0, splitRow(left, hstack(sp1, right...))))
	}
	if len(rows) == 0 {
		rows = append(rows, emptyRow("None configured."))
	}
	return cardBox(title, desc, actions, settingList(rows...))
}

func (a *App) setAllLists(key string, enabled bool) {
	label := "Enabling lists"
	if !enabled {
		label = "Disabling lists"
	}
	// Every switch changes here, so this one does rebuild once it lands.
	a.updateConfig(label, func(c *client.Config) {
		src := c.DNSBlockLists
		if key == "DNSWhiteLists" {
			src = c.DNSWhiteLists
		}
		out := make([]*client.BlockList, len(src))
		for i, l := range src {
			if l == nil {
				continue
			}
			cp := *l
			cp.Enabled = enabled
			out[i] = &cp
		}
		if key == "DNSWhiteLists" {
			c.DNSWhiteLists = out
		} else {
			c.DNSBlockLists = out
		}
	}, a.rebuild)
}

func (a *App) deleteListItem(key string, index int) {
	a.updateConfig("Removing list", func(c *client.Config) {
		src := c.DNSBlockLists
		if key == "DNSWhiteLists" {
			src = c.DNSWhiteLists
		}
		if index < 0 || index >= len(src) {
			return
		}
		src = append(src[:index], src[index+1:]...)
		if key == "DNSWhiteLists" {
			c.DNSWhiteLists = src
		} else {
			c.DNSBlockLists = src
		}
	}, a.rebuild)
}

func (a *App) editDNSList(key string, index int, list *client.BlockList) {
	tag := kEntry("list name", list.Tag)
	url := kEntry("https://…", list.URL)
	en := bindCheck("Enabled", list.Enabled, nil)
	form := container.New(fixedLayout{w: z(420)},
		vstack(sp3, field("Name", tag), field("URL", url), en))
	d := dialog.NewCustomConfirm("Filter list", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		cp := *list
		cp.Tag = strings.TrimSpace(tag.Text)
		cp.URL = strings.TrimSpace(url.Text)
		cp.Enabled = en.Checked
		a.updateConfig("Saving list", func(c *client.Config) {
			src := c.DNSBlockLists
			if key == "DNSWhiteLists" {
				src = c.DNSWhiteLists
			}
			src = append([]*client.BlockList(nil), src...)
			if index >= 0 && index < len(src) {
				src[index] = &cp
			} else {
				src = append(src, &cp)
			}
			if key == "DNSWhiteLists" {
				c.DNSWhiteLists = src
			} else {
				c.DNSBlockLists = src
			}
		}, a.rebuild)
	}, a.win)
	d.Resize(fyne.NewSize(z(480), z(320)))
	d.Show()
}

func (a *App) editDNSRecord(index int, rec *types.DNSRecord) {
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
		if !ok {
			return
		}
		cp := types.DNSRecord{
			Domain:   strings.TrimSpace(domain.Text),
			Wildcard: wild.Checked,
			IP:       splitLines(ips.Text),
			TXT:      splitLines(txt.Text),
		}
		a.updateConfig("Saving record", func(c *client.Config) {
			list := append([]*types.DNSRecord(nil), c.DNSRecords...)
			if index >= 0 && index < len(list) {
				list[index] = &cp
			} else {
				list = append(list, &cp)
			}
			c.DNSRecords = list
		}, a.rebuild)
	}, a.win)
	d.Resize(fyne.NewSize(z(480), z(480)))
	d.Show()
}

func (a *App) editCustomList(kind string) {
	a.note("Loading list…")
	go func() {
		data, err := client.GetDNSListContent(kind)
		a.uiDo(func() {
			if err != nil {
				a.fail(err.Error())
				return
			}
			area := kMultiline(data.Content, 18)
			area.TextStyle = fyne.TextStyle{Monospace: true}
			d := dialog.NewCustomConfirm("Custom "+kind, "Save", "Cancel",
				container.NewScroll(area), func(ok bool) {
					if !ok {
						return
					}
					a.note("Saving list…")
					go func() {
						out, err := client.SetDNSListContent(kind, area.Text)
						a.uiDo(func() {
							if err != nil {
								a.fail(err.Error())
								return
							}
							n := 0
							if out != nil {
								n = out.Count
							}
							a.refreshState()
							a.note(fmt.Sprintf("Custom list saved (%d domains)", n))
							a.rebuild()
						})
					}()
				}, a.win)
			d.Resize(fyne.NewSize(z(680), z(560)))
			d.Show()
		})
	}()
}
