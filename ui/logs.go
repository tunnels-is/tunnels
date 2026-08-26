package ui

import (
	"fmt"
	"image/color"
	"strings"

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
}

func newLogRow() *logRow {
	r := &logRow{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *logRow) set(id widget.ListItemID, list *widget.List, when, level, fn, msg string) {
	r.id, r.list = id, list
	r.when, r.level, r.fn, r.msg = when, level, fn, msg
	r.Refresh()
}

func logMsgStyle() fyne.TextStyle { return fyne.TextStyle{Monospace: true} }

func logLineHeight() float32 {
	return fyne.MeasureText("Ag", fsSmall, logMsgStyle()).Height
}

func (r *logRow) prefixWidth() float32 {
	w := float32(0)
	if r.when != "" {
		w += fyne.MeasureText(r.when, fsCaption, fyne.TextStyle{Monospace: true}).Width + sp3
	}
	return w + logLevelCol + logFnCol
}

func (r *logRow) wrapped(width float32) []string {
	remain := width - r.prefixWidth()
	if remain < z(64) {
		remain = z(64)
	}
	return wrapToWidth(r.msg, remain, fsSmall, logMsgStyle())
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
}

func (d *logRowRenderer) Destroy() {}

func (d *logRowRenderer) Objects() []fyne.CanvasObject {
	out := []fyne.CanvasObject{d.when, d.level, d.fn}
	for _, t := range d.msgs {
		out = append(out, t)
	}
	return out
}

func (d *logRowRenderer) ensureMsgs(n int) {
	p := pal()
	for len(d.msgs) < n {
		d.msgs = append(d.msgs, monoText("", fsSmall, p.Content))
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
	d.when.Text = d.r.when
	d.level.Text = d.r.level
	d.fn.Text = elide(d.r.fn, logFnCol-sp3, d.fn.TextSize, d.fn.TextStyle)

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
	for i, t := range d.msgs {
		if i >= len(lines) {
			t.Text = ""
			t.Hide()
			t.Move(fyne.NewPos(0, 0))
			t.Resize(fyne.NewSize(0, 0))
			continue
		}
		t.Show()
		t.Text = lines[i]
		t.TextSize = fsSmall
		t.TextStyle = logMsgStyle()
		t.Color = fg
		t.Move(fyne.NewPos(x, lh*float32(i)))
		t.Resize(t.MinSize())
	}

	want := d.r.heightFor(size.Width)
	if d.r.list != nil && want != size.Height {
		d.r.list.SetItemHeight(d.r.id, want)
	}
}

func (d *logRowRenderer) Refresh() {
	d.apply()
	d.when.Refresh()
	d.level.Refresh()
	d.fn.Refresh()
	for _, t := range d.msgs {
		t.Refresh()
	}
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
