package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

// bwRange is one selectable window over the history.
type bwRange struct {
	label   string
	seconds int
}

// The client samples once a second and keeps MaxBandwidthRecords (86400) of
// them, so 24h is the whole buffer. The web UI also offers 7d, which can never
// show more than a day of data, so that option is left out rather than showing
// a range that silently tops out.
var bwRanges = []bwRange{
	{"1m", 60},
	{"5m", 300},
	{"15m", 900},
	{"1h", 3600},
	{"6h", 21600},
	{"24h", 86400},
}

const bwRangePrefKey = "dash-bw-range"

func (a *App) bwRangeSeconds() int {
	if a.bwRange <= 0 {
		a.bwRange = int(a.fyneApp.Preferences().FloatWithFallback(bwRangePrefKey, 60))
	}
	for _, r := range bwRanges {
		if r.seconds == a.bwRange {
			return r.seconds
		}
	}
	return 60
}

func (a *App) setBWRange(sec int) {
	a.bwRange = sec
	a.fyneApp.Preferences().SetFloat(bwRangePrefKey, float64(sec))
	a.reloadCurrent()
}

// bwRangePicker is the shared window selector. One control drives every chart,
// as in the web UI, rather than a copy per tunnel.
func (a *App) bwRangePicker() fyne.CanvasObject {
	cur := a.bwRangeSeconds()
	items := make([]segItem, 0, len(bwRanges))
	for _, r := range bwRanges {
		items = append(items, segItem{key: r.label, label: r.label})
	}
	active := ""
	for _, r := range bwRanges {
		if r.seconds == cur {
			active = r.label
		}
	}
	return newSegmented(items, active, func(key string) {
		for _, r := range bwRanges {
			if r.label == key {
				a.setBWRange(r.seconds)
				return
			}
		}
	})
}

// ---------------------------------------------------------------- aggregation

// bucketSizeFor mirrors the web UI's thinning table: long windows are averaged
// into buckets so the chart stays legible instead of drawing 86400 bars.
func bucketSizeFor(rangeSeconds int) int {
	switch {
	case rangeSeconds <= 300:
		return 1
	case rangeSeconds <= 3600:
		return 10
	case rangeSeconds <= 21600:
		return 60
	case rangeSeconds <= 86400:
		return 240
	default:
		return 1800
	}
}

// bwStats is one direction's summary over the selected window.
type bwStats struct {
	current, avg, peak, total int64
}

// summarise computes stats from the raw samples, not the drawing buckets:
// averaging first would report a peak-of-averages and understate real spikes.
func summarise(recs []client.BandwidthRecord) (down, up bwStats) {
	if len(recs) == 0 {
		return
	}
	for _, r := range recs {
		down.total += r.IngressBytes
		up.total += r.EgressBytes
		if r.IngressBytes > down.peak {
			down.peak = r.IngressBytes
		}
		if r.EgressBytes > up.peak {
			up.peak = r.EgressBytes
		}
	}
	n := int64(len(recs))
	down.avg, up.avg = down.total/n, up.total/n
	last := recs[len(recs)-1]
	down.current, up.current = last.IngressBytes, last.EgressBytes
	return
}

// maxChartBars caps how many bars are drawn. The reference thinning table alone
// still yields 360 bars for a 6h window, which at typical card widths is under
// 2px each and reads as a solid block rather than a graph.
const maxChartBars = 140

// displayBuckets thins the window down to something drawable: the reference
// bucket size first, then a further pass if that still leaves too many bars.
func displayBuckets(recs []client.BandwidthRecord, rangeSeconds int) []client.BandwidthRecord {
	size := bucketSizeFor(rangeSeconds)
	if n := len(recs); n > maxChartBars*size {
		size = (n + maxChartBars - 1) / maxChartBars
	}
	return bucket(recs, size)
}

// bucket averages consecutive samples so the chart has one bar per bucket.
func bucket(recs []client.BandwidthRecord, size int) []client.BandwidthRecord {
	if size <= 1 || len(recs) == 0 {
		return recs
	}
	out := make([]client.BandwidthRecord, 0, len(recs)/size+1)
	for i := 0; i < len(recs); i += size {
		end := i + size
		if end > len(recs) {
			end = len(recs)
		}
		slice := recs[i:end]
		var ig, eg int64
		for _, r := range slice {
			ig += r.IngressBytes
			eg += r.EgressBytes
		}
		n := int64(len(slice))
		out = append(out, client.BandwidthRecord{
			Timestamp:    slice[len(slice)/2].Timestamp,
			IngressBytes: ig / n,
			EgressBytes:  eg / n,
		})
	}
	return out
}

// ---------------------------------------------------------------- panel

// bandwidthPanel draws live throughput for one tunnel: download above the
// centre line, upload below, with the full stat grid underneath.
//
// It owns everything it shows and re-reads the tunnel's history once a second.
// One self-updating widget avoids rebuilding the page just to move a graph.
type bandwidthPanel struct {
	widget.BaseWidget
	tun     *client.TUN
	seconds int
}

func newBandwidthPanel(t *client.TUN, seconds int) *bandwidthPanel {
	p := &bandwidthPanel{tun: t, seconds: seconds}
	p.ExtendBaseWidget(p)
	return p
}

// samples returns the records inside the selected window.
func (p *bandwidthPanel) samples() []client.BandwidthRecord {
	if p.tun == nil {
		return nil
	}
	bh := p.tun.BandwidthHistory.Load()
	if bh == nil {
		return nil
	}
	all := bh.Snapshot()
	cutoff := time.Now().Add(-time.Duration(p.seconds) * time.Second)
	start := 0
	for i, r := range all {
		if r.Timestamp.After(cutoff) {
			start = i
			break
		}
		start = i + 1
	}
	return all[start:]
}

var bwStatCols = []string{"NOW", "AVG", "PEAK", "TOTAL"}

func (p *bandwidthPanel) CreateRenderer() fyne.WidgetRenderer {
	pl := pal()
	r := &bwRenderer{
		p:      p,
		mid:    canvas.NewRectangle(pl.Divider),
		rule:   canvas.NewRectangle(pl.Divider),
		down:   text("", fsLarge, pl.Success, true),
		up:     text("", fsLarge, pl.Primary, true),
		window: text("", fsCaption, pl.Faint, false),
		empty:  text("", fsSmall, pl.Faint, false),
		dLabel: text("DOWNLOAD", fsCaption, pl.Success, true),
		uLabel: text("UPLOAD", fsCaption, pl.Primary, true),
		ticker: time.NewTicker(time.Second),
		stop:   make(chan struct{}),
	}
	for range bwStatCols {
		r.heads = append(r.heads, text("", fsCaption, pl.Faint, true))
		r.dCells = append(r.dCells, monoText("", fsSmall, pl.Content))
		r.uCells = append(r.uCells, monoText("", fsSmall, pl.Content))
	}
	r.refreshData()
	go r.run()
	return r
}

type bwRenderer struct {
	p    *bandwidthPanel
	mid  *canvas.Rectangle
	rule *canvas.Rectangle
	bars []*canvas.Rectangle // 2 per bucket: download then upload

	down, up       *canvas.Text
	window, empty  *canvas.Text
	dLabel, uLabel *canvas.Text
	heads          []*canvas.Text
	dCells, uCells []*canvas.Text

	buckets []client.BandwidthRecord
	ticker  *time.Ticker
	stop    chan struct{}
	closed  bool
}

func (r *bwRenderer) run() {
	for {
		select {
		case <-r.stop:
			return
		case <-r.ticker.C:
			// Same guard as the clock: if the widget is detached without
			// Destroy being called, stop rather than wake the UI forever.
			app := fyne.CurrentApp()
			if app == nil || app.Driver() == nil || app.Driver().CanvasForObject(r.p) == nil {
				r.Destroy()
				return
			}
			fyne.Do(func() {
				r.refreshData()
				if sz := r.p.Size(); sz.Width > 0 {
					r.Layout(sz)
				}
				canvasRefresh(r.p)
			})
		}
	}
}

func (r *bwRenderer) Destroy() {
	if r.closed {
		return
	}
	r.closed = true
	r.ticker.Stop()
	close(r.stop)
}

func (r *bwRenderer) refreshData() {
	pl := pal()
	recs := r.p.samples()
	r.buckets = displayBuckets(recs, r.p.seconds)

	r.mid.FillColor = pl.Divider
	r.rule.FillColor = pl.Divider

	need := len(r.buckets) * 2
	for len(r.bars) < need {
		r.bars = append(r.bars, canvas.NewRectangle(pl.Success))
	}
	for i, b := range r.bars {
		if i >= need {
			b.Resize(fyne.NewSize(0, 0))
			continue
		}
		if i%2 == 0 {
			b.FillColor = pl.Success
		} else {
			b.FillColor = pl.Primary
		}
	}

	down, up := summarise(recs)
	r.down.Text = "↓ " + client.BandwidthBytesToString(down.current) + "/s"
	r.up.Text = "↑ " + client.BandwidthBytesToString(up.current) + "/s"
	r.down.Color, r.up.Color = pl.Success, pl.Primary
	r.dLabel.Color, r.uLabel.Color = pl.Success, pl.Primary

	label := ""
	for _, br := range bwRanges {
		if br.seconds == r.p.seconds {
			label = br.label
		}
	}
	r.window.Text = fmt.Sprintf("last %s · %d samples", label, len(recs))
	r.window.Color = pl.Faint

	rate := func(v int64) string { return client.BandwidthBytesToString(v) + "/s" }
	vals := [][]string{
		{rate(down.current), rate(down.avg), rate(down.peak), client.BandwidthBytesToString(down.total)},
		{rate(up.current), rate(up.avg), rate(up.peak), client.BandwidthBytesToString(up.total)},
	}
	for i := range bwStatCols {
		r.heads[i].Text = bwStatCols[i]
		r.heads[i].Color = pl.Faint
		r.dCells[i].Text = vals[0][i]
		r.uCells[i].Text = vals[1][i]
		r.dCells[i].Color = pl.Content
		r.uCells[i].Color = pl.Content
	}

	if len(recs) == 0 {
		r.empty.Text = "No samples in this window. Bandwidth graphs must be on in Settings."
	} else {
		r.empty.Text = ""
	}
	r.empty.Color = pl.Faint
}

func (r *bwRenderer) Objects() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, len(r.bars)+len(r.heads)*3+8)
	out = append(out, r.mid, r.rule)
	for _, b := range r.bars {
		out = append(out, b)
	}
	out = append(out, r.down, r.up, r.window, r.empty, r.dLabel, r.uLabel)
	for i := range r.heads {
		out = append(out, r.heads[i], r.dCells[i], r.uCells[i])
	}
	return out
}

func (r *bwRenderer) MinSize() fyne.Size {
	rows := r.down.MinSize().Height + z(3) // rate line
	chart := z(64)
	grid := r.heads[0].MinSize().Height + r.dCells[0].MinSize().Height*2 + sp3*2
	return fyne.NewSize(z(300), rows+chart+grid+sp5*2)
}

// statCols splits the width into a label column plus four right-aligned values.
func (r *bwRenderer) statCols(width float32) (labelW float32, cols []cellRect) {
	labelW = z(84)
	avail := width - labelW
	n := float32(len(bwStatCols))
	cw := avail / n
	for i := range bwStatCols {
		cols = append(cols, cellRect{x: labelW + float32(i)*cw, w: cw - sp2})
	}
	return
}

func (r *bwRenderer) Layout(size fyne.Size) {
	// Rate readouts, with the window summary trailing.
	dms := r.down.MinSize()
	r.down.Move(fyne.NewPos(0, 0))
	r.up.Move(fyne.NewPos(dms.Width+sp5, 0))
	wms := r.window.MinSize()
	r.window.Move(fyne.NewPos(max32(0, size.Width-wms.Width), (dms.Height-wms.Height)/2))

	top := dms.Height + sp4
	gridH := r.heads[0].MinSize().Height + r.dCells[0].MinSize().Height*2 + sp3*2
	chartH := max32(z(48), size.Height-top-gridH-sp5)
	mid := top + chartH/2

	r.mid.Resize(fyne.NewSize(size.Width, z(1)))
	r.mid.Move(fyne.NewPos(0, mid))
	r.empty.Move(fyne.NewPos(0, mid+sp2))

	r.drawBars(size.Width, mid, chartH)

	// Stat grid under a rule.
	gridTop := top + chartH + sp4
	r.rule.Resize(fyne.NewSize(size.Width, z(1)))
	r.rule.Move(fyne.NewPos(0, gridTop))

	labelW, cols := r.statCols(size.Width)
	_ = labelW
	hy := gridTop + sp3
	rowH := r.dCells[0].MinSize().Height + sp2
	for i := range r.heads {
		ms := r.heads[i].MinSize()
		r.heads[i].Move(fyne.NewPos(cols[i].x+cols[i].w-ms.Width, hy))
	}
	headH := r.heads[0].MinSize().Height
	dY := hy + headH + sp2
	uY := dY + rowH

	r.dLabel.Move(fyne.NewPos(0, dY+z(1)))
	r.uLabel.Move(fyne.NewPos(0, uY+z(1)))
	for i := range cols {
		dms := r.dCells[i].MinSize()
		r.dCells[i].Move(fyne.NewPos(cols[i].x+cols[i].w-dms.Width, dY))
		ums := r.uCells[i].MinSize()
		r.uCells[i].Move(fyne.NewPos(cols[i].x+cols[i].w-ums.Width, uY))
	}
}

func (r *bwRenderer) drawBars(width, mid, chartH float32) {
	n := len(r.buckets)
	if n == 0 {
		for _, b := range r.bars {
			b.Resize(fyne.NewSize(0, 0))
		}
		return
	}

	// Scale both series against the single largest sample so download and
	// upload stay comparable rather than each filling its own half.
	var peak int64 = 1
	for _, rec := range r.buckets {
		if rec.IngressBytes > peak {
			peak = rec.IngressBytes
		}
		if rec.EgressBytes > peak {
			peak = rec.EgressBytes
		}
	}

	half := chartH/2 - z(2)
	slot := width / float32(n)
	barW := max32(z(1), slot-max32(z(1), slot*0.15))

	for i, rec := range r.buckets {
		x := float32(i) * slot
		dh := half * float32(rec.IngressBytes) / float32(peak)
		uh := half * float32(rec.EgressBytes) / float32(peak)
		if rec.IngressBytes > 0 && dh < z(1) {
			dh = z(1)
		}
		if rec.EgressBytes > 0 && uh < z(1) {
			uh = z(1)
		}
		d, u := r.bars[i*2], r.bars[i*2+1]
		d.Resize(fyne.NewSize(barW, dh))
		d.Move(fyne.NewPos(x, mid-dh))
		u.Resize(fyne.NewSize(barW, uh))
		u.Move(fyne.NewPos(x, mid+z(1)))
	}
}

func (r *bwRenderer) Refresh() {
	r.refreshData()
	if sz := r.p.Size(); sz.Width > 0 {
		r.Layout(sz)
	}
	for _, o := range r.Objects() {
		o.Refresh()
	}
}

// bandwidthCard wraps the panel with the tunnel's identity.
func (a *App) bandwidthCard(t *client.TUN) fyne.CanvasObject {
	tag := "tunnel"
	if t.CR != nil && t.CR.Tag != "" {
		tag = t.CR.Tag
	}
	where := ""
	if t.CR != nil {
		if s := a.serverByID(t.CR.ServerID); s != nil {
			where = s.Tag
			if c := countryName(s.Country); c != "" {
				where += "  ·  " + c
			}
		}
	}
	if t.ServerResponse != nil && t.ServerResponse.InterfaceIP != "" {
		if where != "" {
			where += "  ·  "
		}
		where += t.ServerResponse.InterfaceIP
	}

	return cardBox(tag, where, badge("live", toneSuccess),
		container.New(vCentreLayout{}, newBandwidthPanel(t, a.bwRangeSeconds())))
}
