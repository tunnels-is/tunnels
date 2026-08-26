package ui

import (
	"fmt"
	"image/color"
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ---------------------------------------------------------------- log row

// logRow renders one parsed log line as timestamp / level / message columns.
// A list of these replaces the editable multiline entry the log used to be:
// levels can be colour-coded and the text is monospaced and read-only.
type logRow struct {
	widget.BaseWidget
	when  string
	level string
	fn    string
	msg   string
	list  *widget.List
	id    widget.ListItemID

	wrapW     float32
	wrapLines []string
	prefixW   float32
}

const maxLogWrapLines = 8

func newLogRow() *logRow {
	r := &logRow{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *logRow) set(id widget.ListItemID, list *widget.List, when, level, fn, msg string) {
	if r.id == id && r.list == list && r.when == when && r.level == level && r.fn == fn && r.msg == msg {
		return
	}
	if r.when != when || r.fn != fn {
		r.prefixW = 0
	}
	if r.msg != msg || r.when != when || r.fn != fn {
		r.wrapLines = nil
		r.wrapW = 0
	}
	r.id, r.list = id, list
	r.when, r.level, r.fn, r.msg = when, level, fn, msg
	r.Refresh()
}

func logMsgStyle() fyne.TextStyle { return fyne.TextStyle{Monospace: true} }

func logLineHeight() float32 {
	return cachedLineHeight(fsSmall, logMsgStyle())
}

func (r *logRow) prefixWidth() float32 {
	if r.prefixW > 0 {
		return r.prefixW
	}
	w := float32(0)
	if r.when != "" {
		w += measureWidth(r.when, fsCaption, fyne.TextStyle{Monospace: true}) + sp3
	}
	r.prefixW = w + logLevelCol + logFnCol
	return r.prefixW
}

func (r *logRow) wrapped(width float32) []string {
	remain := width - r.prefixWidth()
	if remain < z(64) {
		remain = z(64)
	}
	if r.wrapLines != nil && r.wrapW == remain {
		return r.wrapLines
	}
	lines := wrapToWidth(r.msg, remain, fsSmall, logMsgStyle())
	if len(lines) > maxLogWrapLines {
		lines = lines[:maxLogWrapLines]
	}
	r.wrapW = remain
	r.wrapLines = lines
	return lines
}

func (r *logRow) heightFor(width float32) float32 {
	n := len(r.wrapped(width))
	if n < 1 {
		n = 1
	}
	return logLineHeight()*float32(n) + 2
}

func (r *logRow) CreateRenderer() fyne.WidgetRenderer {
	p := pal()
	d := &logRowRenderer{
		r:     r,
		when:  monoText("", fsCaption, p.Faint),
		level: monoText("", fsCaption, p.Muted),
		fn:    monoText("", fsCaption, p.Muted),
	}
	d.level.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	d.apply()
	return d
}

type logRowRenderer struct {
	r     *logRow
	when  *canvas.Text
	level *canvas.Text
	fn    *canvas.Text
	msgs  []*canvas.Text
	objs  []fyne.CanvasObject

	laidW    float32
	laidWhen string
	laidMsg  string
	laidFn   string
	laidLvl  string
}

func (d *logRowRenderer) Destroy() {}

func (d *logRowRenderer) Objects() []fyne.CanvasObject {
	if d.objs == nil {
		d.rebuildObjs()
	}
	return d.objs
}

func (d *logRowRenderer) rebuildObjs() {
	n := 3 + len(d.msgs)
	if cap(d.objs) < n {
		d.objs = make([]fyne.CanvasObject, 0, n)
	} else {
		d.objs = d.objs[:0]
	}
	d.objs = append(d.objs, d.when, d.level, d.fn)
	for _, t := range d.msgs {
		d.objs = append(d.objs, t)
	}
}

func (d *logRowRenderer) ensureMsgs(n int) {
	p := pal()
	grew := false
	for len(d.msgs) < n {
		d.msgs = append(d.msgs, monoText("", fsSmall, p.Content))
		grew = true
	}
	if grew || d.objs == nil {
		d.rebuildObjs()
	}
}

func (d *logRowRenderer) MinSize() fyne.Size {
	w := d.r.Size().Width
	h := logLineHeight() + 2
	if w > 0 {
		h = d.r.heightFor(w)
	}
	return fyne.NewSize(z(400), h)
}

func (d *logRowRenderer) Layout(size fyne.Size) {
	same := d.laidW == size.Width && d.laidWhen == d.r.when && d.laidMsg == d.r.msg &&
		d.laidFn == d.r.fn && d.laidLvl == d.r.level
	if same {
		return
	}
	d.laidW, d.laidWhen, d.laidMsg = size.Width, d.r.when, d.r.msg
	d.laidFn, d.laidLvl = d.r.fn, d.r.level

	setCanvasText(d.when, d.r.when, d.when.TextSize, d.when.TextStyle, d.when.Color)
	setCanvasText(d.level, d.r.level, d.level.TextSize, d.level.TextStyle, d.level.Color)
	fn := elide(d.r.fn, logFnCol-sp3, d.fn.TextSize, d.fn.TextStyle)
	setCanvasText(d.fn, fn, d.fn.TextSize, d.fn.TextStyle, d.fn.Color)

	x := d.r.prefixWidth()
	whenW := x - logLevelCol - logFnCol
	d.when.Move(fyne.NewPos(0, 0))
	d.level.Move(fyne.NewPos(whenW, 0))
	d.fn.Move(fyne.NewPos(whenW+logLevelCol, 0))

	lines := d.r.wrapped(size.Width)
	d.ensureMsgs(len(lines))
	lh := logLineHeight()
	fg := pal().Content
	if d.r.level == "ERROR" {
		fg = pal().Error
	}
	style := logMsgStyle()
	cw := monoCharWidth(fsSmall, style)
	for i, t := range d.msgs {
		if i >= len(lines) {
			if t.Text != "" {
				t.Text = ""
				t.Hide()
			}
			t.Move(fyne.NewPos(0, 0))
			t.Resize(fyne.NewSize(0, 0))
			continue
		}
		t.Show()
		setCanvasText(t, lines[i], fsSmall, style, fg)
		t.Move(fyne.NewPos(x, lh*float32(i)))
		w := t.Size().Width
		if cw > 0 {
			w = cw * float32(utf8.RuneCountInString(lines[i]))
		} else {
			w = t.MinSize().Width
		}
		t.Resize(fyne.NewSize(w, lh))
	}
}

func (d *logRowRenderer) Refresh() {
	d.apply()
	d.laidW = 0
	if sz := d.r.Size(); sz.Width > 0 {
		d.Layout(sz)
	}
	canvasRefresh(d.r)
}

func (d *logRowRenderer) apply() {
	p := pal()
	d.when.Text = d.r.when
	d.when.Color = p.Faint
	d.level.Text = d.r.level
	d.level.Color = logLevelColor(d.r.level)
	d.fn.Color = p.Faint
}

func logLevelColor(level string) color.Color {
	p := pal()
	switch level {
	case "ERROR":
		return p.Error
	case "INFO":
		return p.Info
	case "DEBUG":
		return p.Muted
	case "WARN", "WARNING":
		return p.Warning
	case "SECURITY":
		return p.Warning
	default:
		return p.Faint
	}
}

// splitLogLine pulls apart the client's log format, which is
// "<time> || <LEVEL> || <func> || <message>" (see client/logging.go). Shorter
// lines are tolerated, and any remaining " || " stays in the message so a
// payload that happens to contain the separator is not truncated.
func splitLogLine(line string) (when, level, fn, msg string) {
	parts := strings.Split(line, " || ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	switch len(parts) {
	case 1:
		return "", "", "", parts[0]
	case 2:
		return parts[0], "", "", parts[1]
	case 3:
		return parts[0], parts[1], "", parts[2]
	default:
		return parts[0], parts[1], parts[2], strings.Join(parts[3:], " || ")
	}
}

// ---------------------------------------------------------------- page

func (a *App) recomputeLogView() {
	a.logView = a.filteredLogs()
}

var logTags = []segItem{
	{"", "All"}, {"INFO", "Info"}, {"ERROR", "Errors"},
	{"SECURITY", "Security"}, {"DEBUG", "Debug"}, {"ROUTINE", "Routine"},
}

func (a *App) logsPage() fyne.CanvasObject {
	_, search := searchField("Filter log lines", a.filterLogs, func(s string) {
		a.filterLogs = s
	}, func(s string) {
		a.filterLogs = s
		a.reloadCurrent()
	})

	tags := newSegmented(logTags, a.logTag, func(key string) {
		a.logTag = key
		a.reloadCurrent()
	})

	clear := newIconBtn(theme.DeleteIcon(), kDanger, func() {
		a.confirm("Clear logs", "Remove all captured log lines from this view?", func() {
			a.logs = nil
			a.reloadCurrent()
		})
	})

	a.recomputeLogView()
	sub := fmt.Sprintf("%d lines", len(a.logView))
	actions := hstackFlex(sp2, 0, search, tags, clear)

	if len(a.logView) == 0 {
		msg, desc := "Nothing captured yet", "Log lines will appear here as the client runs."
		if a.filterLogs != "" || a.logTag != "" {
			msg, desc = "No matching lines", "Try a different filter or level."
		}
		return pageShell("Logs", sub, actions, emptyState(msg, desc))
	}

	a.logList = widget.NewList(
		func() int { return len(a.logView) },
		func() fyne.CanvasObject { return newLogRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row, ok := obj.(*logRow)
			if !ok || id < 0 || id >= len(a.logView) {
				return
			}
			when, level, fn, msg := splitLogLine(a.logView[id])
			row.set(id, a.logList, when, level, fn, msg)
			a.queueLogHeight(id, row)
		},
	)
	a.logList.HideSeparators = true

	boost := &scrollBoost{list: a.logList}
	boost.ExtendBaseWidget(boost)
	panel := container.NewStack(
		surface(radLg, pal().Base100, pal().Base300),
		insetXY(sp4, sp2, container.NewStack(
			container.NewThemeOverride(a.logList, compactLogTheme{live}),
			boost,
		)),
	)
	return pageShell("Logs", sub, actions, insetEach(sp4, gutter, gutter, gutter, panel))
}

func (a *App) queueLogHeight(id widget.ListItemID, row *logRow) {
	if a.settingLogHeight || a.logList == nil || row == nil {
		return
	}
	w := row.Size().Width
	if w <= 0 {
		w = a.logList.Size().Width
	}
	if w <= 0 {
		return
	}
	want := row.heightFor(w)
	delta := want - row.Size().Height
	if delta < 0 {
		delta = -delta
	}
	if delta < 0.5 {
		return
	}
	if a.logHeights == nil {
		a.logHeights = make(map[widget.ListItemID]float32)
	}
	a.logHeights[id] = want
	if a.logHeightQueued {
		return
	}
	a.logHeightQueued = true
	fyne.Do(func() {
		a.flushLogHeights()
	})
}

func (a *App) flushLogHeights() {
	pending := a.logHeights
	a.logHeights = nil
	a.logHeightQueued = false
	if a.logList == nil || a.settingLogHeight || len(pending) == 0 {
		return
	}
	a.settingLogHeight = true
	defer func() { a.settingLogHeight = false }()
	for id, h := range pending {
		a.logList.SetItemHeight(id, h)
	}
}

func (a *App) filteredLogs() []string {
	out := make([]string, 0, len(a.logs))
	q := strings.ToLower(a.filterLogs)
	for i := len(a.logs) - 1; i >= 0; i-- {
		line := a.logs[i]
		if q != "" && !strings.Contains(strings.ToLower(line), q) {
			continue
		}
		_, tag, _, _ := splitLogLine(line)
		if a.logTag != "" {
			if tag != a.logTag {
				continue
			}
		} else if tag == "ROUTINE" {
			continue
		}
		out = append(out, line)
	}
	return out
}

// compactLogTheme drops the theme padding widget.List inserts between items,
// which is what made log lines look double-spaced.
type compactLogTheme struct{ fyne.Theme }

func (t compactLogTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNamePadding {
		return 1
	}
	return t.Theme.Size(n)
}
