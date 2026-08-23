package ui

import (
	"fmt"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) dnsStatsPage() fyne.CanvasObject {
	if !a.advanced {
		return container.NewCenter(muted("Enable Advanced mode in Settings to view DNS stats."))
	}
	if a.dnsStats == nil {
		a.dnsStats = client.GetDNSStats()
	}

	refresh := outlineBtn("Refresh", func() {
		a.dnsStats = client.GetDNSStats()
		a.rebuild()
	})
	filter := widget.NewEntry()
	filter.SetPlaceHolder("Filter domains...")
	filter.SetText(a.dnsStatsFilter)
	filter.OnChanged = func(s string) { a.dnsStatsFilter = s }
	filter.OnSubmitted = func(s string) {
		a.dnsStatsFilter = s
		a.rebuild()
	}

	tabs := container.NewAppTabs(
		container.NewTabItem("Blocked", a.dnsStatsTable(true)),
		container.NewTabItem("Resolved", a.dnsStatsTable(false)),
	)
	if a.dnsStatsTab == "resolved" {
		tabs.SelectIndex(1)
	}
	tabs.OnSelected = func(t *container.TabItem) {
		if t.Text == "Resolved" {
			a.dnsStatsTab = "resolved"
		} else {
			a.dnsStatsTab = "blocked"
		}
	}

	return container.NewBorder(
		container.NewPadded(container.NewVBox(toolbar(widget.NewLabel("DNS stats"), refresh), filter)),
		nil, nil, nil,
		tabs,
	)
}

func (a *App) dnsStatsTable(blocked bool) fyne.CanvasObject {
	type row struct {
		domain string
		stat   *client.DNSStats
	}
	var items []row
	for d, s := range a.dnsStats {
		if s == nil {
			continue
		}
		if blocked && s.LastBlocked.IsZero() {
			continue
		}
		if !blocked && s.LastResolved.IsZero() {
			continue
		}
		if !filterMatch(a.dnsStatsFilter, d) {
			continue
		}
		items = append(items, row{d, s})
	}
	sort.Slice(items, func(i, j int) bool {
		if blocked {
			return items[i].stat.LastBlocked.After(items[j].stat.LastBlocked)
		}
		return items[i].stat.LastResolved.After(items[j].stat.LastResolved)
	})
	if len(items) == 0 {
		return container.NewCenter(muted("No data"))
	}
	list := widget.NewList(
		func() int { return len(items) },
		func() fyne.CanvasObject {
			return container.NewVBox(widget.NewLabel("domain"), muted("meta"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(items) {
				return
			}
			it := items[id]
			box := obj.(*fyne.Container)
			title := box.Objects[0].(*widget.Label)
			meta := box.Objects[1].(*widget.Label)
			title.SetText(it.domain)
			when := it.stat.LastResolved
			extra := ""
			if blocked {
				when = it.stat.LastBlocked
				extra = it.stat.Tag + "  ·  "
			}
			meta.SetText(fmt.Sprintf("%s%d  ·  %s", extra, it.stat.Count, fmtTimeShort(when)))
		},
	)
	return list
}
