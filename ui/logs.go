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
	msg   string
}

func newLogRow() *logRow {
	r := &logRow{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *logRow) set(when, level, msg string) {
	r.when, r.level, r.msg = when, level, msg
	r.Refresh()
}

func (r *logRow) CreateRenderer() fyne.WidgetRenderer {
	p := pal()
	d := &logRowRenderer{
		r:     r,
		when:  monoText("", fsCaption, p.Faint),
		level: monoText("", fsCaption, p.Muted),
		msg:   monoText("", fsSmall, p.Content),
	}
	d.level.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	d.apply()
	return d
}

type logRowRenderer struct {
	r     *logRow
	when  *canvas.Text
	level *canvas.Text
	msg   *canvas.Text
}

func (d *logRowRenderer) Destroy() {}

func (d *logRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{d.when, d.level, d.msg}
}

func (d *logRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(z(400), d.msg.MinSize().Height+sp1)
}

func (d *logRowRenderer) Layout(size fyne.Size) {
	y := func(t *canvas.Text) float32 { return (size.Height - t.MinSize().Height) / 2 }
	d.when.Move(fyne.NewPos(0, y(d.when)))
	wq := d.when.MinSize().Width
	d.level.Move(fyne.NewPos(wq+sp3, y(d.level)))
	d.msg.Move(fyne.NewPos(wq+sp3+logLevelCol, y(d.msg)))
}

func (d *logRowRenderer) Refresh() {
	d.apply()
	d.when.Refresh()
	d.level.Refresh()
	d.msg.Refresh()
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
	d.msg.Text = d.r.msg
	d.msg.Color = p.Content
	if d.r.level == "ERROR" {
		d.msg.Color = p.Error
	}
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
	default:
		return p.Faint
	}
}

// splitLogLine pulls "<time> || <LEVEL> || <message>" apart, tolerating lines
// that do not follow the convention.
func splitLogLine(line string) (when, level, msg string) {
	parts := strings.Split(line, " || ")
	switch len(parts) {
	case 0:
		return "", "", line
	case 1:
		return "", "", strings.TrimSpace(parts[0])
	case 2:
		return strings.TrimSpace(parts[0]), "", strings.TrimSpace(parts[1])
	default:
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]),
			strings.TrimSpace(strings.Join(parts[2:], " || "))
	}
}

// ---------------------------------------------------------------- page

func (a *App) recomputeLogView() {
	a.logView = a.filteredLogs()
}

var logTags = []segItem{
	{"", "All"}, {"INFO", "Info"}, {"ERROR", "Errors"}, {"DEBUG", "Debug"}, {"ROUTINE", "Routine"},
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

	clear := newIconBtn(theme.DeleteIcon(), kOutline, func() {
		a.confirm("Clear logs", "Remove all captured log lines from this view?", func() {
			a.logs = nil
			a.reloadCurrent()
		})
	})

	a.recomputeLogView()
	sub := fmt.Sprintf("%d lines", len(a.logView))
	actions := hstack(sp2, search, tags, clear)

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
			when, level, msg := splitLogLine(a.logView[id])
			row.set(when, level, msg)
		},
	)
	a.logList.HideSeparators = true

	panel := container.NewStack(
		surface(radLg, pal().Base100, pal().Base300),
		insetXY(sp4, sp3, a.logList),
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
		_, tag, _ := splitLogLine(line)
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
