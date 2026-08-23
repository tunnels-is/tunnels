package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// kRow is the reusable list row: status rail, title, meta, badge, actions.
// Used as a Fyne List item template so UpdateItem calls Set instead of tearing
// widgets down and rebuilding them.
type kRow struct {
	widget.BaseWidget
	title    string
	meta     string
	pillText string
	pillTone tone
	on       bool
	hovered  bool
	main     *kBtn
	ghost    *kBtn
	iconA    *kBtn
	iconB    *kBtn
}

func newKRow() *kRow {
	r := &kRow{
		main:  newKBtn("Connect", kSuccess, nil).small(),
		ghost: newKBtn("More", kGhost, nil).small(),
		iconA: newIconBtn(theme.DocumentCreateIcon(), kGhost, nil).small(),
		iconB: newIconBtn(theme.DeleteIcon(), kGhost, nil).small(),
	}
	r.iconA.SetHidden(true)
	r.iconB.SetHidden(true)
	r.ghost.SetHidden(true)
	r.ExtendBaseWidget(r)
	return r
}

// SetRow fills the row. tone drives the badge colour independently of the live
// state, so "This device" reads differently from "Connected".
func (r *kRow) SetRow(title, meta string, on bool, pill string, t tone) {
	r.title = title
	r.meta = meta
	r.on = on
	r.pillText = pill
	r.pillTone = t
	if on {
		r.pillTone = toneSuccess
	}
	r.Refresh()
}

func (r *kRow) MouseIn(*desktop.MouseEvent)    { r.hovered = true; r.Refresh() }
func (r *kRow) MouseOut()                      { r.hovered = false; r.Refresh() }
func (r *kRow) MouseMoved(*desktop.MouseEvent) {}
func (r *kRow) Cursor() desktop.Cursor         { return desktop.DefaultCursor }

func (r *kRow) CreateRenderer() fyne.WidgetRenderer {
	p := pal()
	bg := surface(radMd, color.Transparent, nil)
	rail := canvas.NewRectangle(color.Transparent)
	rail.CornerRadius = radFull
	line := canvas.NewRectangle(p.Divider)
	title := text("", fsBody, p.Content, true)
	meta := monoText("", fsSmall, p.Muted)
	pillBg := surface(radFull, color.Transparent, nil)
	pillTxt := text("", fsCaption, p.Faint, true)
	rd := &kRowRenderer{
		r: r, bg: bg, rail: rail, line: line,
		title: title, meta: meta, pillBg: pillBg, pillTxt: pillTxt,
	}
	rd.apply()
	return rd
}

type kRowRenderer struct {
	r       *kRow
	bg      *canvas.Rectangle
	rail    *canvas.Rectangle
	line    *canvas.Rectangle
	title   *canvas.Text
	meta    *canvas.Text
	pillBg  *canvas.Rectangle
	pillTxt *canvas.Text
}

func (d *kRowRenderer) Destroy() {}

func (d *kRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{
		d.bg, d.line, d.rail, d.title, d.meta, d.pillBg, d.pillTxt,
		d.r.main, d.r.ghost, d.r.iconA, d.r.iconB,
	}
}

func (d *kRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(z(520), rowHeight)
}

func (d *kRowRenderer) Layout(size fyne.Size) {
	d.bg.Resize(size)
	d.bg.Move(fyne.NewPos(0, 0))
	d.line.Resize(fyne.NewSize(size.Width, 1))
	d.line.Move(fyne.NewPos(0, size.Height-1))

	// Live rows get a rounded accent rail on the leading edge.
	d.rail.Resize(fyne.NewSize(z(3), size.Height-sp4))
	d.rail.Move(fyne.NewPos(0, sp2))

	right := size.Width - sp4
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
		right -= sp1
	}
	place(d.r.iconB)
	place(d.r.iconA)
	place(d.r.ghost)
	right -= sp1
	place(d.r.main)

	// Text is laid out last: it gets whatever the actions left behind, elided
	// so a long tag cannot slide under the buttons in a narrow window.
	avail := right - sp4 - sp3
	if d.r.pillText != "" {
		avail -= d.pillTxt.MinSize().Width + sp2*2 + sp3
	}
	d.title.Text = elide(d.r.title, avail, d.title.TextSize, d.title.TextStyle)
	d.meta.Text = elide(d.r.meta, avail, d.meta.TextSize, d.meta.TextStyle)
	tms := d.title.MinSize()
	mms := d.meta.MinSize()
	block := tms.Height + z(1) + mms.Height
	top := (size.Height - block) / 2
	d.title.Move(fyne.NewPos(sp4, top))
	d.meta.Move(fyne.NewPos(sp4, top+tms.Height+z(1)))

	if d.r.pillText != "" {
		pw := d.pillTxt.MinSize().Width + sp2*2
		ph := z(19)
		right -= sp3 + pw
		d.pillBg.Resize(fyne.NewSize(pw, ph))
		d.pillBg.Move(fyne.NewPos(right, (size.Height-ph)/2))
		pms := d.pillTxt.MinSize()
		d.pillTxt.Move(fyne.NewPos(right+sp2, (size.Height-pms.Height)/2))
	} else {
		d.pillBg.Resize(fyne.NewSize(0, 0))
		d.pillTxt.Move(fyne.NewPos(right, 0))
	}
}

func (d *kRowRenderer) Refresh() {
	d.apply()
	d.bg.Refresh()
	d.rail.Refresh()
	d.line.Refresh()
	d.title.Refresh()
	d.meta.Refresh()
	d.pillBg.Refresh()
	d.pillTxt.Refresh()
	d.r.main.Refresh()
	d.r.ghost.Refresh()
	d.r.iconA.Refresh()
	d.r.iconB.Refresh()
	if sz := d.r.Size(); sz.Width > 0 {
		d.Layout(sz)
	}
	canvasRefresh(d.r)
}

func (d *kRowRenderer) apply() {
	p := pal()
	d.title.Color = p.Content
	d.meta.Color = p.Muted
	d.line.FillColor = p.Divider

	switch {
	case d.r.on:
		d.rail.FillColor = p.Success
		if d.r.hovered {
			d.bg.FillColor = withAlpha(p.Success, 30)
		} else {
			d.bg.FillColor = withAlpha(p.Success, 18)
		}
	default:
		d.rail.FillColor = color.Transparent
		if d.r.hovered {
			d.bg.FillColor = p.Hover
		} else {
			d.bg.FillColor = color.Transparent
		}
	}

	d.pillTxt.Text = d.r.pillText
	fg, bg := toneColors(d.r.pillTone)
	d.pillTxt.Color = fg
	d.pillBg.FillColor = bg
}

// listRows wraps a widget.List so it inherits row metrics from the kit.
func newRowList(count func() int, bind func(widget.ListItemID, *kRow)) *widget.List {
	l := widget.NewList(
		count,
		func() fyne.CanvasObject { return newKRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row, ok := obj.(*kRow)
			if !ok {
				return
			}
			bind(id, row)
		},
	)
	l.HideSeparators = true
	return l
}
