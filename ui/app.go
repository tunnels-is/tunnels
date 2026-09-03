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
	pageDashboard   pageID = "dashboard"
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
	pageBox   *fyne.Container
	side      *sidebar
	toastBox  *fyne.Container
	busyBox   *fyne.Container
	busyN     int
	toastKind string
	toastMsg  string

	current  pageID
	editTag  string
	peersTag string
	zoom     float32

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

	probeResults []client.ServerProbe
	probeAt      time.Time
	probing      bool
	probedOnce   bool
	bwRange      int

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
	logList    *widget.List
	logView    []string

	liveByServer    map[string]*client.TUN
	liveByTag       map[string]*client.TUN
	deviceConnIPs   map[string]struct{}
	deviceLocalIDs  map[string]struct{}
	deviceLocalPubs map[string]struct{}

	bwLive []*bandwidthPanel

	logHeights       map[widget.ListItemID]float32
	logHeightQueued  bool
	settingLogHeight bool

	serversLoaded   bool
	devicesLoaded   bool
	serversFetching bool
	devicesFetching bool
	lastLogPaint    time.Time
	reloading       bool
}

// Run starts the desktop window. The client service must already be initialized.
func Run(icon []byte) {
	defer recoverRun("Run")
	hardenFyne()
	a := newApp(icon)
	a.bootstrap()
	a.win.ShowAndRun()
}

func newApp(icon []byte) *App {
	fy := app.NewWithID(appID)
	name := fy.Preferences().StringWithFallback("ui-theme", defaultThemeName)
	setLiveTheme(name)
	var iconRes fyne.Resource
	if len(icon) > 0 {
		iconRes = fyne.NewStaticResource("appicon.png", icon)
		fy.SetIcon(iconRes)
	}

	// Apply the saved zoom before anything is built so the first layout is
	// already at the right scale.
	zoom := loadZoom(fy)

	w := fy.NewWindow(appName)
	// GLFW is initialized by NewWindow; the native window is created on Show.
	setLinuxWindowIdentity()
	registerLinuxDesktop(icon)
	if iconRes != nil {
		w.SetIcon(iconRes)
	}
	w.Resize(fyne.NewSize(1200, 780))
	w.SetMaster()

	a := &App{
		fyneApp:     fy,
		win:         w,
		loginMode:   1,
		dnsStatsTab: "blocked",
		accountTab:  "account",
		logs:        client.SnapshotLogs(),
		advanced:    fy.Preferences().BoolWithFallback("advanced", false),
		zoom:        zoom,
	}

	a.content = container.NewStack()
	a.side = newSidebar(a)
	a.toastBox = container.NewStack()
	a.busyBox = container.NewStack()
	a.pageBox = container.New(railLayout{}, a.side, a.content)
	shell := container.New(&shellLayout{win: w}, a.pageBox, a.toastBox, a.busyBox)
	w.SetContent(shell)
	// Fyne pads window content by default, which draws a border of window
	// background around the whole app. The shell manages its own gutters.
	w.SetPadded(false)

	a.keepDisplayAwake()
	a.installQuitMenu()
	a.surviveWindowClose()

	a.registerZoomShortcuts()
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
		a.show(pageDashboard)
		a.startupFetch()
	case len(users) == 1:
		a.setUser(users[0])
		a.show(pageDashboard)
		a.startupFetch()
	case len(users) > 1:
		a.show(pageAccounts)
	default:
		a.show(pageLogin)
	}
}

// startupFetch loads the server list in the background so the dashboard can
// probe without waiting for input. bootstrap runs before the window is shown,
// so nothing here may block: fetchServers hands off to a goroutine and the
// probe is chained from its completion.
func (a *App) startupFetch() {
	a.fetchServers(false)
}

func (a *App) loggedIn() bool {
	return a.user != nil && (a.user.Email != "" || a.user.ID != "")
}

func (a *App) show(id pageID) {
	defer a.recoverUI("show")
	prev := a.current
	a.teardownPage()
	a.current = id
	// Login is a focused, full-window flow: the rail has nothing to offer yet.
	// Hiding a child does not re-run the parent layout, so refresh the page
	// container explicitly or it keeps the rail's gap.
	if a.side != nil {
		wasHidden := a.side.Hidden
		if id == pageLogin {
			a.side.Hide()
		} else {
			a.side.Show()
		}
		if wasHidden != a.side.Hidden && a.pageBox != nil {
			defer a.pageBox.Refresh()
		}
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

func (a *App) stopBandwidthPanels() {
	for _, p := range a.bwLive {
		if p != nil {
			p.stop()
		}
	}
	a.bwLive = nil
}

func (a *App) teardownPage() {
	a.stopBandwidthPanels()
	a.dropLiveLists()
}

func (a *App) dropLiveLists() {
	a.serverList = nil
	a.tunnelList = nil
	a.deviceList = nil
	a.logList = nil
	a.logHeights = nil
	a.logHeightQueued = false
	a.settingLogHeight = false
	a.dnsStats = nil
}

func (a *App) refreshLivePage() bool {
	switch a.current {
	case pageServers:
		if a.serverList == nil {
			return false
		}
		a.recomputeServerView()
		a.serverList.Refresh()
		return true
	case pageTunnels:
		if a.tunnelList == nil {
			return false
		}
		a.recomputeTunnelView()
		a.tunnelList.Refresh()
		return true
	case pageDevices:
		if a.deviceList == nil {
			return false
		}
		a.recomputeDeviceView()
		a.deviceList.Refresh()
		return true
	case pageLogs:
		if a.logList == nil {
			return false
		}
		a.paintLogs()
		return true
	default:
		return false
	}
}

func (a *App) buildPage(id pageID) fyne.CanvasObject {
	switch id {
	case pageDashboard:
		return a.dashboardPage()
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
