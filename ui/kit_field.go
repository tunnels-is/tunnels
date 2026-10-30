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

type fixedLayout struct{ w, h float32 }

// Layout ...
func (f fixedLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

// MinSize ...
func (f fixedLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var h float32 = f.h
	for _, o := range objs {
		h = max32(h, o.MinSize().Height)
	}
	return fyne.NewSize(f.w, h)
}

func fixedWidth(w float32, obj fyne.CanvasObject) *fyne.Container {
	return container.New(fixedLayout{w: w}, obj)
}

func kEntry(placeholder, value string) *widget.Entry {
	e := widget.NewEntry()
	e.SetPlaceHolder(placeholder)
	if value != "" {
		e.SetText(value)
	}
	return e
}

func kPassword(placeholder string) *widget.Entry {
	e := widget.NewPasswordEntry()
	e.SetPlaceHolder(placeholder)
	return e
}

func kMultiline(value string, rows int) *widget.Entry {
	e := widget.NewMultiLineEntry()
	e.SetText(value)
	e.SetMinRowsVisible(rows)
	e.Wrapping = fyne.TextWrapOff
	return e
}

func field(label string, obj fyne.CanvasObject) fyne.CanvasObject {
	return vstack(sp1+2, fieldLabel(label), obj)
}

func fieldWith(label, note string, obj fyne.CanvasObject) fyne.CanvasObject {
	return vstack(sp1+2, fieldLabel(label), obj, hint(note))
}

func formRows(objs ...fyne.CanvasObject) fyne.CanvasObject {
	return capWidth(formWidth, vstack(sp3, objs...))
}

func formPair(a, b fyne.CanvasObject) fyne.CanvasObject {
	return container.New(equalColsLayout{gap: sp3}, a, b)
}

type equalColsLayout struct{ gap float32 }

// Layout ...
func (e equalColsLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) == 0 {
		return
	}
	n := float32(len(objs))
	w := (size.Width - e.gap*(n-1)) / n
	x := float32(0)
	for _, o := range objs {
		o.Move(fyne.NewPos(x, 0))
		o.Resize(fyne.NewSize(w, size.Height))
		x += w + e.gap
	}
}

// MinSize ...
func (e equalColsLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objs {
		ms := o.MinSize()
		w = max32(w, ms.Width)
		h = max32(h, ms.Height)
	}
	return fyne.NewSize(w*float32(len(objs))+e.gap*float32(len(objs)-1), h)
}

func kSelect(opts []string, selected string, onChange func(string)) *widget.Select {
	s := widget.NewSelect(opts, onChange)
	if selected != "" {
		s.SetSelected(selected)
	}
	return s
}

func searchField(placeholder, value string, onChange, onSubmit func(string)) (*widget.Entry, fyne.CanvasObject) {
	e := widget.NewEntry()
	icon := iconImage(theme.NewColoredResource(theme.SearchIcon(), colFaint))
	e.ActionItem = fixedWidth(iconSize, icon)
	e.SetPlaceHolder(placeholder)
	e.SetText(value)
	e.OnChanged = onChange
	e.OnSubmitted = onSubmit
	return e, fixedWidth(searchWidth, e)
}

type weightedColsLayout struct {
	weights []float32
	gap     float32
}

// Layout ...
func (w weightedColsLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) == 0 {
		return
	}
	var total float32
	for i := range objs {
		total += w.weight(i)
	}
	avail := size.Width - w.gap*float32(len(objs)-1)
	x := float32(0)
	for i, o := range objs {
		cw := avail * w.weight(i) / total
		o.Move(fyne.NewPos(x, 0))
		o.Resize(fyne.NewSize(cw, size.Height))
		x += cw + w.gap
	}
}

func (w weightedColsLayout) weight(i int) float32 {
	if i < len(w.weights) && w.weights[i] > 0 {
		return w.weights[i]
	}
	return 1
}

// MinSize ...
func (w weightedColsLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var width, h float32
	for _, o := range objs {
		ms := o.MinSize()
		width += ms.Width
		h = max32(h, ms.Height)
	}
	if len(objs) > 1 {
		width += w.gap * float32(len(objs)-1)
	}
	return fyne.NewSize(width, h)
}

type fieldCol struct {
	label       string
	placeholder string
	weight      float32
}

type rowEditor struct {
	noun   string
	cols   []fieldCol
	rows   [][]*widget.Entry
	empty  string
	box    *fyne.Container
	reflow func()
}

func newRowEditor(noun string, cols []fieldCol, values [][]string, empty string, reflow func()) *rowEditor {
	e := &rowEditor{noun: noun, cols: cols, empty: empty, box: container.NewStack(), reflow: reflow}
	for _, v := range values {
		e.appendRow(v)
	}
	e.render()
	return e
}

func (e *rowEditor) appendRow(vals []string) {
	row := make([]*widget.Entry, len(e.cols))
	for i, c := range e.cols {
		v := ""
		if i < len(vals) {
			v = vals[i]
		}
		row[i] = kEntry(c.placeholder, v)
	}
	e.rows = append(e.rows, row)
}

func (e *rowEditor) removeRow(i int) {
	if i < 0 || i >= len(e.rows) {
		return
	}
	e.rows = append(e.rows[:i], e.rows[i+1:]...)
	e.render()
}

func (e *rowEditor) weights() []float32 {
	w := make([]float32, len(e.cols))
	for i, c := range e.cols {
		w[i] = c.weight
	}
	return w
}

func (e *rowEditor) render() {
	items := make([]fyne.CanvasObject, 0, len(e.rows)+2)

	if len(e.cols) > 1 {
		heads := make([]fyne.CanvasObject, len(e.cols))
		for i, c := range e.cols {
			heads[i] = fieldLabel(c.label)
		}
		items = append(items, container.NewBorder(nil, nil, nil, hspace(ctrlHeight+sp2),
			container.New(weightedColsLayout{weights: e.weights(), gap: sp2}, heads...)))
	}

	for i, row := range e.rows {
		idx := i
		cells := make([]fyne.CanvasObject, len(row))
		for j := range row {
			cells[j] = row[j]
		}
		del := newIconBtn(theme.DeleteIcon(), kGhost, func() { e.removeRow(idx) })
		items = append(items, container.NewBorder(nil, nil, nil, hstack(0, hspace(sp2), del),
			container.New(weightedColsLayout{weights: e.weights(), gap: sp2}, cells...)))
	}

	if len(e.rows) == 0 && e.empty != "" {
		items = append(items, hint(e.empty))
	}

	add := outlineBtn("Add "+e.noun, func() {
		e.appendRow(nil)
		e.render()
	}).withIcon(theme.ContentAddIcon()).small()
	items = append(items, hstack(0, add))

	e.box.Objects = []fyne.CanvasObject{vstack(sp2, items...)}
	e.box.Refresh()
	if e.reflow != nil {
		e.reflow()
	}
}

func (e *rowEditor) values() [][]string {
	out := make([][]string, 0, len(e.rows))
	for _, row := range e.rows {
		vals := make([]string, len(row))
		any := false
		for i, en := range row {
			vals[i] = strings.TrimSpace(en.Text)
			if vals[i] != "" {
				any = true
			}
		}
		if any {
			out = append(out, vals)
		}
	}
	return out
}

func (e *rowEditor) column(i int) []string {
	var out []string
	for _, v := range e.values() {
		if i < len(v) && v[i] != "" {
			out = append(out, v[i])
		}
	}
	return out
}

func (e *rowEditor) object() fyne.CanvasObject { return e.box }

type kSwitch struct {
	widget.BaseWidget
	on       bool
	hovered  bool
	disabled bool
	onChange func(bool)
}

func newSwitch(on bool, onChange func(bool)) *kSwitch {
	s := &kSwitch{on: on, onChange: onChange}
	s.ExtendBaseWidget(s)
	return s
}

// Tapped ...
func (s *kSwitch) Tapped(*fyne.PointEvent) {
	if s.disabled {
		return
	}
	s.on = !s.on
	s.Refresh()
	if s.onChange != nil {
		s.onChange(s.on)
	}
}

// MouseIn ...
func (s *kSwitch) MouseIn(*desktop.MouseEvent) { s.hovered = true; s.Refresh() }

// MouseOut ...
func (s *kSwitch) MouseOut() { s.hovered = false; s.Refresh() }

// MouseMoved ...
func (s *kSwitch) MouseMoved(*desktop.MouseEvent) {}

// Cursor ...
func (s *kSwitch) Cursor() desktop.Cursor {
	if s.disabled {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

// CreateRenderer ...
func (s *kSwitch) CreateRenderer() fyne.WidgetRenderer {
	track := surface(radFull, color.Transparent, nil)
	knob := canvas.NewCircle(color.White)
	r := &switchRenderer{s: s, track: track, knob: knob}
	r.apply()
	return r
}

type switchRenderer struct {
	s     *kSwitch
	track *canvas.Rectangle
	knob  *canvas.Circle
	objs  []fyne.CanvasObject
}

// Destroy ...
func (r *switchRenderer) Destroy() {}

// Objects ...
func (r *switchRenderer) Objects() []fyne.CanvasObject {
	if r.objs == nil {
		r.objs = []fyne.CanvasObject{r.track, r.knob}
	}
	return r.objs
}

// MinSize ...
func (r *switchRenderer) MinSize() fyne.Size { return fyne.NewSize(swWidth, swHeight) }

// Layout ...
func (r *switchRenderer) Layout(size fyne.Size) {
	y := (size.Height - swHeight) / 2
	r.track.Resize(fyne.NewSize(swWidth, swHeight))
	r.track.Move(fyne.NewPos(0, y))
	off := (swHeight - swKnob) / 2
	x := off
	if r.s.on {
		x = swWidth - swKnob - off
	}
	r.knob.Resize(fyne.NewSize(swKnob, swKnob))
	r.knob.Move(fyne.NewPos(x, y+off))
}

// Refresh ...
func (r *switchRenderer) Refresh() {
	r.apply()
	r.Layout(r.s.Size())
	r.track.Refresh()
	r.knob.Refresh()
	canvasRefresh(r.s)
}

func (r *switchRenderer) apply() {
	p := pal()
	if r.s.on {
		r.track.FillColor = p.Primary
		if r.s.hovered {
			r.track.FillColor = p.PrimaryHover
		}
		r.track.StrokeWidth = 0
		r.knob.FillColor = p.PrimaryContent
	} else {
		r.track.FillColor = p.Elevate
		r.track.StrokeColor = p.Base300
		r.track.StrokeWidth = 1
		r.knob.FillColor = p.Faint
		if r.s.hovered {
			r.knob.FillColor = p.Muted
		}
	}
	if r.s.disabled {
		r.track.FillColor = withAlpha(toNRGBA(r.track.FillColor), 90)
		r.knob.FillColor = withAlpha(toNRGBA(r.knob.FillColor), 120)
	}
}

func toNRGBA(c color.Color) color.NRGBA {
	if n, ok := c.(color.NRGBA); ok {
		return n
	}
	r, g, b, a := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func settingRow(title, desc string, control fyne.CanvasObject) fyne.CanvasObject {
	left := []fyne.CanvasObject{body(title)}
	if desc != "" {
		left = append(left, hint(desc))
	}
	row := splitRow(vstack(2, left...), control)
	return insetEach(sp3, 0, sp3, 0, row)
}

func settingList(rows ...fyne.CanvasObject) fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, len(rows)*2)
	for i, r := range rows {
		if i > 0 {
			out = append(out, divider())
		}
		out = append(out, r)
	}
	return vstack(0, out...)
}

func toggleRow(title, desc string, on bool, onChange func(bool)) fyne.CanvasObject {
	return settingRow(title, desc, newSwitch(on, onChange))
}

type segItem struct {
	key   string
	label string
}

type segControl struct {
	widget.BaseWidget
	items    []segItem
	widths   []float32
	active   string
	hover    int
	onSelect func(string)
}

func newSegmented(items []segItem, active string, onSelect func(string)) *segControl {
	s := &segControl{items: items, active: active, hover: -1, onSelect: onSelect}
	s.widths = make([]float32, len(items))
	for i, it := range items {
		s.widths[i] = text(it.label, fsSmall, color.Black, true).MinSize().Width + sp3*2
	}
	s.ExtendBaseWidget(s)
	return s
}

func (s *segControl) index(x float32) int {
	cur := segInset
	for i := range s.items {
		w := s.itemWidth(i)
		if x >= cur && x < cur+w {
			return i
		}
		cur += w
	}
	return -1
}

func (s *segControl) itemWidth(i int) float32 {
	if i < 0 || i >= len(s.widths) {
		return 0
	}
	return s.widths[i]
}

// Tapped ...
func (s *segControl) Tapped(e *fyne.PointEvent) {
	if i := s.index(e.Position.X); i >= 0 {
		s.active = s.items[i].key
		s.Refresh()
		if s.onSelect != nil {
			s.onSelect(s.items[i].key)
		}
	}
}

// MouseIn ...
func (s *segControl) MouseIn(e *desktop.MouseEvent) { s.MouseMoved(e) }

// MouseOut ...
func (s *segControl) MouseOut() { s.hover = -1; s.Refresh() }

// MouseMoved ...
func (s *segControl) MouseMoved(e *desktop.MouseEvent) {
	if i := s.index(e.Position.X); i != s.hover {
		s.hover = i
		s.Refresh()
	}
}

// Cursor ...
func (s *segControl) Cursor() desktop.Cursor { return desktop.PointerCursor }

// CreateRenderer ...
func (s *segControl) CreateRenderer() fyne.WidgetRenderer {
	r := &segRenderer{s: s, bg: surface(radMd, pal().Elevate, pal().Base300)}
	for range s.items {
		r.pills = append(r.pills, surface(radSm, color.Transparent, nil))
		r.labels = append(r.labels, text("", fsSmall, pal().Muted, true))
	}
	r.apply()
	return r
}

type segRenderer struct {
	s      *segControl
	bg     *canvas.Rectangle
	pills  []*canvas.Rectangle
	labels []*canvas.Text
	objs   []fyne.CanvasObject
}

// Destroy ...
func (r *segRenderer) Destroy() {}

// Objects ...
func (r *segRenderer) Objects() []fyne.CanvasObject {
	if r.objs == nil {
		r.objs = make([]fyne.CanvasObject, 0, len(r.pills)*2+1)
		r.objs = append(r.objs, r.bg)
		for i := range r.pills {
			r.objs = append(r.objs, r.pills[i])
		}
		for i := range r.labels {
			r.objs = append(r.objs, r.labels[i])
		}
	}
	return r.objs
}

// MinSize ...
func (r *segRenderer) MinSize() fyne.Size {
	var w float32
	for i := range r.s.items {
		w += r.s.itemWidth(i)
	}
	return fyne.NewSize(w+segInset*2, segHeight)
}

// Layout ...
func (r *segRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	x := segInset
	for i := range r.s.items {
		w := r.s.itemWidth(i)
		r.pills[i].Resize(fyne.NewSize(w, size.Height-segInset*2))
		r.pills[i].Move(fyne.NewPos(x, segInset))
		lms := r.labels[i].MinSize()
		r.labels[i].Move(fyne.NewPos(x+(w-lms.Width)/2, (size.Height-lms.Height)/2))
		x += w
	}
}

// Refresh ...
func (r *segRenderer) Refresh() {
	r.apply()
	r.bg.Refresh()
	for i := range r.pills {
		r.pills[i].Refresh()
		r.labels[i].Refresh()
	}
	canvasRefresh(r.s)
}

func (r *segRenderer) apply() {
	p := pal()
	r.bg.FillColor = p.Elevate
	r.bg.StrokeColor = p.Base300
	for i, it := range r.s.items {
		r.labels[i].Text = it.label
		switch {
		case it.key == r.s.active:
			r.pills[i].FillColor = p.Base100
			r.pills[i].StrokeColor = p.Base300
			r.pills[i].StrokeWidth = 1
			r.labels[i].Color = p.Content
		case i == r.s.hover:
			r.pills[i].FillColor = p.Hover
			r.pills[i].StrokeWidth = 0
			r.labels[i].Color = p.Content
		default:
			r.pills[i].FillColor = color.Transparent
			r.pills[i].StrokeWidth = 0
			r.labels[i].Color = p.Muted
		}
	}
}

func linkRow(a *App, name, dest, href string) fyne.CanvasObject {
	p := pal()
	title := text(name, fsBody, p.Content, false)
	sub := text(dest, fsSmall, p.Faint, false)
	go2 := newIconBtn(theme.NavigateNextIcon(), kGhost, func() { a.openURL(href) })
	row := splitRow(vstack(1, title, sub), go2)
	return newTap(insetEach(sp2, 0, sp2, 0, row), func() { a.openURL(href) })
}

func pageShell(title, subtitle string, actions fyne.CanvasObject, content fyne.CanvasObject) fyne.CanvasObject {
	return buildShell(title, subtitle, actions, content, true)
}

func pageShellFlush(title, subtitle string, actions fyne.CanvasObject, content fyne.CanvasObject) fyne.CanvasObject {
	return buildShell(title, subtitle, actions, content, false)
}

func buildShell(title, subtitle string, actions, content fyne.CanvasObject, rule bool) fyne.CanvasObject {
	titleBlock := []fyne.CanvasObject{text(title, fsTitle, pal().Content, true)}
	if subtitle != "" {
		titleBlock = append(titleBlock, text(subtitle, fsSmall, pal().Muted, false))
	}
	head := splitRow(vstack(sp1+1, titleBlock...), actions)
	rows := []fyne.CanvasObject{insetEach(sp5, gutter, sp5, gutter, head)}
	if rule {
		rows = append(rows, strongDivider())
	}
	return container.NewBorder(vstack(0, rows...), nil, nil, nil, content)
}

func scrollFlow(objs ...fyne.CanvasObject) *fyne.Container {
	return container.New(&cardFlowLayout{minCol: z(430), maxCol: 3, gap: sp4}, objs...)
}

func scrollBody(objs ...fyne.CanvasObject) fyne.CanvasObject {
	return scrollBodyOf(scrollFlow(objs...))
}

func wrapBody(objs ...fyne.CanvasObject) fyne.CanvasObject {
	return scrollBodyOf(wrapFlow(objs...))
}

func scrollBodyOf(col fyne.CanvasObject) fyne.CanvasObject {
	return boostScroll(container.NewVScroll(insetEach(sp5, gutter, sp8, gutter, col)))
}

func centredBody(width float32, objs ...fyne.CanvasObject) fyne.CanvasObject {
	col := vstack(sp4, objs...)
	return boostScroll(container.NewVScroll(
		container.NewCenter(container.New(fixedLayout{w: width}, col))))
}

type stageLayout struct{ w float32 }

// Layout ...
func (s stageLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := s.w
	if size.Width > 0 && w > size.Width {
		w = size.Width
	}
	for _, o := range objs {
		if o == nil || !o.Visible() {
			continue
		}
		o.Resize(fyne.NewSize(w, o.MinSize().Height))
		h := o.MinSize().Height
		y := float32(0)
		if h < size.Height {
			y = (size.Height - h) / 2
		}
		x := float32(0)
		if w < size.Width {
			x = (size.Width - w) / 2
		}
		o.Move(fyne.NewPos(x, y))
		o.Resize(fyne.NewSize(w, h))
	}
}

// MinSize ...
func (s stageLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objs {
		if o == nil || !o.Visible() {
			continue
		}
		h = max32(h, o.MinSize().Height)
	}
	return fyne.NewSize(s.w, h)
}

func stage(width float32, obj fyne.CanvasObject) fyne.CanvasObject {
	return boostScroll(container.NewVScroll(container.New(&stageLayout{w: width}, obj)))
}
