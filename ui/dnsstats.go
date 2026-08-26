package ui

import (
	"fmt"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

func statsTable() *tableSpec {
	return &tableSpec{
		cols: []tableCol{
			{label: "DOMAIN", weight: 3, mono: true, strong: true},
			{label: "LIST", weight: 1.6, optional: true},
			{label: "QUERIES", weight: 1, mono: true, align: fyne.TextAlignTrailing},
			{label: "LAST SEEN", weight: 1.3, mono: true, align: fyne.TextAlignTrailing},
		},
	}
}

func (a *App) dnsStatsPage() fyne.CanvasObject {
	if !a.advanced {
		return pageShell("DNS statistics", "", nil,
			emptyState("Advanced mode required", "Turn on Advanced mode in Settings to view DNS statistics."))
	}
	a.dnsStats = client.GetDNSStats()

	blocked := a.dnsStatsTab != "resolved"
	items := a.statItems(blocked)

	_, search := searchField("Filter domains", a.dnsStatsFilter, func(s string) {
		a.dnsStatsFilter = s
	}, func(s string) {
		a.dnsStatsFilter = s
		a.reloadCurrent()
	})
	tabs := newSegmented([]segItem{
		{"blocked", "Blocked"},
		{"resolved", "Resolved"},
	}, a.dnsStatsTab, func(key string) {
		a.dnsStatsTab = key
		a.reloadCurrent()
	})
	refresh := newIconBtn(theme.ViewRefreshIcon(), kOutline, func() {
		a.reloadCurrent()
	})
	actions := hstackFlex(sp2, 0, search, tabs, refresh)

	nBlocked, nResolved, hits := a.statTotals()
	summary := hstack(sp3,
		statTile("Blocked domains", fmt.Sprintf("%d", nBlocked), toneDanger),
		statTile("Resolved domains", fmt.Sprintf("%d", nResolved), toneSuccess),
		statTile("Total queries", fmt.Sprintf("%d", hits), toneNeutral),
	)
	head := insetEach(sp4, gutter, sp4, gutter, summary)

	if len(items) == 0 {
		msg := "No blocked domains recorded"
		if !blocked {
			msg = "No resolved domains recorded"
		}
		desc := "Enable Collect statistics on the Resolver page to gather data."
		if a.dnsStatsFilter != "" {
			msg, desc = "No matching domains", "Try a different filter."
		}
		return pageShellFlush("DNS statistics", "Queries seen by the local resolver", actions,
			container.NewBorder(head, nil, nil, nil, emptyState(msg, desc)))
	}

	spec := statsTable()
	list := newRowList(spec,
		func() int { return len(items) },
		func(id widget.ListItemID, row *kRow) {
			if id < 0 || id >= len(items) {
				return
			}
			it := items[id]
			when := it.stat.LastResolved
			if blocked {
				when = it.stat.LastBlocked
			}
			src := it.stat.Tag
			if src == "" {
				src = "—"
			}
			row.SetCells([]string{
				it.domain,
				src,
				fmt.Sprintf("%d", it.stat.Count),
				fmtTimeShort(when),
			}, false, toneNeutral)
		},
	)

	return pageShellFlush("DNS statistics", "Queries seen by the local resolver", actions,
		container.NewBorder(head, nil, nil, nil, tableBody(spec, list)))
}

type statItem struct {
	domain string
	stat   *client.DNSStats
}

func (a *App) statItems(blocked bool) []statItem {
	var items []statItem
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
		items = append(items, statItem{d, s})
	}
	sort.Slice(items, func(i, j int) bool {
		if blocked {
			return items[i].stat.LastBlocked.After(items[j].stat.LastBlocked)
		}
		return items[i].stat.LastResolved.After(items[j].stat.LastResolved)
	})
	return items
}

func (a *App) statTotals() (blocked, resolved, hits int) {
	for _, s := range a.dnsStats {
		if s == nil {
			continue
		}
		if !s.LastBlocked.IsZero() {
			blocked++
		}
		if !s.LastResolved.IsZero() {
			resolved++
		}
		hits += s.Count
	}
	return
}
