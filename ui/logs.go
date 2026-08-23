package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (a *App) recomputeLogView() {
	a.logView = a.filteredLogs()
}

func (a *App) logsPage() fyne.CanvasObject {
	filter := widget.NewEntry()
	filter.SetPlaceHolder("Filter logs... (Enter to apply)")
	filter.SetText(a.filterLogs)
	filter.OnChanged = func(s string) { a.filterLogs = s }
	filter.OnSubmitted = func(s string) {
		a.filterLogs = s
		a.paintLogs()
	}

	clear := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		a.logs = nil
		a.paintLogs()
	})

	tags := []struct{ key, label string }{
		{"", "All"}, {"INFO", "Info"}, {"ERROR", "Error"}, {"DEBUG", "Debug"}, {"ROUTINE", "Routine"},
	}
	tagBtns := make([]*widget.Button, len(tags))
	btns := make([]fyne.CanvasObject, 0, len(tags)+1)
	btns = append(btns, clear)
	for i, t := range tags {
		t := t
		b := widget.NewButton(t.label, nil)
		if a.logTag == t.key {
			b.Importance = widget.HighImportance
		}
		tagBtns[i] = b
		b.OnTapped = func() {
			a.logTag = t.key
			for j, tb := range tagBtns {
				if tags[j].key == a.logTag {
					tb.Importance = widget.HighImportance
				} else {
					tb.Importance = widget.MediumImportance
				}
				tb.Refresh()
			}
			a.paintLogs()
		}
		btns = append(btns, b)
	}

	a.logBox = widget.NewMultiLineEntry()
	a.logBox.Wrapping = fyne.TextWrapOff
	a.paintLogs()

	bar := container.NewBorder(nil, nil, nil, container.NewHBox(btns...), filter)
	p := pal()
	bg := canvas.NewRectangle(p.Base100)
	bg.CornerRadius = cardRadius
	bg.StrokeColor = p.Base300
	bg.StrokeWidth = 1
	listCard := container.NewStack(bg, container.NewPadded(a.logBox))
	return container.NewBorder(container.NewPadded(bar), nil, hspace(8), hspace(16), container.NewPadded(listCard))
}

func (a *App) filteredLogs() []string {
	out := make([]string, 0, len(a.logs))
	q := strings.ToLower(a.filterLogs)
	for i := len(a.logs) - 1; i >= 0; i-- {
		line := a.logs[i]
		if q != "" && !strings.Contains(strings.ToLower(line), q) {
			continue
		}
		tag := ""
		if parts := strings.Split(line, " || "); len(parts) > 1 {
			tag = strings.TrimSpace(parts[1])
		}
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
