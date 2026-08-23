package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func (a *App) dnsPage() fyne.CanvasObject {
	if !a.advanced {
		return container.NewCenter(muted("Enable Advanced mode in Settings to manage DNS."))
	}
	cfg := a.config
	if cfg == nil {
		return muted("Config not loaded")
	}

	ip := widget.NewEntry()
	ip.SetText(cfg.DNSServerIP)
	port := widget.NewEntry()
	port.SetText(cfg.DNSServerPort)
	dns1 := widget.NewEntry()
	dns1.SetText(cfg.DNS1Default)
	dns2 := widget.NewEntry()
	dns2.SetText(cfg.DNS2Default)
	saveDNS := widget.NewButton("Save DNS server", func() {
		next := client.CloneConfig()
		next.DNSServerIP = strings.TrimSpace(ip.Text)
		next.DNSServerPort = strings.TrimSpace(port.Text)
		next.DNS1Default = strings.TrimSpace(dns1.Text)
		next.DNS2Default = strings.TrimSpace(dns2.Text)
		if a.saveConfig(next) {
			a.rebuild()
		}
	})

	behave := container.NewVBox(
		bindCheck("Secure DNS", cfg.DNSOverHTTPS, func(v bool) { a.toggleConfig("DNSOverHTTPS") }),
		bindCheck("Log blocked", cfg.LogBlockedDomains, func(v bool) { a.toggleConfig("LogBlockedDomains") }),
		bindCheck("Log all", cfg.LogAllDomains, func(v bool) { a.toggleConfig("LogAllDomains") }),
		bindCheck("Stats", cfg.DNSstats, func(v bool) { a.toggleConfig("DNSstats") }),
		bindCheck("Dynamic encryption", cfg.DNSHTTPSAutomatic, func(v bool) { a.toggleConfig("DNSHTTPSAutomatic") }),
	)

	records := []fyne.CanvasObject{}
	for i, r := range cfg.DNSRecords {
		i, r := i, r
		if r == nil {
			continue
		}
		name := r.Domain
		if r.Wildcard {
			name += " *"
		}
		edit := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() { a.editDNSRecord(i, r) })
		del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			next := client.CloneConfig()
			list := append([]*types.DNSRecord(nil), next.DNSRecords...)
			if i >= 0 && i < len(list) {
				list = append(list[:i], list[i+1:]...)
			}
			next.DNSRecords = list
			a.saveConfig(next)
			a.rebuild()
		})
		records = append(records, container.NewBorder(nil, nil, nil, container.NewHBox(edit, del),
			container.NewVBox(widget.NewLabel(name), muted(strings.Join(r.IP, ", ")))))
	}
	if len(records) == 0 {
		records = append(records, muted("No records configured"))
	}

	return pageScroll(
		card("DNS server", "Address the resolver listens on and upstream fallback resolvers.", container.NewVBox(
			labeled("Server IP", ip),
			labeled("Port", port),
			labeled("Primary DNS", dns1),
			labeled("Backup DNS", dns2),
			saveDNS,
		)),
		card("Behaviour", "Encryption, logging and statistics for the resolver.", behave),
		card("Records", "Locally resolved A and TXT records.", container.NewVBox(append([]fyne.CanvasObject{
			widget.NewButton("Create", func() {
				a.editDNSRecord(-1, &types.DNSRecord{Domain: "yourdomain.com", IP: []string{"127.0.0.1"}, Wildcard: true})
			}),
		}, records...)...)),
		a.dnsListCard("Block lists", "External lists of domains that will be blocked.", "DNSBlockLists", cfg.DNSBlockLists, "blocklist"),
		a.dnsListCard("White lists", "Domains here always resolve, even if they appear on a block list.", "DNSWhiteLists", cfg.DNSWhiteLists, "whitelist"),
	)
}

func (a *App) dnsListCard(title, desc, key string, lists []*client.BlockList, kind string) fyne.CanvasObject {
	rows := []fyne.CanvasObject{}
	enableAll := widget.NewButton("Enable all", func() { a.setAllLists(key, true) })
	disableAll := widget.NewButton("Disable all", func() { a.setAllLists(key, false) })
	update := widget.NewButton("Update", func() {
		a.note("Updating lists...")
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
	})
	create := widget.NewButton("Create", func() {
		a.editDNSList(key, -1, &client.BlockList{Tag: "new-list", URL: "https://example.com/list.txt", Enabled: true})
	})
	for i, l := range lists {
		i, l := i, l
		if l == nil {
			continue
		}
		custom := strings.EqualFold(l.Tag, "custom")
		on := bindCheck("On", l.Enabled, func(v bool) {
			next := client.CloneConfig()
			src := next.DNSBlockLists
			if key == "DNSWhiteLists" {
				src = next.DNSWhiteLists
			}
			if i < len(src) && src[i] != nil {
				cp := *src[i]
				cp.Enabled = v
				src[i] = &cp
			}
			if key == "DNSWhiteLists" {
				next.DNSWhiteLists = src
			} else {
				next.DNSBlockLists = src
			}
			a.saveConfig(next)
			a.rebuild()
		})
		btns := []fyne.CanvasObject{on}
		if custom {
			btns = append(btns, widget.NewButton("Edit domains", func() { a.editCustomList(kind) }))
		} else {
			btns = append(btns,
				widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() { a.editDNSList(key, i, l) }),
				widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { a.deleteListItem(key, i) }),
			)
		}
		rows = append(rows, container.NewBorder(nil, nil, nil, container.NewHBox(btns...),
			container.NewVBox(widget.NewLabel(l.Tag), muted(fmt.Sprintf("%d domains", l.Count)))))
	}
	if len(rows) == 0 {
		rows = append(rows, muted("None configured"))
	}
	return card(title, desc, container.NewVBox(append([]fyne.CanvasObject{
		container.NewHBox(enableAll, disableAll, update, create),
	}, rows...)...))
}

func (a *App) setAllLists(key string, enabled bool) {
	next := client.CloneConfig()
	src := next.DNSBlockLists
	if key == "DNSWhiteLists" {
		src = next.DNSWhiteLists
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
		next.DNSWhiteLists = out
	} else {
		next.DNSBlockLists = out
	}
	a.saveConfig(next)
	a.rebuild()
}

func (a *App) deleteListItem(key string, index int) {
	next := client.CloneConfig()
	src := next.DNSBlockLists
	if key == "DNSWhiteLists" {
		src = next.DNSWhiteLists
	}
	if index < 0 || index >= len(src) {
		return
	}
	src = append(src[:index], src[index+1:]...)
	if key == "DNSWhiteLists" {
		next.DNSWhiteLists = src
	} else {
		next.DNSBlockLists = src
	}
	a.saveConfig(next)
	a.rebuild()
}

func (a *App) editDNSList(key string, index int, list *client.BlockList) {
	tag := widget.NewEntry()
	tag.SetText(list.Tag)
	url := widget.NewEntry()
	url.SetText(list.URL)
	en := bindCheck("Enabled", list.Enabled, nil)
	form := container.NewVBox(labeled("Tag", tag), labeled("URL", url), en)
	dialog.ShowCustomConfirm("List", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		cp := *list
		cp.Tag = strings.TrimSpace(tag.Text)
		cp.URL = strings.TrimSpace(url.Text)
		cp.Enabled = en.Checked
		next := client.CloneConfig()
		src := next.DNSBlockLists
		if key == "DNSWhiteLists" {
			src = next.DNSWhiteLists
		}
		src = append([]*client.BlockList(nil), src...)
		if index >= 0 && index < len(src) {
			src[index] = &cp
		} else {
			src = append(src, &cp)
		}
		if key == "DNSWhiteLists" {
			next.DNSWhiteLists = src
		} else {
			next.DNSBlockLists = src
		}
		a.saveConfig(next)
		a.rebuild()
	}, a.win)
}

func (a *App) editDNSRecord(index int, rec *types.DNSRecord) {
	domain := widget.NewEntry()
	domain.SetText(rec.Domain)
	ips := widget.NewMultiLineEntry()
	ips.SetText(strings.Join(rec.IP, "\n"))
	txt := widget.NewMultiLineEntry()
	txt.SetText(strings.Join(rec.TXT, "\n"))
	wild := bindCheck("Wildcard", rec.Wildcard, nil)
	form := container.NewVBox(labeled("Domain", domain), labeled("IPs", ips), labeled("TXT", txt), wild)
	dialog.ShowCustomConfirm("DNS record", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		cp := types.DNSRecord{
			Domain:   strings.TrimSpace(domain.Text),
			Wildcard: wild.Checked,
			IP:       splitLines(ips.Text),
			TXT:      splitLines(txt.Text),
		}
		next := client.CloneConfig()
		list := append([]*types.DNSRecord(nil), next.DNSRecords...)
		if index >= 0 && index < len(list) {
			list[index] = &cp
		} else {
			list = append(list, &cp)
		}
		next.DNSRecords = list
		a.saveConfig(next)
		a.rebuild()
	}, a.win)
}

func (a *App) editCustomList(kind string) {
	a.note("Loading list...")
	go func() {
		data, err := client.GetDNSListContent(kind)
		a.uiDo(func() {
			if err != nil {
				a.fail(err.Error())
				return
			}
			area := widget.NewMultiLineEntry()
			area.SetText(data.Content)
			area.SetMinRowsVisible(16)
			d := dialog.NewCustomConfirm("Edit custom "+kind, "Save", "Cancel", container.NewScroll(area), func(ok bool) {
				if !ok {
					return
				}
				a.note("Saving list...")
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
			d.Resize(fyne.NewSize(640, 480))
			d.Show()
		})
	}()
}
