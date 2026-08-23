package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
		{pageLogin, "Login", "", theme.LoginIcon(), func(*App) bool { return !logged }},
		{pageServers, "Servers", "", theme.StorageIcon(), func(*App) bool { return logged }},
		{pageTunnels, "Tunnels", "", theme.ListIcon(), func(ap *App) bool { return logged && ap.advanced }},
		{pageDevices, "Devices", "", theme.ComputerIcon(), func(*App) bool { return logged }},
		{pageDNS, "Settings", "DNS", theme.SearchIcon(), func(ap *App) bool { return ap.advanced }},
		{pageDNSStats, "Stats", "DNS", theme.DocumentIcon(), func(ap *App) bool { return ap.advanced }},
		{pageSettings, "Settings", "App", theme.SettingsIcon(), func(*App) bool { return true }},
		{pageAccounts, "Accounts", "App", theme.AccountIcon(), func(*App) bool { return !logged }},
		{pageLogs, "Logs", "App", theme.FileTextIcon(), func(*App) bool { return true }},
		{pageSupport, "Support", "App", theme.HelpIcon(), func(*App) bool { return true }},
	}
}

type sidebar struct {
	widget.BaseWidget
	a        *App
	expanded bool
}

func newSidebar(a *App) *sidebar {
	s := &sidebar{a: a, expanded: true}
	s.ExtendBaseWidget(s)
	return s
}

func (s *sidebar) MinSize() fyne.Size {
	return fyne.NewSize(railExpanded, 200)
}

func (s *sidebar) CreateRenderer() fyne.WidgetRenderer {
	r := &sidebarRenderer{s: s}
	r.bg = canvas.NewRectangle(pal().Base100)
	r.edge = canvas.NewRectangle(pal().Base300)
	r.body = container.NewVBox()
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
}

func (r *sidebarRenderer) rebuild() {
	a := r.s.a
	exp := r.s.expanded
	p := pal()
	items := []fyne.CanvasObject{vspace(12)}
	lastGroup := "\x00"
	for _, it := range a.navItems() {
		if !it.show(a) {
			continue
		}
		if exp && it.group != "" && it.group != lastGroup {
			items = append(items, vspace(8),
				container.NewPadded(text(it.group, 10, p.Faint, true)))
			lastGroup = it.group
		}
		items = append(items, r.navRow(it, exp, a.current == it.id))
	}
	items = append(items, layout.NewSpacer())
	if a.loggedIn() {
		email := a.user.Email
		if email == "" {
			email = "anonymous"
		}
		avatar := canvas.NewCircle(p.Base300)
		ico := widget.NewIcon(theme.AccountIcon())
		av := container.NewStack(spacer(28, 28), avatar, container.NewCenter(ico))
		var row fyne.CanvasObject
		if exp {
			lab := text(email, 12, p.Content, true)
			row = container.NewHBox(av, lab)
		} else {
			row = container.NewCenter(av)
		}
		active := a.current == pageAccount
		items = append(items, r.wrapRow(row, active, func() { a.show(pageAccount) }))
		items = append(items, vspace(8))
	}
	r.body.Objects = items
}

func (r *sidebarRenderer) navRow(it navItem, exp, active bool) fyne.CanvasObject {
	ico := widget.NewIcon(it.icon)
	var inner fyne.CanvasObject
	if exp {
		lab := text(it.label, 13, pal().Content, false)
		if !active {
			lab.Color = pal().Muted
		}
		inner = container.NewPadded(container.NewHBox(hspace(6), ico, lab))
	} else {
		inner = container.NewCenter(container.NewPadded(ico))
	}
	return r.wrapRow(inner, active, func() {
		a := r.s.a
		if it.id == pageServers {
			a.fetchServers(false)
		}
		if it.id == pageDevices {
			a.fetchDevices()
		}
		if it.id == pageTunnels {
			a.fetchServers(false)
		}
		a.show(it.id)
	})
}

func (r *sidebarRenderer) wrapRow(inner fyne.CanvasObject, active bool, tap func()) fyne.CanvasObject {
	p := pal()
	bg := canvas.NewRectangle(color.Transparent)
	if active {
		bg.FillColor = p.Hover
	}
	bar := canvas.NewRectangle(color.Transparent)
	bar.SetMinSize(fyne.NewSize(2, 1))
	if active {
		bar.FillColor = p.Primary
	}
	return newTap(container.NewStack(bg, container.NewBorder(nil, nil, nil, bar, inner)), tap)
}

func (a *App) refreshNav() {
	if a.side != nil {
		a.side.Refresh()
	}
}
