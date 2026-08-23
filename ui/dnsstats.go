package ui

import (
	"fmt"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

// statRow is one domain line: domain, source list, hit count, last seen.
type statRow struct {
	widget.BaseWidget
	domain string
	source string
	count  string
	when   string
	tone   tone
}

func newStatRow() *statRow {
	r := &statRow{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *statRow) set(domain, source, count, when string, t tone) {
	r.domain, r.source, r.count, r.when, r.tone = domain, source, count, when, t
	r.Refresh()
}

func (r *statRow) CreateRenderer() fyne.WidgetRenderer {
	p := pal()
	d := &statRowRenderer{
		r:      r,
		dot:    canvas.NewCircle(p.Faint),
		domain: monoText("", fsBody, p.Content),
		source: text("", fsCaption, p.Faint, false),
		count:  monoText("", fsSmall, p.Muted),
		when:   text("", fsCaption, p.Faint, false),
		line:   canvas.NewRectangle(p.Divider),
	}
	d.count.Alignment = fyne.TextAlignTrailing
	d.when.Alignment = fyne.TextAlignTrailing
	d.apply()
	return d
}

type statRowRenderer struct {
	r      *statRow
	dot    *canvas.Circle
	domain *canvas.Text
	source *canvas.Text
	count  *canvas.Text
	when   *canvas.Text
	line   *canvas.Rectangle
}

func (d *statRowRenderer) Destroy() {}

func (d *statRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{d.line, d.dot, d.domain, d.source, d.count, d.when}
}

func (d *statRowRenderer) MinSize() fyne.Size { return fyne.NewSize(z(420), z(44)) }

func (d *statRowRenderer) Layout(size fyne.Size) {
	d.line.Resize(fyne.NewSize(size.Width, 1))
	d.line.Move(fyne.NewPos(0, size.Height-1))
	d.dot.Resize(fyne.NewSize(z(6), z(6)))
	d.dot.Move(fyne.NewPos(0, (size.Height-z(6))/2))

	x := sp3 + z(3)
	dms := d.domain.MinSize()
	sms := d.source.MinSize()
	if d.r.source == "" {
		d.domain.Move(fyne.NewPos(x, (size.Height-dms.Height)/2))
		d.source.Move(fyne.NewPos(x, size.Height))
	} else {
		block := dms.Height + sms.Height
		top := (size.Height - block) / 2
		d.domain.Move(fyne.NewPos(x, top))
		d.source.Move(fyne.NewPos(x, top+dms.Height))
	}

	wms := d.when.MinSize()
	d.when.Resize(fyne.NewSize(wms.Width, wms.Height))
	d.when.Move(fyne.NewPos(size.Width-wms.Width, (size.Height-wms.Height)/2))
	cms := d.count.MinSize()
	d.count.Resize(fyne.NewSize(cms.Width, cms.Height))
	d.count.Move(fyne.NewPos(size.Width-wms.Width-sp5-cms.Width, (size.Height-cms.Height)/2))
}

func (d *statRowRenderer) Refresh() {
	d.apply()
	for _, o := range d.Objects() {
		o.Refresh()
	}
	if sz := d.r.Size(); sz.Width > 0 {
		d.Layout(sz)
	}
	canvasRefresh(d.r)
}

func (d *statRowRenderer) apply() {
	p := pal()
	fg, _ := toneColors(d.r.tone)
	d.dot.FillColor = fg
	d.domain.Text = d.r.domain
	d.domain.Color = p.Content
	d.source.Text = d.r.source
	d.source.Color = p.Faint
	d.count.Text = d.r.count
	d.count.Color = p.Muted
	d.when.Text = d.r.when
	d.when.Color = p.Faint
	d.line.FillColor = p.Divider
}

// ---------------------------------------------------------------- page

func (a *App) dnsStatsPage() fyne.CanvasObject {
	if !a.advanced {
		return pageShell("DNS statistics", "", nil,
			emptyState("Advanced mode required", "Turn on Advanced mode in Settings to view DNS statistics."))
	}
	if a.dnsStats == nil {
		a.dnsStats = client.GetDNSStats()
	}

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
		a.dnsStats = client.GetDNSStats()
		a.reloadCurrent()
	})
	actions := hstack(sp2, search, tabs, refresh)

	nBlocked, nResolved, hits := a.statTotals()
	summary := hstack(sp3,
		statTile("Blocked domains", fmt.Sprintf("%d", nBlocked), toneDanger),
		statTile("Resolved domains", fmt.Sprintf("%d", nResolved), toneSuccess),
		statTile("Total queries", fmt.Sprintf("%d", hits), toneNeutral),
	)

	var listing fyne.CanvasObject
	if len(items) == 0 {
		msg := "No blocked domains recorded"
		if !blocked {
			msg = "No resolved domains recorded"
		}
		desc := "Enable Collect statistics on the Resolver page to gather data."
		if a.dnsStatsFilter != "" {
			msg, desc = "No matching domains", "Try a different filter."
		}
		listing = inset(sp8, emptyState(msg, desc))
	} else {
		list := widget.NewList(
			func() int { return len(items) },
			func() fyne.CanvasObject { return newStatRow() },
			func(id widget.ListItemID, obj fyne.CanvasObject) {
				row, ok := obj.(*statRow)
				if !ok || id < 0 || id >= len(items) {
					return
				}
				it := items[id]
				when, t := it.stat.LastResolved, toneSuccess
				if blocked {
					when, t = it.stat.LastBlocked, toneDanger
				}
				row.set(it.domain, it.stat.Tag, fmt.Sprintf("%d×", it.stat.Count), fmtTimeShort(when), t)
			},
		)
		list.HideSeparators = true
		listing = container.NewStack(
			surface(radLg, pal().Base100, pal().Base300),
			insetXY(sp4, sp2, list),
		)
	}

	head := insetEach(sp5, gutter, sp4, gutter, summary)
	body := container.NewBorder(head, nil, nil, nil,
		insetEach(0, gutter, gutter, gutter, listing))

	return pageShell("DNS statistics", "Queries seen by the local resolver", actions, body)
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
