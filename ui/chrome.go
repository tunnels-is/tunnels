package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const (
	railWidth    float32 = 56
	railExpanded float32 = 208
	cardRadius   float32 = 8
)

func spacer(w, h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(w, h))
	return r
}

func vspace(h float32) fyne.CanvasObject { return spacer(1, h) }
func hspace(w float32) fyne.CanvasObject { return spacer(w, 1) }

func text(s string, size float32, c color.Color, bold bool) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: bold}
	return t
}

func caption(s string) fyne.CanvasObject {
	return text(s, 11, pal().Muted, false)
}

func heading(s string) fyne.CanvasObject {
	return text(s, 13, pal().Content, true)
}

func titleText(s string) fyne.CanvasObject {
	return text(s, 15, pal().Content, true)
}

type tapWrap struct {
	widget.BaseWidget
	obj    fyne.CanvasObject
	tapped func()
}

func newTap(obj fyne.CanvasObject, tapped func()) *tapWrap {
	t := &tapWrap{obj: obj, tapped: tapped}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapWrap) Tapped(*fyne.PointEvent) {
	if t.tapped != nil {
		t.tapped()
	}
}

func (t *tapWrap) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (t *tapWrap) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.obj)
}

func cardBox(title, desc string, actions fyne.CanvasObject, body fyne.CanvasObject) fyne.CanvasObject {
	p := pal()
	bg := canvas.NewRectangle(p.Base100)
	bg.CornerRadius = cardRadius
	bg.StrokeColor = p.Base300
	bg.StrokeWidth = 1

	var inner []fyne.CanvasObject
	if title != "" || desc != "" || actions != nil {
		var head fyne.CanvasObject
		if title != "" {
			head = heading(title)
		}
		if actions != nil {
			head = container.NewBorder(nil, nil, head, actions)
		}
		if head != nil {
			inner = append(inner, head)
		}
		if desc != "" {
			d := widget.NewLabel(desc)
			d.Wrapping = fyne.TextWrapWord
			d.Importance = widget.LowImportance
			inner = append(inner, d)
		}
		inner = append(inner, vspace(6))
	}
	if body != nil {
		inner = append(inner, body)
	}
	return container.NewStack(bg, container.NewPadded(container.NewPadded(container.NewVBox(inner...))))
}

func primaryBtn(label string, fn func()) *kBtn { return newKBtn(label, kPrimary, fn) }
func successBtn(label string, fn func()) *kBtn { return newKBtn(label, kSuccess, fn) }
func dangerBtn(label string, fn func()) *kBtn  { return newKBtn(label, kDanger, fn) }
func ghostBtn(label string, fn func()) *kBtn   { return newKBtn(label, kGhost, fn) }
func outlineBtn(label string, fn func()) *kBtn { return newKBtn(label, kOutline, fn) }
func iconBtn(res fyne.Resource, fn func()) *kBtn {
	return newIconBtn(res, kGhost, fn)
}

func statusDot(on bool) fyne.CanvasObject {
	p := pal()
	c := color.Color(p.Faint)
	if on {
		c = p.Success
	}
	r := canvas.NewCircle(c)
	box := spacer(10, 10)
	return container.NewCenter(container.NewStack(box, r))
}

func pill(label string, on bool) fyne.CanvasObject {
	p := pal()
	fg := color.Color(p.Faint)
	bgc := color.Color(p.Hover)
	if on {
		fg = p.Success
		bgc = p.SuccessSoft
	}
	bg := canvas.NewRectangle(bgc)
	bg.CornerRadius = 999
	t := text(label, 11, fg, true)
	return container.NewStack(bg, container.NewPadded(t))
}

func fieldLabel(s string) fyne.CanvasObject {
	return text(s, 12, pal().Muted, true)
}

func toastBanner(kind, msg string) fyne.CanvasObject {
	if msg == "" {
		return spacer(1, 1)
	}
	p := pal()
	bg := canvas.NewRectangle(p.Base100)
	bg.CornerRadius = 8
	bg.StrokeColor = p.Base300
	bg.StrokeWidth = 1
	barCol := p.Success
	if kind == "error" {
		barCol = p.Error
	}
	bar := canvas.NewRectangle(barCol)
	bar.SetMinSize(fyne.NewSize(4, 1))
	lbl := widget.NewLabel(msg)
	lbl.Wrapping = fyne.TextWrapWord
	inner := container.NewBorder(nil, nil, bar, nil, container.NewPadded(lbl))
	card := container.NewStack(bg, inner)
	card.Resize(fyne.NewSize(320, card.MinSize().Height))
	return container.NewBorder(nil, nil, nil, nil,
		container.NewHBox(layout.NewSpacer(), container.NewVBox(card)))
}

// shellLayout: full-window content plus a top-right toast overlay.
type shellLayout struct{}

func (shellLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 2 {
		return
	}
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(size)
	toast := objs[1]
	ts := toast.MinSize()
	if ts.Width < 8 || ts.Height < 8 {
		toast.Move(fyne.NewPos(size.Width, 0))
		toast.Resize(fyne.NewSize(0, 0))
		return
	}
	w := min32(360, size.Width-32)
	h := ts.Height
	if h < 40 {
		h = 48
	}
	toast.Resize(fyne.NewSize(w, h))
	toast.Move(fyne.NewPos(size.Width-w-16, 16))
}

func (shellLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(960, 640)
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
