package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func kInput(inner fyne.CanvasObject) fyne.CanvasObject {
	p := pal()
	bg := canvas.NewRectangle(p.Base100)
	bg.CornerRadius = 6
	bg.StrokeColor = p.Base300
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.NewPadded(inner))
}

func kLabeled(label string, obj fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(fieldLabel(label), vspace(4), obj)
}

func kLabeledEntry(label, placeholder string, password bool) (*widget.Entry, fyne.CanvasObject) {
	e := widget.NewEntry()
	if password {
		e = widget.NewPasswordEntry()
	}
	e.SetPlaceHolder(placeholder)
	return e, kLabeled(label, kInput(e))
}

func kSearch(placeholder, value string, onChange func(string), onSubmit func(string)) *widget.Entry {
	e := widget.NewEntry()
	e.SetPlaceHolder(placeholder)
	e.SetText(value)
	e.OnChanged = onChange
	e.OnSubmitted = onSubmit
	return e
}

func kSearchBox(e *widget.Entry) fyne.CanvasObject {
	return kInput(e)
}

func pageHeader(title, count string, extras ...fyne.CanvasObject) fyne.CanvasObject {
	left := container.NewHBox(heading(title))
	if count != "" {
		left.Add(hspace(8))
		left.Add(caption(count))
	}
	right := container.NewHBox(extras...)
	return container.NewBorder(nil, nil, left, right)
}

func listPage(header, body fyne.CanvasObject) fyne.CanvasObject {
	top := container.NewPadded(container.NewVBox(vspace(4), header, vspace(8)))
	return container.NewBorder(top, nil, hspace(12), hspace(16), body)
}

func emptyState(msg string) fyne.CanvasObject {
	return container.NewCenter(container.NewVBox(vspace(40), caption(msg)))
}

func kInfo(label, value string) fyne.CanvasObject {
	if value == "" {
		value = "—"
	}
	p := pal()
	line := canvas.NewRectangle(p.Base200)
	line.SetMinSize(fyne.NewSize(1, 1))
	k := text(label, 13, p.Muted, false)
	v := text(value, 13, p.Content, false)
	v.Alignment = fyne.TextAlignTrailing
	return container.NewVBox(container.NewBorder(nil, nil, k, nil, v), vspace(6), line, vspace(6))
}
