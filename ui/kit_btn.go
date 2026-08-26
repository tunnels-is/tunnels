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
	kSubtle
)

type kSize int

const (
	kMed kSize = iota
	kSm
)

// kBtn is the only button in the app. Stock Fyne buttons are never used, so
// every clickable surface shares one set of metrics and states.
type kBtn struct {
	widget.BaseWidget
	label    string
	icon     fyne.Resource
	kind     kKind
	size     kSize
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

func (b *kBtn) small() *kBtn {
	b.size = kSm
	return b
}

func (b *kBtn) withIcon(res fyne.Resource) *kBtn {
	b.icon = res
	return b
}

func (b *kBtn) Set(label string, kind kKind, tap func()) {
	b.onTap = tap
	same := b.label == label && b.kind == kind && b.icon == nil && !b.hidden
	b.label = label
	b.icon = nil
	b.kind = kind
	b.hidden = false
	b.Show()
	if !same {
		b.Refresh()
	}
}

func (b *kBtn) SetIconOnly(res fyne.Resource, kind kKind, tap func()) {
	b.onTap = tap
	same := b.label == "" && b.kind == kind && b.icon == res && !b.hidden
	b.label = ""
	b.icon = res
	b.kind = kind
	b.hidden = false
	b.Show()
	if !same {
		b.Refresh()
	}
}

func (b *kBtn) SetHidden(h bool) {
	if b.hidden == h {
		if h {
			b.Hide()
		}
		return
	}
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
	if b.disabled {
		return
	}
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
	if b.disabled {
		return
	}
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

func (b *kBtn) height() float32 {
	if b.size == kSm {
		return z(26)
	}
	return ctrlHeight
}

func (b *kBtn) fontSize() float32 {
	if b.size == kSm {
		return fsSmall
	}
	return fsBody
}

func (b *kBtn) CreateRenderer() fyne.WidgetRenderer {
	bg := surface(radSm, color.Transparent, nil)
	lab := canvas.NewText(b.label, pal().Content)
	lab.TextStyle = fyne.TextStyle{Bold: true}
	lab.FontSource = fontForStyle(lab.TextStyle)
	ico := canvas.NewImageFromResource(b.icon)
	ico.FillMode = canvas.ImageFillContain
	r := &kBtnRenderer{b: b, bg: bg, lab: lab, ico: ico}
	if r.apply() {
		r.ico.Refresh()
	}
	return r
}

type kBtnRenderer struct {
	b      *kBtn
	bg     *canvas.Rectangle
	lab    *canvas.Text
	ico    *canvas.Image
	icoSrc fyne.Resource
	icoFg  fyne.ThemeColorName
	objs   []fyne.CanvasObject
}

func (r *kBtnRenderer) Destroy() {}

func (r *kBtnRenderer) Objects() []fyne.CanvasObject {
	if r.objs == nil {
		r.objs = []fyne.CanvasObject{r.bg, r.ico, r.lab}
	}
	return r.objs
}

func (r *kBtnRenderer) iconSize() float32 {
	if r.b.size == kSm {
		return z(13)
	}
	return z(15)
}

func (r *kBtnRenderer) MinSize() fyne.Size {
	if r.b.hidden {
		return fyne.NewSize(0, 0)
	}
	h := r.b.height()
	if r.b.label == "" {
		return fyne.NewSize(h, h)
	}
	pad := sp3
	if r.b.size == kSm {
		pad = sp2 + 2
	}
	w := r.lab.MinSize().Width + pad*2
	if r.b.icon != nil {
		w += r.iconSize() + sp2
	}
	return fyne.NewSize(w, h)
}

func (r *kBtnRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))
	is := r.iconSize()

	if r.b.label == "" {
		if r.b.icon != nil {
			r.ico.Resize(fyne.NewSize(is, is))
			r.ico.Move(fyne.NewPos((size.Width-is)/2, (size.Height-is)/2))
		}
		r.lab.Move(fyne.NewPos(0, 0))
		return
	}

	lms := r.lab.MinSize()
	if r.b.icon == nil {
		r.ico.Resize(fyne.NewSize(0, 0))
		r.lab.Move(fyne.NewPos((size.Width-lms.Width)/2, (size.Height-lms.Height)/2))
		return
	}
	total := is + sp2 + lms.Width
	x := (size.Width - total) / 2
	r.ico.Resize(fyne.NewSize(is, is))
	r.ico.Move(fyne.NewPos(x, (size.Height-is)/2))
	r.lab.Move(fyne.NewPos(x+is+sp2, (size.Height-lms.Height)/2))
}

func (r *kBtnRenderer) Refresh() {
	iconChanged := r.apply()
	r.bg.Refresh()
	r.lab.Refresh()
	if iconChanged {
		r.ico.Refresh()
	}
	canvasRefresh(r.b)
}

func (r *kBtnRenderer) apply() (iconChanged bool) {
	r.lab.Text = r.b.label
	r.lab.TextSize = r.b.fontSize()
	r.lab.TextStyle = fyne.TextStyle{Bold: true}

	bg, fg, stroke, fgName := r.colors()
	r.bg.FillColor = bg
	r.bg.StrokeColor = stroke
	if stroke.A == 0 {
		r.bg.StrokeWidth = 0
	} else {
		r.bg.StrokeWidth = 1
	}
	r.lab.Color = fg
	if r.b.icon == nil {
		r.icoSrc = nil
		r.icoFg = ""
		return false
	}
	if r.icoSrc == r.b.icon && r.icoFg == fgName {
		return false
	}
	r.ico.Resource = theme.NewColoredResource(r.b.icon, fgName)
	r.icoSrc = r.b.icon
	r.icoFg = fgName
	return true
}

func (r *kBtnRenderer) colors() (bg, fg, stroke color.NRGBA, fgName fyne.ThemeColorName) {
	p := pal()
	switch r.b.kind {
	case kPrimary:
		bg, fg, fgName = p.Primary, p.PrimaryContent, colOnPrimary
		if r.b.hovered {
			bg = p.PrimaryHover
		}
	case kSuccess:
		bg, fg, fgName = p.Success, hex(0xff, 0xff, 0xff), colOnSolid
		if r.b.hovered {
			bg = p.SuccessHover
		}
	case kDanger:
		bg, fg, fgName = p.Error, hex(0xff, 0xff, 0xff), colOnSolid
		if r.b.hovered {
			bg = p.ErrorHover
		}
	case kOutline:
		bg, fg, stroke, fgName = p.Base100, p.Content, p.Base300, colContent
		if r.b.hovered {
			bg = p.Elevate
		}
	case kSubtle:
		bg, fg, fgName = p.Elevate, p.Content, colContent
		if r.b.hovered {
			bg = p.Hover
		}
	default: // ghost
		bg, fg, fgName = color.NRGBA{}, p.Muted, colMuted
		if r.b.hovered {
			bg, fg, fgName = p.Hover, p.Content, colContent
		}
	}
	if r.b.disabled {
		bg = withAlpha(bg, bg.A/3)
		fg = withAlpha(fg, 110)
		stroke = withAlpha(stroke, stroke.A/2)
		fgName = colFaint
	} else if r.b.pressed {
		bg = darken(bg, 18)
	}
	return
}

func darken(c color.NRGBA, d uint8) color.NRGBA {
	if c.A == 0 {
		return c
	}
	sub := func(v uint8) uint8 {
		if int(v)-int(d) < 0 {
			return 0
		}
		return v - d
	}
	c.R, c.G, c.B = sub(c.R), sub(c.G), sub(c.B)
	return c
}

// Shorthand constructors used across the pages.
func primaryBtn(label string, fn func()) *kBtn { return newKBtn(label, kPrimary, fn) }
func successBtn(label string, fn func()) *kBtn { return newKBtn(label, kSuccess, fn) }
func dangerBtn(label string, fn func()) *kBtn  { return newKBtn(label, kDanger, fn) }
func ghostBtn(label string, fn func()) *kBtn   { return newKBtn(label, kGhost, fn) }
func outlineBtn(label string, fn func()) *kBtn { return newKBtn(label, kOutline, fn) }
