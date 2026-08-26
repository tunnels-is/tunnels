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
	"github.com/tunnels-is/tunnels/version"
)

type navItem struct {
	id    pageID
	label string
	group string
	icon  fyne.Resource
	show  func(*App) bool
}

func (a *App) navItems() []navItem {
	logged := a.loggedIn()
	return []navItem{
		{pageLogin, "Sign in", "", theme.LoginIcon(), func(*App) bool { return !logged }},
		{pageDashboard, "Dashboard", "", theme.HomeIcon(), func(*App) bool { return logged }},
		{pageServers, "Servers", "", theme.StorageIcon(), func(*App) bool { return logged }},
		{pageTunnels, "Tunnels", "", theme.ListIcon(), func(ap *App) bool { return logged && ap.advanced }},
		{pageDevices, "Devices", "", theme.ComputerIcon(), func(*App) bool { return logged }},
		{pageAccounts, "Accounts", "", theme.AccountIcon(), func(*App) bool { return !logged }},
		{pageDNS, "Resolver", "DNS", theme.SearchIcon(), func(ap *App) bool { return ap.advanced }},
		{pageDNSStats, "Statistics", "DNS", theme.DocumentIcon(), func(ap *App) bool { return ap.advanced }},
		{pageSettings, "Settings", "App", theme.SettingsIcon(), func(*App) bool { return true }},
		{pageLogs, "Logs", "App", theme.FileTextIcon(), func(*App) bool { return true }},
		{pageSupport, "Support", "App", theme.HelpIcon(), func(*App) bool { return true }},
	}
}

// ---------------------------------------------------------------- nav row

type navRow struct {
	widget.BaseWidget
	icon    fyne.Resource
	label   string
	active  bool
	hovered bool
	tap     func()
}

func newNavRow(icon fyne.Resource, label string, active bool, tap func()) *navRow {
	n := &navRow{icon: icon, label: label, active: active, tap: tap}
	n.ExtendBaseWidget(n)
	return n
}

func (n *navRow) Tapped(*fyne.PointEvent) {
	if n.tap != nil {
		n.tap()
	}
}

func (n *navRow) MouseIn(*desktop.MouseEvent)    { n.hovered = true; n.Refresh() }
func (n *navRow) MouseOut()                      { n.hovered = false; n.Refresh() }
func (n *navRow) MouseMoved(*desktop.MouseEvent) {}
func (n *navRow) Cursor() desktop.Cursor         { return desktop.PointerCursor }

func (n *navRow) CreateRenderer() fyne.WidgetRenderer {
	bg := surface(radSm, color.Transparent, nil)
	rail := canvas.NewRectangle(color.Transparent)
	rail.CornerRadius = radFull
	ico := canvas.NewImageFromResource(n.icon)
	ico.FillMode = canvas.ImageFillContain
	lab := text(n.label, fsBody, pal().Muted, false)
	r := &navRowRenderer{n: n, bg: bg, rail: rail, ico: ico, lab: lab}
	r.apply()
	return r
}

type navRowRenderer struct {
	n    *navRow
	bg   *canvas.Rectangle
	rail *canvas.Rectangle
	ico  *canvas.Image
	lab  *canvas.Text

	icoActive fyne.Resource
	icoHover  fyne.Resource
	icoIdle   fyne.Resource
	objs      []fyne.CanvasObject
}

func (r *navRowRenderer) Destroy() {}

func (r *navRowRenderer) Objects() []fyne.CanvasObject {
	if r.objs == nil {
		r.objs = []fyne.CanvasObject{r.bg, r.rail, r.ico, r.lab}
	}
	return r.objs
}

func (r *navRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(railWidth-sp3*2, navRowH)
}

func (r *navRowRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.rail.Resize(fyne.NewSize(z(3), size.Height-z(14)))
	r.rail.Move(fyne.NewPos(-sp2, z(7)))
	r.ico.Resize(fyne.NewSize(iconSize, iconSize))
	r.ico.Move(fyne.NewPos(sp2+z(2), (size.Height-iconSize)/2))
	lms := r.lab.MinSize()
	r.lab.Move(fyne.NewPos(sp2+z(2)+iconSize+sp3, (size.Height-lms.Height)/2))
}

func (r *navRowRenderer) Refresh() {
	old := r.ico.Resource
	r.apply()
	r.bg.Refresh()
	r.rail.Refresh()
	if r.ico.Resource != old {
		r.ico.Refresh()
	}
	r.lab.Refresh()
	canvasRefresh(r.n)
}

func (r *navRowRenderer) apply() {
	p := pal()
	r.lab.Text = r.n.label
	if r.icoActive == nil {
		r.icoActive = theme.NewColoredResource(r.n.icon, theme.ColorNamePrimary)
		r.icoHover = theme.NewColoredResource(r.n.icon, colContent)
		r.icoIdle = theme.NewColoredResource(r.n.icon, colMuted)
	}
	switch {
	case r.n.active:
		r.bg.FillColor = p.PrimarySoft
		r.rail.FillColor = p.Primary
		r.lab.Color = p.Content
		r.lab.TextStyle = fyne.TextStyle{Bold: true}
		r.ico.Resource = r.icoActive
	case r.n.hovered:
		r.bg.FillColor = p.Hover
		r.rail.FillColor = color.Transparent
		r.lab.Color = p.Content
		r.lab.TextStyle = fyne.TextStyle{}
		r.ico.Resource = r.icoHover
	default:
		r.bg.FillColor = color.Transparent
		r.rail.FillColor = color.Transparent
		r.lab.Color = p.Muted
		r.lab.TextStyle = fyne.TextStyle{}
		r.ico.Resource = r.icoIdle
	}
}

// ---------------------------------------------------------------- sidebar

type sidebar struct {
	widget.BaseWidget
	a *App
}

func newSidebar(a *App) *sidebar {
	s := &sidebar{a: a}
	s.ExtendBaseWidget(s)
	return s
}

func (s *sidebar) MinSize() fyne.Size {
	return fyne.NewSize(railWidth, 320)
}

func (s *sidebar) CreateRenderer() fyne.WidgetRenderer {
	r := &sidebarRenderer{s: s}
	r.bg = canvas.NewRectangle(pal().Base100)
	r.edge = canvas.NewRectangle(pal().Base300)
	r.body = container.NewStack()
	r.objects = []fyne.CanvasObject{r.bg, r.edge, r.body}
	r.rebuild()
	return r
}

type sidebarRenderer struct {
	s       *sidebar
	bg      *canvas.Rectangle
	edge    *canvas.Rectangle
	body    *fyne.Container
	objects []fyne.CanvasObject
}

func (r *sidebarRenderer) Destroy() {}

func (r *sidebarRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.edge.Resize(fyne.NewSize(1, size.Height))
	r.edge.Move(fyne.NewPos(size.Width-1, 0))
	r.body.Resize(size)
}

func (r *sidebarRenderer) MinSize() fyne.Size { return r.s.MinSize() }

func (r *sidebarRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *sidebarRenderer) Refresh() {
	p := pal()
	r.bg.FillColor = p.Base100
	r.edge.FillColor = p.Base300
	r.rebuild()
	r.bg.Refresh()
	r.edge.Refresh()
	r.body.Refresh()
	canvasRefresh(r.s)
}

func (r *sidebarRenderer) rebuild() {
	a := r.s.a

	rows := []fyne.CanvasObject{}
	lastGroup := "\x00"
	for _, it := range a.navItems() {
		if !it.show(a) {
			continue
		}
		if it.group != lastGroup {
			gap := sp4
			if lastGroup == "\x00" {
				gap = sp1
			}
			rows = append(rows, vspace(gap))
			if it.group != "" {
				rows = append(rows, insetEach(0, 0, sp1, sp2+2, eyebrow(strings.ToUpper(it.group))))
			}
			lastGroup = it.group
		}
		item := it
		rows = append(rows, newNavRow(item.icon, item.label, a.current == item.id, func() {
			a.navigate(item.id)
		}))
	}

	nav := insetEach(sp2, sp3, sp4, sp3, vstack(2, rows...))
	// No rule under the brand: the rail already reads as its own surface, so the
	// line only chopped the logo off from the nav it belongs with.
	top := r.brand()

	var bottom fyne.CanvasObject
	if a.loggedIn() {
		bottom = vstack(0, strongDivider(), insetEach(sp2, sp3, sp2, sp3, r.userChip()))
	}

	// The nav list scrolls: with the brand pinned top and the account chip
	// pinned bottom, a short window (or a high zoom level) would otherwise
	// squeeze the Border layout until the two overlapped.
	scroll := container.NewVScroll(nav)
	scroll.Direction = container.ScrollVerticalOnly
	r.body.Objects = []fyne.CanvasObject{container.NewBorder(top, bottom, nil, nil, scroll)}
}

func (r *sidebarRenderer) brand() fyne.CanvasObject {
	ver := version.Version
	if st := r.s.a.state; st != nil && st.Version != "" {
		ver = st.Version
	}
	name := text("Tunnels", fsLarge, pal().Content, true)
	sub := text(ver, fsCaption, pal().Faint, false)
	block := hstack(sp3, brandMark(z(20)), vstack(0, name, sub))
	return insetEach(sp4, sp3, sp4, sp3+2, block)
}

func (r *sidebarRenderer) userChip() fyne.CanvasObject {
	a := r.s.a
	p := pal()
	email := a.user.Email
	if email == "" {
		email = "Anonymous"
	}
	initial := strings.ToUpper(email[:1])

	ring := canvas.NewCircle(p.PrimarySoft)
	glyph := text(initial, fsSmall, p.Primary, true)
	avatar := container.NewStack(spacer(z(26), z(26)), ring, container.NewCenter(glyph))

	name := text(email, fsSmall, p.Content, false)
	role := text("Manage account", fsCaption, p.Faint, false)
	chev := canvas.NewImageFromResource(theme.NewColoredResource(theme.NavigateNextIcon(), colFaint))
	chev.FillMode = canvas.ImageFillContain
	chevBox := container.New(fixedLayout{w: z(14), h: z(14)}, chev)

	row := container.NewBorder(nil, nil, avatar, chevBox,
		insetEach(0, 0, 0, sp2, vstack(0, name, role)))
	return newNavRowWrap(row, a.current == pageAccount, func() { a.navigate(pageAccount) })
}

// navRowWrap gives arbitrary content the same hover/active surface as a nav row.
type navRowWrap struct {
	widget.BaseWidget
	content fyne.CanvasObject
	active  bool
	hovered bool
	tap     func()
}

func newNavRowWrap(content fyne.CanvasObject, active bool, tap func()) *navRowWrap {
	w := &navRowWrap{content: content, active: active, tap: tap}
	w.ExtendBaseWidget(w)
	return w
}

func (w *navRowWrap) Tapped(*fyne.PointEvent) {
	if w.tap != nil {
		w.tap()
	}
}

func (w *navRowWrap) MouseIn(*desktop.MouseEvent)    { w.hovered = true; w.Refresh() }
func (w *navRowWrap) MouseOut()                      { w.hovered = false; w.Refresh() }
func (w *navRowWrap) MouseMoved(*desktop.MouseEvent) {}
func (w *navRowWrap) Cursor() desktop.Cursor         { return desktop.PointerCursor }

func (w *navRowWrap) CreateRenderer() fyne.WidgetRenderer {
	bg := surface(radSm, color.Transparent, nil)
	r := &navWrapRenderer{w: w, bg: bg}
	r.apply()
	return r
}

type navWrapRenderer struct {
	w    *navRowWrap
	bg   *canvas.Rectangle
	objs []fyne.CanvasObject
}

func (r *navWrapRenderer) Destroy() {}

func (r *navWrapRenderer) Objects() []fyne.CanvasObject {
	if r.objs == nil {
		r.objs = []fyne.CanvasObject{r.bg, r.w.content}
	}
	return r.objs
}

func (r *navWrapRenderer) MinSize() fyne.Size {
	ms := r.w.content.MinSize()
	return fyne.NewSize(ms.Width+sp2*2, ms.Height+sp2*2)
}

func (r *navWrapRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.w.content.Move(fyne.NewPos(sp2, sp2))
	r.w.content.Resize(fyne.NewSize(size.Width-sp2*2, size.Height-sp2*2))
}

func (r *navWrapRenderer) Refresh() {
	r.apply()
	r.bg.Refresh()
	canvasRefresh(r.w)
}

func (r *navWrapRenderer) apply() {
	p := pal()
	switch {
	case r.w.active:
		r.bg.FillColor = p.PrimarySoft
	case r.w.hovered:
		r.bg.FillColor = p.Hover
	default:
		r.bg.FillColor = color.Transparent
	}
}

// navigate loads a page and kicks off whatever data it needs.
func (a *App) navigate(id pageID) {
	switch id {
	case pageDashboard:
		a.refreshState()
		a.fetchServers(false)
	case pageServers:
		a.fetchServers(false)
	case pageTunnels:
		// Tunnels are read from the account workspace, which the CLI or a
		// previous session may have changed, so re-read rather than trusting
		// the snapshot taken at startup.
		a.refreshState()
		a.fetchServers(false)
	case pageDevices:
		a.fetchDevices()
	}
	a.show(id)
}

func (a *App) refreshNav() {
	if a.side != nil {
		a.side.Refresh()
	}
}
