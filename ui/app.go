package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

type pageID string

const (
	pageLogin       pageID = "login"
	pageAccounts    pageID = "accounts"
	pageServers     pageID = "servers"
	pageTunnels     pageID = "tunnels"
	pageTunnelEdit  pageID = "tunnel-edit"
	pageTunnelPeers pageID = "tunnel-peers"
	pageDevices     pageID = "devices"
	pageAccount     pageID = "account"
	pageTwoFactor   pageID = "twofactor"
	pageDNS         pageID = "dns"
	pageDNSStats    pageID = "dnsstats"
	pageLogs        pageID = "logs"
	pageSettings    pageID = "settings"
	pageSupport     pageID = "support"
	pageConnections pageID = "connections"
)

// App is the Fyne desktop UI. It talks to the client package in-process.
type App struct {
	fyneApp   fyne.App
	win       fyne.Window
	content   *fyne.Container
	side      *sidebar
	toastBox  *fyne.Container
	toastKind string
	toastMsg  string
	status    *widget.Label

	current  pageID
	editTag  string
	peersTag string

	user         *client.User
	users        []*client.User
	config       *client.Config
	state        *client.StateResponse
	tunnels      []*client.TunnelMETA
	active       []*client.TUN
	servers      []types.Server
	devices      []types.Device
	localDevices []client.LocalDeviceInfo
	dnsStats     map[string]*client.DNSStats
	logs         []string
	advanced     bool

	filterServers  string
	filterTunnels  string
	filterDevices  string
	filterLogs     string
	logTag         string
	dnsStatsTab    string
	dnsStatsFilter string
	accountTab     string

	loginMode     int
	loginRemember bool
	loginToken    bool
	loginServerID string

	serverList *widget.List
	serverView []types.Server
	tunnelList *widget.List
	tunnelView []*client.TunnelMETA
	deviceList *widget.List
	deviceView []types.Device
	logBox     *widget.Entry
	logView    []string

	serversLoaded   bool
	devicesLoaded   bool
	serversFetching bool
	devicesFetching bool
	lastLogPaint    time.Time
	reloading       bool
}

// Run starts the desktop window. The client service must already be initialized.
func Run(icon []byte) {
	a := newApp(icon)
	a.bootstrap()
	a.win.ShowAndRun()
}

func newApp(icon []byte) *App {
	fy := app.NewWithID("is.tunnels.desktop")
	name := fy.Preferences().StringWithFallback("ui-theme", defaultThemeName)
	setLiveTheme(name)
	if len(icon) > 0 {
		fy.SetIcon(fyne.NewStaticResource("appicon.png", icon))
	}

	w := fy.NewWindow("Tunnels")
	w.Resize(fyne.NewSize(1280, 800))
	w.SetMaster()

	a := &App{
		fyneApp:     fy,
		win:         w,
		status:      widget.NewLabel(""),
		loginMode:   1,
		dnsStatsTab: "blocked",
		accountTab:  "account",
		logs:        client.SnapshotLogs(),
		advanced:    fy.Preferences().BoolWithFallback("advanced", false),
	}
	a.status.Wrapping = fyne.TextWrapOff

	a.content = container.NewStack()
	a.side = newSidebar(a)
	a.toastBox = container.NewStack()
	page := container.NewBorder(nil, nil, a.side, nil, a.content)
	shell := container.New(shellLayout{}, page, a.toastBox)
	w.SetContent(shell)

	w.SetCloseIntercept(func() {
		if client.CancelFunc != nil {
			client.CancelFunc()
		}
		client.ResetEverything()
		a.fyneApp.Quit()
	})

	a.startLogPump()
	return a
}

func (a *App) bootstrap() {
	a.refreshState()
	users, err := client.GetUsers()
	if err != nil {
		a.fail("Unable to load accounts: " + err.Error())
	}
	a.users = users

	wantID := a.fyneApp.Preferences().String("activeUserID")
	var match *client.User
	for _, u := range users {
		if wantID != "" && u.ID == wantID {
			match = u
			break
		}
	}
	switch {
	case match != nil:
		a.setUser(match)
		a.show(pageServers)
	case len(users) == 1:
		a.setUser(users[0])
		a.show(pageServers)
	case len(users) > 1:
		a.show(pageAccounts)
	default:
		a.show(pageLogin)
	}
}

func (a *App) loggedIn() bool {
	return a.user != nil && (a.user.Email != "" || a.user.ID != "")
}

func (a *App) show(id pageID) {
	defer a.recoverUI("show")
	prev := a.current
	a.current = id
	if prev != id {
		a.dropLiveLists()
	}
	if a.content != nil {
		for _, o := range a.content.Objects {
			if o != nil {
				o.Hide()
			}
		}
		a.content.RemoveAll()
		a.content.Add(a.buildPage(id))
	}
	if prev != id {
		a.refreshNav()
	}
}

func (a *App) rebuild() {
	defer a.recoverUI("rebuild")
	a.reloadCurrent()
}

func (a *App) dropLiveLists() {
	a.serverList = nil
	a.tunnelList = nil
	a.deviceList = nil
	a.logBox = nil
}

func (a *App) buildPage(id pageID) fyne.CanvasObject {
	switch id {
	case pageLogin:
		return a.loginPage()
	case pageAccounts:
		return a.accountsPage()
	case pageServers:
		return a.serversPage()
	case pageTunnels:
		return a.tunnelsPage()
	case pageTunnelEdit:
		return a.tunnelEditPage()
	case pageTunnelPeers:
		return a.tunnelPeersPage()
	case pageDevices:
		return a.devicesPage()
	case pageAccount:
		return a.accountPage()
	case pageTwoFactor:
		return a.twoFactorPage()
	case pageDNS:
		return a.dnsPage()
	case pageDNSStats:
		return a.dnsStatsPage()
	case pageLogs:
		return a.logsPage()
	case pageSettings:
		return a.settingsPage()
	case pageSupport:
		return a.supportPage()
	case pageConnections:
		return a.connectionsPage()
	default:
		return widget.NewLabel("Unknown page")
	}
}

func (a *App) setAdvanced(v bool) {
	a.advanced = v
	a.fyneApp.Preferences().SetBool("advanced", v)
	a.refreshNav()
	a.rebuild()
}
