package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ---------------------------------------------------------------- layouts

// insetLayout pads its children by an exact amount on each edge. Preferred over
// nested container.NewPadded so spacing is a number you can read off the call.
type insetLayout struct{ t, r, b, l float32 }

func (i insetLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	inner := fyne.NewSize(size.Width-i.l-i.r, size.Height-i.t-i.b)
	for _, o := range objs {
		o.Move(fyne.NewPos(i.l, i.t))
		o.Resize(inner)
	}
}

func (i insetLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		ms := o.MinSize()
		w = max32(w, ms.Width)
		h = max32(h, ms.Height)
	}
	return fyne.NewSize(w+i.l+i.r, h+i.t+i.b)
}

func inset(all float32, obj fyne.CanvasObject) *fyne.Container {
	return container.New(insetLayout{all, all, all, all}, obj)
}

func insetXY(x, y float32, obj fyne.CanvasObject) *fyne.Container {
	return container.New(insetLayout{y, x, y, x}, obj)
}

func insetEach(t, r, b, l float32, obj fyne.CanvasObject) *fyne.Container {
	return container.New(insetLayout{t, r, b, l}, obj)
}

// dropShadow fakes elevation. Fyne has no shadow primitive, so a few
// progressively larger, fainter rounded rects sit behind the content, offset
// downwards so the light reads as coming from above.
func dropShadow(radius float32, content fyne.CanvasObject) fyne.CanvasObject {
	p := pal()
	layers := make([]fyne.CanvasObject, 0, 4)
	for _, l := range []struct {
		spread float32
		alpha  uint8
	}{{16, 9}, {10, 14}, {5, 20}, {2, 26}} {
		spread := z(l.spread)
		r := canvas.NewRectangle(withAlpha(p.Shadow, l.alpha))
		r.CornerRadius = radius + spread
		drop := spread * 0.5
		layers = append(layers, container.New(
			insetLayout{-spread + drop, -spread, -spread - drop, -spread}, r))
	}
	return container.NewStack(append(layers, content)...)
}

// strictLayout forces an exact size, ignoring what the child asks for.
type strictLayout struct{ w, h float32 }

func (s strictLayout) Layout(objs []fyne.CanvasObject, _ fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(fyne.NewSize(s.w, s.h))
	}
}

func (s strictLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(s.w, s.h)
}

// capLayout caps a child's width and pins it to the left. Keeps forms readable
// instead of stretching inputs across a 1280px window.
type capLayout struct {
	max     float32
	centred bool
}

func (c capLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := min32(c.max, size.Width)
	x := float32(0)
	if c.centred {
		x = (size.Width - w) / 2
	}
	for _, o := range objs {
		o.Move(fyne.NewPos(x, 0))
		o.Resize(fyne.NewSize(w, size.Height))
	}
}

func (c capLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		ms := o.MinSize()
		w = max32(w, ms.Width)
		h = max32(h, ms.Height)
	}
	return fyne.NewSize(min32(w, c.max), h)
}

func capWidth(w float32, obj fyne.CanvasObject) *fyne.Container {
	return container.New(capLayout{max: w}, obj)
}

// vstack lays children out top to bottom with one exact gap between them,
// unlike VBox which uses theme padding.
type vstackLayout struct{ gap float32 }

func (v vstackLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	y := float32(0)
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h + v.gap
	}
}

func (v vstackLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	n := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		ms := o.MinSize()
		w = max32(w, ms.Width)
		h += ms.Height
		n++
	}
	if n > 1 {
		h += v.gap * float32(n-1)
	}
	return fyne.NewSize(w, h)
}

func vstack(gap float32, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(vstackLayout{gap: gap}, objs...)
}

// hstack lays children out left to right with one exact gap, each at its
// minimum width, vertically centred. flex names the child to shrink when the
// row does not fit; -1 means shrink whichever is widest.
type hstackLayout struct {
	gap  float32
	flex int
}

func (h hstackLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	// When the row does not fit, take the overflow off one child rather than
	// letting the tail spill past the edge. Shrinking a text field degrades
	// gracefully; shrinking a tab strip or a button clips its labels, so
	// callers can nominate which child gives way.
	over := h.MinSize(objs).Width - size.Width
	shrink := -1
	if over > 0 {
		if h.flex >= 0 && h.flex < len(objs) && objs[h.flex].Visible() {
			shrink = h.flex
		} else {
			var w float32
			for i, o := range objs {
				if o.Visible() && o.MinSize().Width > w {
					w, shrink = o.MinSize().Width, i
				}
			}
		}
	}

	x := float32(0)
	for i, o := range objs {
		if !o.Visible() {
			continue
		}
		ms := o.MinSize()
		if i == shrink {
			ms.Width = max32(z(64), ms.Width-over)
		}
		o.Resize(ms)
		o.Move(fyne.NewPos(x, (size.Height-ms.Height)/2))
		x += ms.Width + h.gap
	}
}

func (h hstackLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, ht float32
	n := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		ms := o.MinSize()
		w += ms.Width
		ht = max32(ht, ms.Height)
		n++
	}
	if n > 1 {
		w += h.gap * float32(n-1)
	}
	return fyne.NewSize(w, ht)
}

func hstack(gap float32, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(hstackLayout{gap: gap, flex: -1}, objs...)
}

// hstackFlex is hstack with an explicit child to compress when space is tight.
func hstackFlex(gap float32, flex int, objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(hstackLayout{gap: gap, flex: flex}, objs...)
}

// rightAlign pins content to the trailing edge, vertically centred.
type rightLayout struct{}

func (rightLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		ms := o.MinSize()
		o.Resize(ms)
		o.Move(fyne.NewPos(size.Width-ms.Width, (size.Height-ms.Height)/2))
	}
}

func (rightLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		ms := o.MinSize()
		w = max32(w, ms.Width)
		h = max32(h, ms.Height)
	}
	return fyne.NewSize(w, h)
}

func rightAlign(obj fyne.CanvasObject) *fyne.Container {
	return container.New(rightLayout{}, obj)
}

// splitRow puts content on the left and actions hard right on one baseline.
func splitRow(left, right fyne.CanvasObject) fyne.CanvasObject {
	if right == nil {
		return left
	}
	if left == nil {
		return rightAlign(right)
	}
	return container.New(splitLayout{}, container.New(vCentreLayout{}, left), right)
}

// splitLayout is Border's left/right pairing with one difference: the actions
// are capped at 60% of the row. A Border hands the trailing object its full
// MinSize, so a zoomed action cluster would cover the title instead of
// compressing.
type splitLayout struct{}

func (splitLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 2 {
		return
	}
	left, right := objs[0], objs[1]
	rw := min32(right.MinSize().Width, size.Width*0.6)
	rh := min32(right.MinSize().Height, size.Height)
	right.Resize(fyne.NewSize(rw, rh))
	right.Move(fyne.NewPos(size.Width-rw, (size.Height-rh)/2))

	lw := max32(0, size.Width-rw-sp3)
	left.Resize(fyne.NewSize(lw, size.Height))
	left.Move(fyne.NewPos(0, 0))
}

func (splitLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		ms := o.MinSize()
		w += ms.Width
		h = max32(h, ms.Height)
	}
	return fyne.NewSize(w+sp3, h)
}

// vCentreLayout gives a child its full width but only its natural height,
// centred vertically in whatever space it is handed.
type vCentreLayout struct{}

func (vCentreLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		h := min32(o.MinSize().Height, size.Height)
		o.Resize(fyne.NewSize(size.Width, h))
		o.Move(fyne.NewPos(0, (size.Height-h)/2))
	}
}

func (vCentreLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		ms := o.MinSize()
		w = max32(w, ms.Width)
		h = max32(h, ms.Height)
	}
	return fyne.NewSize(w, h)
}

// railLayout puts the sidebar on the leading edge and the page beside it, with
// the rail capped at a third of the window. Without the cap a zoomed rail
// squeezes the page into an unusable sliver, since the rail's width scales but
// the window does not.
type railLayout struct{}

func (railLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 2 {
		return
	}
	side, content := objs[0], objs[1]
	var w float32
	if side.Visible() {
		w = min32(side.MinSize().Width, size.Width/3)
	}
	side.Move(fyne.NewPos(0, 0))
	side.Resize(fyne.NewSize(w, size.Height))
	content.Move(fyne.NewPos(w, 0))
	content.Resize(fyne.NewSize(size.Width-w, size.Height))
}

// MinSize is deliberately zero: shellLayout owns the window minimum, and
// letting the scaled rail contribute here would grow the OS window on zoom.
func (railLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

// elide truncates s with an ellipsis so it fits max, so a long title cannot
// run underneath the action buttons in a narrow row.
func elide(s string, max float32, size float32, style fyne.TextStyle) string {
	if max <= 0 || s == "" {
		return ""
	}
	if fyne.MeasureText(s, size, style).Width <= max {
		return s
	}
	r := []rune(s)
	for len(r) > 1 {
		// Drop proportionally to the overshoot, so wide strings converge fast.
		w := fyne.MeasureText(string(r)+"…", size, style).Width
		if w <= max {
			return string(r) + "…"
		}
		cut := int(float32(len(r)) * (1 - max/w))
		if cut < 1 {
			cut = 1
		}
		r = r[:len(r)-cut]
	}
	return "…"
}

// ---------------------------------------------------------------- scrolling

// scrollFactor multiplies wheel deltas. Fyne's per-notch distance is a driver
// constant (scrollSpeed, 25px on Windows and Linux) which is well under one
// list row here, so the default feels sluggish.
const scrollFactor float32 = 3

// scrollBoost amplifies wheel scrolling for the object beneath it.
//
// It has to sit on top rather than wrap: the hit test hands a scroll event to
// the *last* matching object in the tree walk, so a parent never sees it. Being
// scroll-only is what makes this safe — tap, hover, drag and cursor lookups use
// their own predicates, so they skip straight past to the real content.
type scrollBoost struct {
	widget.BaseWidget
	scroll *container.Scroll
	list   *widget.List
}

func (b *scrollBoost) Scrolled(ev *fyne.ScrollEvent) {
	dx := ev.Scrolled.DX * scrollFactor
	dy := ev.Scrolled.DY * scrollFactor
	switch {
	case b.scroll != nil:
		e := *ev
		e.Scrolled = fyne.NewDelta(dx, dy)
		b.scroll.Scrolled(&e)
	case b.list != nil:
		// Offsets grow downwards while wheel-down is negative, matching
		// Scroll.updateOffset's use of -delta.
		b.list.ScrollToOffset(b.list.GetScrollOffset() - dy)
	}
}

func (b *scrollBoost) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

// boostScroll returns the scroll container with a sensitivity overlay on top.
func boostScroll(sc *container.Scroll) fyne.CanvasObject {
	b := &scrollBoost{scroll: sc}
	b.ExtendBaseWidget(b)
	return container.NewStack(sc, b)
}

// boostList does the same for a widget.List, which owns its scroller privately.
func boostList(l *widget.List) fyne.CanvasObject {
	b := &scrollBoost{list: l}
	b.ExtendBaseWidget(b)
	return container.NewStack(l, b)
}

// ---------------------------------------------------------------- card flow

// cardFlowLayout arranges cards into as many columns as the width allows,
// adding each to the shortest column so the columns stay balanced.
//
// Fyne ships no flex layout: GridWrap forces one cell size on every child,
// which cannot work for cards whose height depends on their content.
type cardFlowLayout struct {
	minCol float32
	maxCol int
	gap    float32

	// Height of the last layout. Cards containing wrapped text only report a
	// true MinSize once they have a width, so MinSize reuses what Layout
	// measured rather than guessing.
	lastH float32
}

func (c *cardFlowLayout) columns(width float32) int {
	n := int((width + c.gap) / (c.minCol + c.gap))
	if n < 1 {
		n = 1
	}
	if c.maxCol > 0 && n > c.maxCol {
		n = c.maxCol
	}
	return n
}

// fullWidthCard is a flow child that occupies the whole row instead of one
// masonry cell, so long content (file paths) is not clipped to a column.
type fullWidthCard struct{ fyne.CanvasObject }

func fullRow(obj fyne.CanvasObject) fyne.CanvasObject {
	return &fullWidthCard{CanvasObject: obj}
}

func flowItemWidth(o fyne.CanvasObject, colW, total float32) float32 {
	if _, ok := o.(*fullWidthCard); ok {
		return total
	}
	return colW
}

func (c *cardFlowLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	n := c.columns(size.Width)
	colW := (size.Width - c.gap*float32(n-1)) / float32(n)

	// First pass sets the width, so wrapping children can remeasure; the second
	// reads the settled height and places them.
	for _, o := range objs {
		if o.Visible() {
			o.Resize(fyne.NewSize(flowItemWidth(o, colW, size.Width), o.MinSize().Height))
		}
	}

	heights := make([]float32, n)
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		h := o.MinSize().Height
		if _, ok := o.(*fullWidthCard); ok {
			var y float32
			for _, colH := range heights {
				y = max32(y, colH)
			}
			o.Move(fyne.NewPos(0, y))
			o.Resize(fyne.NewSize(size.Width, h))
			next := y + h + c.gap
			for i := range heights {
				heights[i] = next
			}
			continue
		}
		col := 0
		for i := 1; i < n; i++ {
			if heights[i] < heights[col] {
				col = i
			}
		}
		o.Move(fyne.NewPos(float32(col)*(colW+c.gap), heights[col]))
		o.Resize(fyne.NewSize(colW, h))
		heights[col] += h + c.gap
	}

	var tallest float32
	for _, h := range heights {
		tallest = max32(tallest, h)
	}
	c.lastH = max32(0, tallest-c.gap)
}

func (c *cardFlowLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if c.lastH > 0 {
		return fyne.NewSize(c.minCol, c.lastH)
	}
	// Before the first layout, assume a single column.
	var h float32
	n := 0
	for _, o := range objs {
		if !o.Visible() {
			continue
		}
		h += o.MinSize().Height
		n++
	}
	if n > 1 {
		h += c.gap * float32(n-1)
	}
	return fyne.NewSize(c.minCol, h)
}

// ---------------------------------------------------------------- spacers

func spacer(w, h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(w, h))
	return r
}

func vspace(h float32) fyne.CanvasObject { return spacer(0, h) }
func hspace(w float32) fyne.CanvasObject { return spacer(w, 0) }

// divider is a hairline that spans the width it is given.
func divider() fyne.CanvasObject {
	r := canvas.NewRectangle(pal().Divider)
	r.SetMinSize(fyne.NewSize(0, z(1)))
	return r
}

func strongDivider() fyne.CanvasObject {
	r := canvas.NewRectangle(pal().Base300)
	r.SetMinSize(fyne.NewSize(0, z(1)))
	return r
}

// ---------------------------------------------------------------- text

// text is the low-level primitive for custom renderers, where positioning is
// handled by hand. Flowing content should use the rich* helpers so every block
// shares the same internal inset and stays aligned.
func text(s string, size float32, c color.Color, bold bool) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: bold}
	return t
}

func monoText(s string, size float32, c color.Color) *canvas.Text {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

func rich(s string, size fyne.ThemeSizeName, col fyne.ThemeColorName, bold bool) fyne.CanvasObject {
	seg := &widget.TextSegment{Text: s, Style: widget.RichTextStyle{
		ColorName: col,
		SizeName:  size,
		TextStyle: fyne.TextStyle{Bold: bold},
		Inline:    true,
	}}
	rt := widget.NewRichText(seg)
	rt.Wrapping = fyne.TextWrapWord
	return container.New(insetLayout{-rtPad, -rtPad, -rtPad, -rtPad}, rt)
}

// cardTitle / body / hint form the type hierarchy for flowing content.
func cardTitle(s string) fyne.CanvasObject {
	return rich(s, theme.SizeNameSubHeadingText, colContent, true)
}

func body(s string) fyne.CanvasObject {
	return rich(s, theme.SizeNameText, colContent, false)
}

func hint(s string) fyne.CanvasObject {
	return rich(s, sizeSmall, colMuted, false)
}

func fieldLabel(s string) fyne.CanvasObject {
	return text(s, fsSmall, pal().Muted, false)
}

// eyebrow is the small uppercase group label used in the sidebar.
func eyebrow(s string) fyne.CanvasObject {
	return text(s, fsCaption, pal().Faint, true)
}

// ---------------------------------------------------------------- tap wrapper

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

// ---------------------------------------------------------------- surfaces

// surface is a rounded panel: the single source of card styling.
func surface(radius float32, fill, stroke color.Color) *canvas.Rectangle {
	r := canvas.NewRectangle(fill)
	r.CornerRadius = radius
	if stroke != nil {
		r.StrokeColor = stroke
		r.StrokeWidth = 1
	}
	return r
}

// cardBox is the standard panel: optional title, description and header
// actions above a body, all on one 16px inset.
func cardBox(title, desc string, actions fyne.CanvasObject, content fyne.CanvasObject) fyne.CanvasObject {
	p := pal()
	rows := make([]fyne.CanvasObject, 0, 4)
	if title != "" || actions != nil {
		rows = append(rows, splitRow(nilIfEmpty(title, func() fyne.CanvasObject { return cardTitle(title) }), actions))
	}
	if desc != "" {
		rows = append(rows, hint(desc))
	}
	if content != nil {
		if len(rows) > 0 {
			rows = append(rows, vspace(sp1))
		}
		rows = append(rows, content)
	}
	inner := vstack(sp2, rows...)
	return container.NewStack(surface(radLg, p.Base100, p.Base300), inset(sp4, inner))
}

func card(title, desc string, content fyne.CanvasObject) fyne.CanvasObject {
	return cardBox(title, desc, nil, content)
}

func nilIfEmpty(s string, fn func() fyne.CanvasObject) fyne.CanvasObject {
	if s == "" {
		return nil
	}
	return fn()
}

// ---------------------------------------------------------------- indicators

type tone int

const (
	toneNeutral tone = iota
	toneSuccess
	toneDanger
	toneWarning
	toneInfo
	tonePrimary
)

func toneColors(t tone) (fg, bg color.NRGBA) {
	p := pal()
	switch t {
	case toneSuccess:
		return p.Success, p.SuccessSoft
	case toneDanger:
		return p.Error, p.ErrorSoft
	case toneWarning:
		return p.Warning, p.WarningSoft
	case toneInfo:
		return p.Info, p.InfoSoft
	case tonePrimary:
		return p.Primary, p.PrimarySoft
	default:
		return p.Muted, withAlpha(p.Faint, 30)
	}
}

// badge is a small rounded status chip.
func badge(label string, t tone) fyne.CanvasObject {
	fg, bg := toneColors(t)
	r := surface(radFull, bg, nil)
	lbl := text(label, fsCaption, fg, true)
	return container.NewStack(r, insetXY(sp2, z(3), lbl))
}

// statTile is a compact metric block for page summaries.
func statTile(label, value string, t tone) fyne.CanvasObject {
	fg, _ := toneColors(t)
	if t == toneNeutral {
		fg = pal().Content
	}
	v := text(value, fsLarge, fg, true)
	l := text(label, fsCaption, pal().Muted, false)
	inner := vstack(z(2), v, l)
	return container.NewStack(surface(radMd, pal().Base100, pal().Base300), insetXY(sp3, sp2, inner))
}

// kvRow is one label/value line with a trailing hairline. Long values wrap
// and the row grows with them — canvas.Text in a Border was clipping paths.
func kvRow(label, value string, mono bool) fyne.CanvasObject {
	if value == "" {
		value = "—"
	}
	w := &kvLine{label: label, value: value, mono: mono}
	w.ExtendBaseWidget(w)
	return vstack(0, insetEach(sp2+1, 0, sp2+1, 0, w), divider())
}

// kvLine is the wrapping label/value pair inside kvRow.
type kvLine struct {
	widget.BaseWidget
	label string
	value string
	mono  bool
}

func (k *kvLine) CreateRenderer() fyne.WidgetRenderer {
	r := &kvLineRenderer{k: k, label: text(k.label, fsBody, pal().Muted, false)}
	r.ensureValues(16)
	r.apply()
	return r
}

type kvLineRenderer struct {
	k      *kvLine
	label  *canvas.Text
	values []*canvas.Text
}

func (r *kvLineRenderer) Destroy() {}

func (r *kvLineRenderer) Objects() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, 1+len(r.values))
	out = append(out, r.label)
	for _, v := range r.values {
		out = append(out, v)
	}
	return out
}

func (k *kvLine) valueStyle() (fyne.TextStyle, float32) {
	if k.mono {
		return fyne.TextStyle{Monospace: true}, fsSmall
	}
	return fyne.TextStyle{}, fsBody
}

func (k *kvLine) minForWidth(width float32) fyne.Size {
	style, size := k.valueStyle()
	lw := fyne.MeasureText(k.label, fsBody, fyne.TextStyle{}).Width
	lh := fyne.MeasureText(k.label, fsBody, fyne.TextStyle{}).Height
	vw := fyne.MeasureText(k.value, size, style).Width
	vh := fyne.MeasureText("Ag", size, style).Height
	gap := sp3
	if width <= 0 {
		return fyne.NewSize(lw+gap+vw, max32(lh, vh))
	}
	remain := width - lw - gap
	if remain < z(80) {
		lines := wrapToWidth(k.value, width, size, style)
		return fyne.NewSize(width, lh+z(2)+vh*float32(len(lines)))
	}
	lines := wrapToWidth(k.value, remain, size, style)
	return fyne.NewSize(max32(width, lw+gap+z(80)), max32(lh, vh*float32(len(lines))))
}

func (r *kvLineRenderer) MinSize() fyne.Size {
	return r.k.minForWidth(r.k.Size().Width)
}

func (r *kvLineRenderer) ensureValues(n int) {
	for len(r.values) < n {
		t := canvas.NewText("", pal().Content)
		r.values = append(r.values, t)
	}
}

func (r *kvLineRenderer) Layout(size fyne.Size) {
	style, textSize := r.k.valueStyle()
	lw := r.label.MinSize().Width
	lh := r.label.MinSize().Height
	vh := fyne.MeasureText("Ag", textSize, style).Height
	gap := sp3
	remain := size.Width - lw - gap
	stack := remain < z(80)

	var lines []string
	var valueX float32
	var valueY float32
	if stack {
		lines = wrapToWidth(r.k.value, size.Width, textSize, style)
		valueX, valueY = 0, lh+z(2)
		r.label.Move(fyne.NewPos(0, 0))
	} else {
		lines = wrapToWidth(r.k.value, remain, textSize, style)
		valueX, valueY = lw+gap, 0
		r.label.Move(fyne.NewPos(0, 0))
	}
	r.label.Resize(r.label.MinSize())

	r.ensureValues(len(lines))
	for i, t := range r.values {
		if i >= len(lines) {
			t.Text = ""
			t.Hide()
			t.Resize(fyne.NewSize(0, 0))
			continue
		}
		t.Show()
		t.Text = lines[i]
		t.TextSize = textSize
		t.TextStyle = style
		t.Color = pal().Content
		ms := t.MinSize()
		x := valueX
		if !stack && len(lines) == 1 {
			x = size.Width - ms.Width
		}
		t.Move(fyne.NewPos(x, valueY+vh*float32(i)))
		t.Resize(ms)
	}
}

func (r *kvLineRenderer) Refresh() {
	r.apply()
	r.label.Refresh()
	for _, v := range r.values {
		v.Refresh()
	}
	if sz := r.k.Size(); sz.Width > 0 {
		r.Layout(sz)
	}
	canvasRefresh(r.k)
}

func (r *kvLineRenderer) apply() {
	p := pal()
	r.label.Text = r.k.label
	r.label.TextSize = fsBody
	r.label.Color = p.Muted
}

// wrapToWidth breaks s so each line fits maxW. Paths have no spaces, so it
// wraps on characters and prefers a cut after / or \ when one is nearby.
func wrapToWidth(s string, maxW, textSize float32, style fyne.TextStyle) []string {
	s = strings.TrimRight(s, " \t")
	if s == "" {
		return []string{""}
	}
	if maxW <= 0 || fyne.MeasureText(s, textSize, style).Width <= maxW {
		return []string{s}
	}
	var lines []string
	rest := []rune(s)
	for len(rest) > 0 {
		if strings.TrimSpace(string(rest)) == "" {
			break
		}
		lo, hi := 1, len(rest)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if fyne.MeasureText(string(rest[:mid]), textSize, style).Width <= maxW {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		cut := lo
		if cut < len(rest) {
			pref := rest[:cut]
			from := len(pref) * 2 / 3
			for i := len(pref) - 1; i >= from; i-- {
				switch pref[i] {
				case '/', '\\', ' ':
					cut = i + 1
					i = from - 1
				}
			}
		}
		if cut < 1 {
			cut = 1
		}
		lines = append(lines, strings.TrimRight(string(rest[:cut]), " \t"))
		rest = rest[cut:]
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// ---------------------------------------------------------------- empty state

func emptyState(title, desc string) fyne.CanvasObject {
	items := []fyne.CanvasObject{container.NewCenter(text(title, fsBody, pal().Muted, true))}
	if desc != "" {
		items = append(items, container.NewCenter(text(desc, fsSmall, pal().Faint, false)))
	}
	return container.NewCenter(vstack(sp2, items...))
}

func emptyRow(msg string) fyne.CanvasObject {
	return insetEach(sp2, 0, sp2, 0, rich(msg, sizeSmall, colFaint, false))
}

// ---------------------------------------------------------------- toast

// toastCard is a self-sizing notification so the shell can place it exactly.
type toastCard struct {
	widget.BaseWidget
	kind string
	msg  string
}

func newToast(kind, msg string) *toastCard {
	t := &toastCard{kind: kind, msg: msg}
	t.ExtendBaseWidget(t)
	return t
}

func (t *toastCard) CreateRenderer() fyne.WidgetRenderer {
	p := pal()
	accent := p.Success
	icon := theme.ConfirmIcon()
	switch t.kind {
	case "error":
		accent, icon = p.Error, theme.ErrorIcon()
	case "info":
		accent, icon = p.Primary, theme.InfoIcon()
	}
	bg := surface(radMd, p.Base100, p.Base300)
	bar := canvas.NewRectangle(accent)
	bar.CornerRadius = radFull
	ico := widget.NewIcon(icon)
	lbl := widget.NewLabel(t.msg)
	lbl.Wrapping = fyne.TextWrapWord
	return &toastRenderer{t: t, bg: bg, bar: bar, ico: ico, lbl: lbl,
		objects: []fyne.CanvasObject{bg, bar, ico, lbl}}
}

type toastRenderer struct {
	t       *toastCard
	bg      *canvas.Rectangle
	bar     *canvas.Rectangle
	ico     *widget.Icon
	lbl     *widget.Label
	objects []fyne.CanvasObject
}

func (r *toastRenderer) Destroy()                     {}
func (r *toastRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *toastRenderer) MinSize() fyne.Size {
	h := max32(z(44), r.lbl.MinSize().Height+sp3)
	return fyne.NewSize(z(320), h)
}

func (r *toastRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bar.Resize(fyne.NewSize(z(3), size.Height-sp3))
	r.bar.Move(fyne.NewPos(sp2, sp2-z(1)))
	r.ico.Resize(fyne.NewSize(iconSize, iconSize))
	r.ico.Move(fyne.NewPos(sp4, (size.Height-iconSize)/2))
	lw := size.Width - sp4 - iconSize - sp2 - sp3
	lh := r.lbl.MinSize().Height
	r.lbl.Resize(fyne.NewSize(lw, lh))
	r.lbl.Move(fyne.NewPos(sp4+iconSize+sp1, (size.Height-lh)/2))
}

func (r *toastRenderer) Refresh() {
	canvasRefresh(r.t)
}

// ---------------------------------------------------------------- shell

// shellLayout stacks the app content with a top-right toast overlay.
type shellLayout struct{}

func (shellLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 2 {
		return
	}
	objs[0].Move(fyne.NewPos(0, 0))
	objs[0].Resize(size)

	// Overlays are placed independently: an early return for an empty toast
	// used to skip the loader entirely, leaving it parked at 0,0 over the
	// sidebar — which is the normal case, since a toggle shows a loader and no
	// toast.
	//
	// Toast bottom right (the top right is where page actions live, and
	// covering the search box or Connect buttons is worse than no toast);
	// loader bottom left, so both can be on screen at once.
	placeToast(objs[1], size, true)

	// The loader covers the whole window: it carries its own scrim and centres
	// its card, so it is sized to the shell rather than parked in a corner.
	if len(objs) > 2 {
		busy := objs[2]
		if busy.MinSize().Height < 8 {
			busy.Resize(fyne.NewSize(0, 0))
			busy.Move(fyne.NewPos(size.Width, size.Height))
		} else {
			busy.Move(fyne.NewPos(0, 0))
			busy.Resize(size)
		}
	}
}

// placeToast parks an overlay in a bottom corner, or collapses it out of the
// way when it has no content.
func placeToast(o fyne.CanvasObject, size fyne.Size, trailing bool) {
	ms := o.MinSize()
	if ms.Width < 8 || ms.Height < 8 {
		o.Resize(fyne.NewSize(0, 0))
		o.Move(fyne.NewPos(size.Width, size.Height))
		return
	}
	w := min32(ms.Width, size.Width-2*sp5)
	o.Resize(fyne.NewSize(w, ms.Height))
	x := sp5
	if trailing {
		x = size.Width - w - sp5
	}
	o.Move(fyne.NewPos(x, size.Height-ms.Height-sp5))
}

func (shellLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	// Kept deliberately low: this is a logical size, so zoom multiplies it into
	// physical pixels. At 200% a larger floor would demand a window taller than
	// a 1080p display can show.
	return fyne.NewSize(720, 480)
}

// ---------------------------------------------------------------- brand

// brandMark draws the Tunnels glyph: an upright bar crossed by a primary bar,
// matching assets/logo.svg without needing a themed raster.
func brandMark(size float32) fyne.CanvasObject {
	p := pal()
	stem := canvas.NewRectangle(p.Content)
	stem.CornerRadius = size * 0.12
	bar := canvas.NewRectangle(p.Primary)
	bar.CornerRadius = size * 0.12
	return container.NewStack(spacer(size, size), container.New(markLayout{size: size}, stem, bar))
}

type markLayout struct{ size float32 }

func (m markLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) < 2 {
		return
	}
	s := min32(size.Width, size.Height)
	ox, oy := (size.Width-s)/2, (size.Height-s)/2
	stemW := s * 0.24
	objs[0].Resize(fyne.NewSize(stemW, s*0.78))
	objs[0].Move(fyne.NewPos(ox+(s-stemW)/2, oy+s*0.22))
	barH := s * 0.24
	objs[1].Resize(fyne.NewSize(s, barH))
	objs[1].Move(fyne.NewPos(ox, oy+s*0.10))
}

func (m markLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(m.size, m.size)
}

// ---------------------------------------------------------------- helpers

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
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
