package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type kKind int

const (
	kPrimary kKind = iota
	kSuccess
	kDanger
	kGhost
	kOutline
)

// kBtn is a compact rounded button used everywhere instead of stock Fyne buttons.
type kBtn struct {
	widget.BaseWidget
	label    string
	icon     fyne.Resource
	kind     kKind
	onTap    func()
	hovered  bool
	pressed  bool
	disabled bool
	hidden   bool
}

func newKBtn(label string, kind kKind, tap func()) *kBtn {
	b := &kBtn{label: label, kind: kind, onTap: tap}
	b.ExtendBaseWidget(b)
	return b
}

func newIconBtn(res fyne.Resource, kind kKind, tap func()) *kBtn {
	b := &kBtn{icon: res, kind: kind, onTap: tap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *kBtn) Set(label string, kind kKind, tap func()) {
	b.label = label
	b.kind = kind
	b.onTap = tap
	b.hidden = false
	b.Show()
	b.Refresh()
}

func (b *kBtn) SetIconOnly(res fyne.Resource, kind kKind, tap func()) {
	b.label = ""
	b.icon = res
	b.kind = kind
	b.onTap = tap
	b.hidden = false
	b.Show()
	b.Refresh()
}

func (b *kBtn) SetHidden(h bool) {
	b.hidden = h
	if h {
		b.Hide()
	} else {
		b.Show()
	}
}

func (b *kBtn) Disable() {
	b.disabled = true
	b.Refresh()
}

func (b *kBtn) Enable() {
	b.disabled = false
	b.Refresh()
}

func (b *kBtn) Tapped(*fyne.PointEvent) {
	if b.disabled || b.hidden || b.onTap == nil {
		return
	}
	b.onTap()
}

func (b *kBtn) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
}
func (b *kBtn) MouseOut() {
	b.hovered = false
	b.pressed = false
	b.Refresh()
}
func (b *kBtn) MouseMoved(*desktop.MouseEvent) {}
func (b *kBtn) MouseDown(*desktop.MouseEvent) {
	b.pressed = true
	b.Refresh()
}
func (b *kBtn) MouseUp(*desktop.MouseEvent) {
	b.pressed = false
	b.Refresh()
}
func (b *kBtn) Cursor() desktop.Cursor {
	if b.disabled {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (b *kBtn) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 6
	lab := canvas.NewText(b.label, pal().Content)
	lab.TextSize = 12
	lab.TextStyle.Bold = true
	ico := widget.NewIcon(theme.ConfirmIcon())
	r := &kBtnRenderer{b: b, bg: bg, lab: lab, ico: ico}
	r.apply()
	return r
}

type kBtnRenderer struct {
	b   *kBtn
	bg  *canvas.Rectangle
	lab *canvas.Text
	ico *widget.Icon
}

func (r *kBtnRenderer) Destroy() {}

func (r *kBtnRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.ico, r.lab}
}

func (r *kBtnRenderer) MinSize() fyne.Size {
	if r.b.hidden {
		return fyne.NewSize(0, 0)
	}
	if r.b.icon != nil && r.b.label == "" {
		return fyne.NewSize(28, 28)
	}
	w := r.lab.MinSize().Width + 24
	if r.b.icon != nil {
		w += 18
	}
	if w < 72 {
		w = 72
	}
	return fyne.NewSize(w, 28)
}

func (r *kBtnRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))
	if r.b.icon != nil && r.b.label == "" {
		r.ico.Resize(fyne.NewSize(14, 14))
		r.ico.Move(fyne.NewPos((size.Width-14)/2, (size.Height-14)/2))
		r.lab.Move(fyne.NewPos(0, 0))
		return
	}
	lw := r.lab.MinSize().Width
	lh := r.lab.MinSize().Height
	if r.b.icon != nil {
		r.ico.Resize(fyne.NewSize(14, 14))
		total := 14 + 6 + lw
		x := (size.Width - total) / 2
		r.ico.Move(fyne.NewPos(x, (size.Height-14)/2))
		r.lab.Move(fyne.NewPos(x+20, (size.Height-lh)/2))
		return
	}
	r.ico.Resize(fyne.NewSize(0, 0))
	r.lab.Move(fyne.NewPos((size.Width-lw)/2, (size.Height-lh)/2))
}

func (r *kBtnRenderer) Refresh() {
	r.apply()
	r.bg.Refresh()
	r.lab.Refresh()
	canvasRefresh(r.b)
}

func (r *kBtnRenderer) apply() {
	p := pal()
	r.lab.Text = r.b.label
	r.lab.TextSize = 12
	r.lab.TextStyle.Bold = true
	if r.b.icon != nil {
		r.ico.SetResource(r.b.icon)
	}
	bg, fg, stroke := r.colors(p)
	r.bg.FillColor = bg
	r.bg.StrokeColor = stroke
	if stroke.A == 0 {
		r.bg.StrokeWidth = 0
	} else {
		r.bg.StrokeWidth = 1
	}
	r.lab.Color = fg
}

func (r *kBtnRenderer) colors(p palette) (bg, fg, stroke color.NRGBA) {
	switch r.b.kind {
	case kPrimary:
		bg, fg = p.Primary, p.PrimaryContent
	case kSuccess:
		bg, fg = p.Success, hex(0xff, 0xff, 0xff)
	case kDanger:
		bg = withAlpha(p.Error, 22)
		fg = p.Error
		stroke = withAlpha(p.Error, 90)
	case kOutline:
		bg = p.Base100
		fg = p.Content
		stroke = p.Base300
	default: // ghost
		bg = color.NRGBA{}
		fg = p.Muted
		if r.b.hovered {
			bg = p.Hover
			fg = p.Content
		}
	}
	if r.b.disabled {
		bg.A = 120
		fg.A = 160
	} else if r.b.pressed {
		bg = withAlpha(bg, 180)
	} else if r.b.hovered && r.b.kind != kGhost {
		bg = lighten(bg, 12)
	}
	return
}

func lighten(c color.NRGBA, d uint8) color.NRGBA {
	add := func(v uint8) uint8 {
		if int(v)+int(d) > 255 {
			return 255
		}
		return v + d
	}
	if c.A == 0 {
		return c
	}
	c.R, c.G, c.B = add(c.R), add(c.G), add(c.B)
	return c
}

func canvasRefresh(obj fyne.CanvasObject) {
	if c := fyne.CurrentApp(); c != nil {
		if drv := c.Driver(); drv != nil {
			if cv := drv.CanvasForObject(obj); cv != nil {
				cv.Refresh(obj)
			}
		}
	}
}
