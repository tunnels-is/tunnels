package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ---------------------------------------------------------------- table spec

// tableCol describes one column. The same spec drives the header and every row,
// which is what keeps the two aligned: rows used to position their own text with
// no header to align to.
type tableCol struct {
	label  string
	weight float32
	mono   bool // identifiers and addresses, so digits line up
	strong bool // the primary column: content colour, semibold
	align  fyne.TextAlign
	badge  bool // render the value as a status pill instead of plain text
	// optional columns are dropped when the table is too narrow, so the
	// columns that matter keep their width instead of everything truncating.
	optional bool
}

type tableSpec struct {
	cols []tableCol
	// actionW reserves trailing space for the row's buttons. The header leaves
	// the same gap, so a column label never sits over the action cluster.
	actionW float32
}

func (s *tableSpec) weights() []float32 {
	w := make([]float32, len(s.cols))
	for i, c := range s.cols {
		w[i] = c.weight
		if w[i] <= 0 {
			w[i] = 1
		}
	}
	return w
}

type cellRect struct{ x, w float32 }

// narrowWidth is the content width below which optional columns are dropped.
var narrowWidth = func() float32 { return z(560) }

// cellRects computes each column's offset and width for a given row width.
// Dropped columns get zero width, and both the header and the rows render
// nothing for them.
func (s *tableSpec) cellRects(width float32) []cellRect {
	avail := width - z(s.actionW) - sp4*2
	if avail < 0 {
		avail = 0
	}
	weights := s.weights()
	drop := avail < narrowWidth()

	var total float32
	for i, w := range weights {
		if drop && s.cols[i].optional {
			continue
		}
		total += w
	}
	if total == 0 {
		total = 1
	}

	out := make([]cellRect, len(s.cols))
	x := sp4
	for i, w := range weights {
		if drop && s.cols[i].optional {
			out[i] = cellRect{x, 0}
			continue
		}
		cw := avail * w / total
		out[i] = cellRect{x, max32(0, cw-sp3)}
		x += cw
	}
	return out
}

// ---------------------------------------------------------------- header

type tableHeadLayout struct{ spec *tableSpec }

func (t *tableHeadLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	rects := t.spec.cellRects(size.Width)
	for i, o := range objs {
		if i >= len(rects) {
			// Trailing object is the hairline under the labels.
			o.Resize(fyne.NewSize(size.Width, z(1)))
			o.Move(fyne.NewPos(0, size.Height-z(1)))
			continue
		}
		txt, ok := o.(*canvas.Text)
		if !ok {
			continue
		}
		if rects[i].w <= 0 {
			txt.Text = ""
			txt.Refresh()
			continue
		}
		txt.Text = t.spec.cols[i].label
		ms := txt.MinSize()
		x := rects[i].x
		if t.spec.cols[i].align == fyne.TextAlignTrailing {
			x = rects[i].x + rects[i].w - ms.Width
		}
		txt.Move(fyne.NewPos(x, (size.Height-ms.Height)/2))
	}
}

func (t *tableHeadLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, z(28))
}

// newTableHeader builds the column-label strip, including its own hairline.
func newTableHeader(spec *tableSpec) fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, len(spec.cols)+1)
	for _, c := range spec.cols {
		objs = append(objs, text(c.label, fsCaption, pal().Faint, true))
	}
	objs = append(objs, canvas.NewRectangle(pal().Base300))
	return container.New(&tableHeadLayout{spec: spec}, objs...)
}

// ---------------------------------------------------------------- row

// kRow is one table row: a cell per column plus an action cluster. Used as a
// Fyne List item template, so UpdateItem calls SetCells rather than rebuilding.
type kRow struct {
	widget.BaseWidget
	spec      *tableSpec
	cells     []string
	badgeTone tone
	on        bool
	hovered   bool
	main      *kBtn
	ghost     *kBtn
	iconA     *kBtn
	iconB     *kBtn
}

func newKRow(spec *tableSpec) *kRow {
	r := &kRow{
		spec:  spec,
		cells: make([]string, len(spec.cols)),
		main:  newKBtn("Connect", kSuccess, nil).small(),
		ghost: newKBtn("More", kGhost, nil).small(),
		iconA: newIconBtn(theme.DocumentCreateIcon(), kGhost, nil).small(),
		iconB: newIconBtn(theme.DeleteIcon(), kGhost, nil).small(),
	}
	// Every button starts hidden; a page shows only the ones it binds. The main
	// button used to default to visible, which put a stray Connect on tables
	// that have no row actions at all.
	r.main.SetHidden(true)
	r.ghost.SetHidden(true)
	r.iconA.SetHidden(true)
	r.iconB.SetHidden(true)
	r.ExtendBaseWidget(r)
	return r
}

// SetCells fills the row. on drives the live treatment (primary rail and
// tunnels-blue wash); badgeTone colours whichever column is marked as a badge.
func (r *kRow) SetCells(cells []string, on bool, badgeTone tone) {
	for i := range r.cells {
		if i < len(cells) {
			r.cells[i] = cells[i]
		} else {
			r.cells[i] = ""
		}
	}
	r.on = on
	r.badgeTone = badgeTone
	if on && badgeTone == toneNeutral {
		r.badgeTone = tonePrimary
	}
	r.Refresh()
}

func (r *kRow) MouseIn(*desktop.MouseEvent)    { r.hovered = true; r.Refresh() }
func (r *kRow) MouseOut()                      { r.hovered = false; r.Refresh() }
func (r *kRow) MouseMoved(*desktop.MouseEvent) {}
func (r *kRow) Cursor() desktop.Cursor         { return desktop.DefaultCursor }

var (
	_ fyne.Tappable          = (*kRow)(nil)
	_ fyne.SecondaryTappable = (*kRow)(nil)
	_ desktop.Hoverable      = (*kRow)(nil)
)

// Tapped swallows the click so widget.List never selects the row. List draws
// its selection wash on a wrapper behind the child; Unselect in OnSelected
// cannot undo the item's own selected flag, and focusing the list then keeps
// a highlight on currentHighlight. Buttons still receive the tap: they are
// deeper Tappable objects than the row.
func (r *kRow) Tapped(*fyne.PointEvent) {}

func (r *kRow) TappedSecondary(*fyne.PointEvent) {}

func (r *kRow) CreateRenderer() fyne.WidgetRenderer {
	p := pal()
	d := &kRowRenderer{
		r:       r,
		bg:      canvas.NewRectangle(color.Transparent),
		rail:    canvas.NewRectangle(color.Transparent),
		line:    canvas.NewRectangle(p.Divider),
		badgeBg: surface(radFull, color.Transparent, nil),
	}
	d.rail.CornerRadius = radFull
	for _, c := range r.spec.cols {
		var t *canvas.Text
		switch {
		case c.badge:
			t = text("", fsCaption, p.Muted, true)
		case c.mono:
			t = monoText("", fsSmall, p.Muted)
		case c.strong:
			t = text("", fsBody, p.Content, true)
		default:
			t = text("", fsBody, p.Muted, false)
		}
		d.cells = append(d.cells, t)
	}
	d.apply()
	return d
}

type kRowRenderer struct {
	r       *kRow
	bg      *canvas.Rectangle
	rail    *canvas.Rectangle
	line    *canvas.Rectangle
	cells   []*canvas.Text
	badgeBg *canvas.Rectangle
}

func (d *kRowRenderer) Destroy() {}

func (d *kRowRenderer) Objects() []fyne.CanvasObject {
	out := []fyne.CanvasObject{d.bg, d.line, d.rail, d.badgeBg}
	for _, c := range d.cells {
		out = append(out, c)
	}
	return append(out, d.r.main, d.r.ghost, d.r.iconA, d.r.iconB)
}

func (d *kRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(z(520), rowHeight)
}

func (d *kRowRenderer) Layout(size fyne.Size) {
	// widget.List always inserts SizeNamePadding between items and offers no way
	// to turn that off, so each row claims the gap *below* it. Growing downwards
	// rather than in both directions keeps the first row flush with the column
	// header, and gives every row an identical band.
	band := size.Height + sp2
	d.bg.Resize(fyne.NewSize(size.Width, band))
	d.bg.Move(fyne.NewPos(0, 0))

	// The divider closes the band, so it lands on the real row boundary.
	d.line.Resize(fyne.NewSize(size.Width, z(1)))
	d.line.Move(fyne.NewPos(0, band-z(1)))

	// Content is centred on the band, not on the bare row, or it would sit high.
	mid := band / 2
	railH := max32(z(12), band-sp3)
	d.rail.Resize(fyne.NewSize(z(3), railH))
	d.rail.Move(fyne.NewPos(0, mid-railH/2))

	// The action cluster owns the trailing edge; the width the spec reserves for
	// it is what both the cells and the header lay out against.
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
		b.Move(fyne.NewPos(right, mid-ms.Height/2))
		right -= sp1
	}
	place(d.r.iconB)
	place(d.r.iconA)
	place(d.r.ghost)
	place(d.r.main)

	rects := d.r.spec.cellRects(size.Width)
	hideBadge := true
	for i, cell := range d.cells {
		if i >= len(rects) {
			break
		}
		col := d.r.spec.cols[i]
		w := rects[i].w
		cell.Text = elide(d.r.cells[i], w, cell.TextSize, cell.TextStyle)
		ms := cell.MinSize()
		x := rects[i].x
		if col.align == fyne.TextAlignTrailing {
			x = rects[i].x + w - ms.Width
		}
		cell.Move(fyne.NewPos(x, mid-ms.Height/2))

		if col.badge && d.r.cells[i] != "" {
			hideBadge = false
			pw := ms.Width + sp2*2
			ph := z(19)
			d.badgeBg.Resize(fyne.NewSize(pw, ph))
			d.badgeBg.Move(fyne.NewPos(x-sp2, mid-ph/2))
		}
	}
	if hideBadge {
		d.badgeBg.Resize(fyne.NewSize(0, 0))
	}
}

func (d *kRowRenderer) Refresh() {
	d.apply()
	for _, o := range d.Objects() {
		o.Refresh()
	}
	if sz := d.r.Size(); sz.Width > 0 {
		d.Layout(sz)
	}
	canvasRefresh(d.r)
}

func (d *kRowRenderer) apply() {
	p := pal()
	d.line.FillColor = p.Divider

	badgeFg, badgeBg := toneColors(d.r.badgeTone)
	d.badgeBg.FillColor = badgeBg
	for i, cell := range d.cells {
		switch col := d.r.spec.cols[i]; {
		case col.badge:
			cell.Color = badgeFg
		case col.strong:
			cell.Color = p.Content
		default:
			cell.Color = p.Muted
		}
	}

	switch {
	case d.r.on:
		d.rail.FillColor = p.Primary
		if d.r.hovered {
			d.bg.FillColor = withAlpha(p.Primary, 56)
		} else {
			d.bg.FillColor = p.PrimarySoft
		}
	default:
		d.rail.FillColor = color.Transparent
		if d.r.hovered {
			d.bg.FillColor = p.Hover
		} else {
			// Opaque so widget.List's selection wash, drawn *behind* the
			// child, cannot show through after a click.
			d.bg.FillColor = p.Base200
		}
	}
}

// ---------------------------------------------------------------- table body

// newRowList builds the virtualised list of rows for a spec.
func newRowList(spec *tableSpec, count func() int, bind func(widget.ListItemID, *kRow)) *widget.List {
	l := widget.NewList(
		count,
		func() fyne.CanvasObject { return newKRow(spec) },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row, ok := obj.(*kRow)
			if !ok {
				return
			}
			bind(id, row)
		},
	)
	l.HideSeparators = true
	l.OnSelected = func(id widget.ListItemID) {
		l.UnselectAll()
	}
	return l
}

// headerBodyLayout puts a fixed-height header directly above a filling body.
//
// container.NewBorder cannot be used here: it inserts theme padding between the
// border object and the centre, which left an 8px strip of page background
// between the column header and the first row that no hover highlight covered.
type headerBodyLayout struct{}

func (headerBodyLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 2 {
		return
	}
	head, body := objs[0], objs[1]
	hh := head.MinSize().Height
	head.Move(fyne.NewPos(0, 0))
	head.Resize(fyne.NewSize(size.Width, hh))
	body.Move(fyne.NewPos(0, hh))
	body.Resize(fyne.NewSize(size.Width, max32(0, size.Height-hh)))
}

func (headerBodyLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		ms := o.MinSize()
		w = max32(w, ms.Width)
		h += ms.Height
	}
	return fyne.NewSize(w, h)
}

// tableBody stacks the column header above the scrolling rows. The header
// carries its own hairline, which is why table pages drop the page-level rule:
// two lines a few pixels apart read as a mistake.
func tableBody(spec *tableSpec, l *widget.List) fyne.CanvasObject {
	pad := gutter - sp4
	head := insetEach(0, pad, 0, pad, newTableHeader(spec))
	quiet := container.NewThemeOverride(l, listQuietTheme{live})
	boost := &scrollBoost{list: l}
	boost.ExtendBaseWidget(boost)
	rows := insetEach(0, pad, sp3, pad, container.NewStack(quiet, boost))
	return container.New(headerBodyLayout{}, head, rows)
}

// listQuietTheme hides Fyne's list selection wash. Table rows are not
// selectable records; hover and the connected tint are drawn on the row.
type listQuietTheme struct{ fyne.Theme }

func (t listQuietTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == theme.ColorNameSelection {
		return color.Transparent
	}
	return t.Theme.Color(n, v)
}
