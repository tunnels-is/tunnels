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

// ---------------------------------------------------------------- sizing

type fixedLayout struct{ w, h float32 }

func (f fixedLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

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

// ---------------------------------------------------------------- fields

// Entries are left unwrapped: the theme supplies input background, border and
// radius, so a bare Entry already matches the rest of the kit.
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

// field stacks a small label above its control.
func field(label string, obj fyne.CanvasObject) fyne.CanvasObject {
	return vstack(sp1+2, fieldLabel(label), obj)
}

// fieldWith adds a one-line explanation under the control.
func fieldWith(label, note string, obj fyne.CanvasObject) fyne.CanvasObject {
	return vstack(sp1+2, fieldLabel(label), obj, hint(note))
}

// formRows lays out fields in a single readable column.
func formRows(objs ...fyne.CanvasObject) fyne.CanvasObject {
	return capWidth(formWidth, vstack(sp3, objs...))
}

// formPair puts two fields side by side for short values like IP and port.
func formPair(a, b fyne.CanvasObject) fyne.CanvasObject {
	return container.New(equalColsLayout{gap: sp3}, a, b)
}

type equalColsLayout struct{ gap float32 }

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

// searchField is a fixed-width filter box. Fixed width matters: in a header
// row a growing entry pushes the action buttons off the edge.
func searchField(placeholder, value string, onChange, onSubmit func(string)) (*widget.Entry, fyne.CanvasObject) {
	e := widget.NewEntry()
	// ActionItem has to be assigned before anything that can build the
	// renderer (SetText does), otherwise the icon is never added to it.
	icon := canvas.NewImageFromResource(theme.NewColoredResource(theme.SearchIcon(), colFaint))
	icon.FillMode = canvas.ImageFillContain
	e.ActionItem = fixedWidth(z(18), icon)
	e.SetPlaceHolder(placeholder)
	e.SetText(value)
	e.OnChanged = onChange
	e.OnSubmitted = onSubmit
	return e, fixedWidth(searchWidth, e)
}

// ---------------------------------------------------------------- row editor

// weightedColsLayout splits its width between children by weight, so the
// columns of a repeatable row line up with the header above them.
type weightedColsLayout struct {
	weights []float32
	gap     float32
}

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

// fieldCol describes one column of a rowEditor.
type fieldCol struct {
	label       string
	placeholder string
	weight      float32
}

// rowEditor edits a repeatable list of records — one row of entries per record,
// with add and remove. It replaces the multiline "one per line, space
// separated" text areas, which hid the record structure and silently dropped
// any field the parser did not know about.
//
// Rows own their entry widgets, so adding or removing a row leaves text already
// typed in the other rows untouched.
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

	// Column headings, only worth showing when a row has several fields.
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

// values returns the trimmed rows, skipping any row left entirely blank.
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

// column returns column i of every non-blank row.
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

// ---------------------------------------------------------------- switch

// kSwitch replaces widget.Check for on/off settings. Stock checkboxes are the
// one control that never matched the rest of the kit.
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

func (s *kSwitch) MouseIn(*desktop.MouseEvent)    { s.hovered = true; s.Refresh() }
func (s *kSwitch) MouseOut()                      { s.hovered = false; s.Refresh() }
func (s *kSwitch) MouseMoved(*desktop.MouseEvent) {}

func (s *kSwitch) Cursor() desktop.Cursor {
	if s.disabled {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

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
}

func (r *switchRenderer) Destroy()                     {}
func (r *switchRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.track, r.knob} }
func (r *switchRenderer) MinSize() fyne.Size           { return fyne.NewSize(swWidth, swHeight) }

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

// settingRow is title + explanation on the left, control hard right. Used for
// every toggle in Settings and DNS.
func settingRow(title, desc string, control fyne.CanvasObject) fyne.CanvasObject {
	left := []fyne.CanvasObject{body(title)}
	if desc != "" {
		left = append(left, hint(desc))
	}
	row := splitRow(vstack(2, left...), control)
	return insetEach(sp3, 0, sp3, 0, row)
}

// settingList interleaves hairlines between setting rows.
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

// toggleRow is the common case: a switch bound to a config key.
func toggleRow(title, desc string, on bool, onChange func(bool)) fyne.CanvasObject {
	return settingRow(title, desc, newSwitch(on, onChange))
}

// ---------------------------------------------------------------- segmented

type segItem struct {
	key   string
	label string
}

// segControl is a pill tab strip. It replaces both stock AppTabs and the rows
// of highlighted buttons that used to serve as tabs.
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
	// Labels never change, so measure once instead of on every mouse move.
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

func (s *segControl) Tapped(e *fyne.PointEvent) {
	if i := s.index(e.Position.X); i >= 0 {
		s.active = s.items[i].key
		s.Refresh()
		if s.onSelect != nil {
			s.onSelect(s.items[i].key)
		}
	}
}

func (s *segControl) MouseIn(e *desktop.MouseEvent) { s.MouseMoved(e) }
func (s *segControl) MouseOut()                     { s.hover = -1; s.Refresh() }

func (s *segControl) MouseMoved(e *desktop.MouseEvent) {
	if i := s.index(e.Position.X); i != s.hover {
		s.hover = i
		s.Refresh()
	}
}

func (s *segControl) Cursor() desktop.Cursor { return desktop.PointerCursor }

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
}

func (r *segRenderer) Destroy() {}

func (r *segRenderer) Objects() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, len(r.pills)*2+1)
	out = append(out, r.bg)
	for i := range r.pills {
		out = append(out, r.pills[i])
	}
	for i := range r.labels {
		out = append(out, r.labels[i])
	}
	return out
}

func (r *segRenderer) MinSize() fyne.Size {
	var w float32
	for i := range r.s.items {
		w += r.s.itemWidth(i)
	}
	return fyne.NewSize(w+segInset*2, segHeight)
}

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

// ---------------------------------------------------------------- link row

// linkRow is a name plus destination that opens a URL, used on the support page.
func linkRow(a *App, name, dest, href string) fyne.CanvasObject {
	p := pal()
	title := text(name, fsBody, p.Content, false)
	sub := text(dest, fsSmall, p.Faint, false)
	go2 := newIconBtn(theme.NavigateNextIcon(), kGhost, func() { a.openURL(href) })
	row := splitRow(vstack(1, title, sub), go2)
	return newTap(insetEach(sp2, 0, sp2, 0, row), func() { a.openURL(href) })
}

// ---------------------------------------------------------------- page shell

// pageShell is the frame every page uses: title block, hairline, then body.
// One implementation means every page has the same gutters and header rhythm.
// Page titles are always one line, so canvas.Text is used for exact placement:
// the title lands on the same x as card edges and list row titles.
func pageShell(title, subtitle string, actions fyne.CanvasObject, content fyne.CanvasObject) fyne.CanvasObject {
	titleBlock := []fyne.CanvasObject{text(title, fsTitle, pal().Content, true)}
	if subtitle != "" {
		titleBlock = append(titleBlock, text(subtitle, fsSmall, pal().Muted, false))
	}
	head := splitRow(vstack(sp1+1, titleBlock...), actions)
	top := vstack(0, insetEach(sp5, gutter, sp5, gutter, head), strongDivider())
	return container.NewBorder(top, nil, nil, nil, content)
}

// listBody frames a widget.List so row content lines up with the page header
// and the action column lines up with the header actions.
func listBody(l *widget.List) fyne.CanvasObject {
	return insetEach(sp2, sp2, sp3, sp2, boostList(l))
}

// scrollBody is the standard page body: cards flowed into as many columns as
// the window allows, rather than one full-width stack.
func scrollBody(objs ...fyne.CanvasObject) fyne.CanvasObject {
	flow := container.New(&cardFlowLayout{minCol: z(430), maxCol: 3, gap: sp4}, objs...)
	return scrollBodyOf(flow)
}

// scrollBodyOf wraps a column the caller already holds, for pages that need to
// refresh it directly (row editors changing height, for instance) or that read
// better as a single column.
func scrollBodyOf(col fyne.CanvasObject) fyne.CanvasObject {
	return boostScroll(container.NewVScroll(insetEach(sp5, gutter, sp8, gutter, col)))
}

// centredBody centres a fixed-width column, for login and other focused flows.
func centredBody(width float32, objs ...fyne.CanvasObject) fyne.CanvasObject {
	col := vstack(sp4, objs...)
	return boostScroll(container.NewVScroll(
		container.NewCenter(container.New(fixedLayout{w: width}, col))))
}
