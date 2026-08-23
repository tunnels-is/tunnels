package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// kRow is the reusable list/table row: status, title, meta, pill, actions.
// Used as a Fyne List item template so UpdateItem calls Set instead of
// tearing down widgets.
type kRow struct {
	widget.BaseWidget
	title    string
	meta     string
	pillText string
	on       bool
	hovered  bool
	main     *kBtn
	ghost    *kBtn
	iconA    *kBtn
	iconB    *kBtn
}

func newKRow() *kRow {
	r := &kRow{
		main:  newKBtn("Connect", kSuccess, nil),
		ghost: newKBtn("More", kGhost, nil),
		iconA: newIconBtn(theme.DocumentCreateIcon(), kGhost, nil),
		iconB: newIconBtn(theme.DeleteIcon(), kGhost, nil),
	}
	r.iconA.SetHidden(true)
	r.iconB.SetHidden(true)
	r.ghost.SetHidden(true)
	r.ExtendBaseWidget(r)
	return r
}

func (r *kRow) SetTitleMeta(title, meta string, on bool, pill string) {
	r.title = title
	r.meta = meta
	r.on = on
	r.pillText = pill
	r.Refresh()
}

func (r *kRow) MouseIn(*desktop.MouseEvent)    { r.hovered = true; r.Refresh() }
func (r *kRow) MouseOut()                      { r.hovered = false; r.Refresh() }
func (r *kRow) MouseMoved(*desktop.MouseEvent) {}
func (r *kRow) Cursor() desktop.Cursor         { return desktop.DefaultCursor }

func (r *kRow) CreateRenderer() fyne.WidgetRenderer {
	p := pal()
	bg := canvas.NewRectangle(color.Transparent)
	line := canvas.NewRectangle(p.Base300)
	dot := canvas.NewCircle(p.Faint)
	title := canvas.NewText("", p.Content)
	title.TextSize = 13
	title.TextStyle.Bold = true
	meta := canvas.NewText("", p.Muted)
	meta.TextSize = 11
	pillBg := canvas.NewRectangle(p.Hover)
	pillBg.CornerRadius = 999
	pillTxt := canvas.NewText("", p.Faint)
	pillTxt.TextSize = 10
	pillTxt.TextStyle.Bold = true
	rd := &kRowRenderer{
		r: r, bg: bg, line: line, dot: dot,
		title: title, meta: meta, pillBg: pillBg, pillTxt: pillTxt,
	}
	rd.apply()
	return rd
}

type kRowRenderer struct {
	r       *kRow
	bg      *canvas.Rectangle
	line    *canvas.Rectangle
	dot     *canvas.Circle
	title   *canvas.Text
	meta    *canvas.Text
	pillBg  *canvas.Rectangle
	pillTxt *canvas.Text
}

func (d *kRowRenderer) Destroy() {}

func (d *kRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{
		d.bg, d.line, d.dot, d.title, d.meta, d.pillBg, d.pillTxt,
		d.r.main, d.r.ghost, d.r.iconA, d.r.iconB,
	}
}

func (d *kRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(480, 56)
}

func (d *kRowRenderer) Layout(size fyne.Size) {
	d.bg.Resize(size)
	d.line.Resize(fyne.NewSize(size.Width-24, 1))
	d.line.Move(fyne.NewPos(12, size.Height-1))

	const pad float32 = 16
	y := (size.Height - 8) / 2
	d.dot.Resize(fyne.NewSize(8, 8))
	d.dot.Move(fyne.NewPos(pad, y))

	textX := pad + 18
	d.title.Move(fyne.NewPos(textX, 10))
	d.meta.Move(fyne.NewPos(textX, 28))

	right := size.Width - pad
	place := func(b *kBtn) {
		if b.hidden {
			b.Resize(fyne.NewSize(0, 0))
			b.Move(fyne.NewPos(right, 0))
			return
		}
		ms := b.MinSize()
		right -= ms.Width
		b.Resize(ms)
		b.Move(fyne.NewPos(right, (size.Height-ms.Height)/2))
		right -= 6
	}
	place(d.r.iconB)
	place(d.r.iconA)
	place(d.r.ghost)
	place(d.r.main)

	if d.r.pillText != "" {
		pw := d.pillTxt.MinSize().Width + 16
		ph := float32(20)
		right -= 8 + pw
		d.pillBg.Resize(fyne.NewSize(pw, ph))
		d.pillBg.Move(fyne.NewPos(right, (size.Height-ph)/2))
		d.pillTxt.Move(fyne.NewPos(right+8, (size.Height-d.pillTxt.MinSize().Height)/2))
	} else {
		d.pillBg.Resize(fyne.NewSize(0, 0))
	}
}

func (d *kRowRenderer) Refresh() {
	d.apply()
	d.bg.Refresh()
	d.line.Refresh()
	d.dot.Refresh()
	d.title.Refresh()
	d.meta.Refresh()
	d.pillBg.Refresh()
	d.pillTxt.Refresh()
	d.r.main.Refresh()
	d.r.ghost.Refresh()
	d.r.iconA.Refresh()
	d.r.iconB.Refresh()
	canvasRefresh(d.r)
}

func (d *kRowRenderer) apply() {
	p := pal()
	d.title.Text = d.r.title
	d.title.Color = p.Content
	d.meta.Text = d.r.meta
	d.meta.Color = p.Muted
	d.line.FillColor = p.Base300
	if d.r.on {
		d.dot.FillColor = p.Success
		if d.r.hovered {
			d.bg.FillColor = withAlpha(p.Success, 36)
		} else {
			d.bg.FillColor = p.SuccessSoft
		}
	} else {
		d.dot.FillColor = p.Faint
		if d.r.hovered {
			d.bg.FillColor = p.Hover
		} else {
			d.bg.FillColor = color.Transparent
		}
	}
	d.pillTxt.Text = d.r.pillText
	if d.r.on {
		d.pillTxt.Color = p.Success
		d.pillBg.FillColor = p.SuccessSoft
	} else {
		d.pillTxt.Color = p.Faint
		d.pillBg.FillColor = p.Hover
	}
}
