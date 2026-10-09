package ui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

// dashNameSize is the server name on the dashboard. It is larger than the
// page title so the name reads as the screen, not a label inside a card.
func dashNameSize() float32 { return snapDIP(z(34)) }

// dashLatencySize is the round-trip figure above that name.
func dashLatencySize() float32 { return snapDIP(z(22)) }

// ---------------------------------------------------------------- page

func (a *App) dashboardPage() fyne.CanvasObject {
	probe := primaryBtn("Find closest", func() { a.forceProbe() }).withIcon(theme.ViewRefreshIcon())

	// The window selector only means something when a chart is on screen.
	actions := fyne.CanvasObject(probe)
	if len(a.myTunnels()) > 0 {
		actions = hstackFlex(sp2, 0, a.bwRangePicker(), probe)
	}

	// Building a page must not start work. The first probe is chained off the
	// server fetch instead, so it also fires when the list is still in flight.
	a.maybeAutoProbe()

	return pageShell("Dashboard", "", actions, stage(z(520), a.dashBody()))
}

// dashBody is the centred column: the server you would connect to, or the
// one you are on, with a chart under each live tunnel.
func (a *App) dashBody() fyne.CanvasObject {
	if mine := a.myTunnels(); len(mine) > 0 {
		parts := []fyne.CanvasObject{a.dashLiveHero(mine[0])}
		for _, t := range mine {
			parts = append(parts, a.bandwidthBlock(t, len(mine) > 1))
		}
		return vstack(sp8, parts...)
	}
	return a.dashClosest()
}

// myTunnels is the signed-in user's active tunnels.
func (a *App) myTunnels() []*client.TUN {
	var mine []*client.TUN
	for _, t := range a.active {
		if t != nil && t.CR != nil && a.user != nil && t.CR.UserID == a.user.ID {
			mine = append(mine, t)
		}
	}
	return mine
}

// dashClosest is the centred closest-server prompt. Nothing here sits on a card.
func (a *App) dashClosest() fyne.CanvasObject {
	if a.probing && a.probeResults == nil {
		return vstack(sp3,
			dashCenter("Finding the closest server", dashNameSize(), pal().Content, true, false),
			dashCenter("Measuring round-trip time.", fsLarge, pal().Muted, false, false),
		)
	}
	best, ok := a.bestProbe()
	if !ok {
		msg := "No server answered."
		hint := "Run a probe to measure round-trip time."
		if len(a.probeResults) == 0 {
			msg = "Find the closest server"
			hint = "A probe measures round-trip time to every server."
		}
		probe := primaryBtn("Probe", func() { a.forceProbe() }).withIcon(theme.ViewRefreshIcon())
		return vstack(sp4,
			dashCenter(msg, dashNameSize(), pal().Content, true, false),
			dashCenter(hint, fsLarge, pal().Muted, false, false),
			container.NewCenter(probe),
		)
	}

	connect := successBtn("Connect", func() {
		srv, ok := a.closestServer()
		if !ok {
			a.fail("That server is no longer in your list")
			return
		}
		a.confirm("Connect", "Connect to "+srv.Tag+"?", func() { a.connectToServer(srv) })
	})

	lines := []fyne.CanvasObject{
		dashCenter("Closest server", fsLarge, pal().Muted, false, false),
		dashCenter(fmt.Sprintf("%d ms", best.LatencyMS()), dashLatencySize(), pal().Success, true, true),
		dashCenter(best.Tag, dashNameSize(), pal().Content, true, false),
	}
	if c := countryName(best.Country); c != "" {
		lines = append(lines, dashCenter(c, fsLarge, pal().Muted, false, false))
	}
	if best.IP != "" {
		lines = append(lines, dashCenter(best.IP, fsLarge, pal().Content, false, true))
	}
	lines = append(lines, vspace(sp2), container.NewCenter(connect))
	return vstack(sp2, lines...)
}

// dashLiveHero names the server a live tunnel is using.
func (a *App) dashLiveHero(t *client.TUN) fyne.CanvasObject {
	title := "Connected"
	where := ""
	addr := ""
	tunnel := ""
	if t != nil && t.CR != nil {
		tunnel = t.CR.Tag
		if s := a.serverByID(t.CR.ServerID); s != nil {
			if s.Tag != "" {
				title = s.Tag
			}
			where = countryName(s.Country)
			if s.IP != "" {
				addr = s.IP
				if s.Port != "" {
					addr += ":" + s.Port
				}
			}
		}
	}
	lines := []fyne.CanvasObject{
		dashCenter(title, dashNameSize(), pal().Content, true, false),
	}
	if where != "" {
		lines = append(lines, dashCenter(where, fsLarge, pal().Muted, false, false))
	}
	if addr != "" {
		lines = append(lines, dashCenter(addr, fsLarge, pal().Content, false, true))
	}
	if tunnel != "" && tunnel != title {
		lines = append(lines, dashCenter(tunnel, fsLarge, pal().Muted, false, false))
	}
	lines = append(lines, dashCenter("in use", fsLarge, pal().Success, true, false))
	return vstack(sp2, lines...)
}

// dashCenter is one centred line. mono is for addresses and timings.
func dashCenter(s string, size float32, c color.Color, bold, mono bool) fyne.CanvasObject {
	var label *canvas.Text
	if mono {
		label = monoText(s, size, c)
		label.TextStyle.Bold = bold
	} else {
		label = text(s, size, c, bold)
	}
	return container.NewCenter(label)
}

// ---------------------------------------------------------------- actions

// maybeAutoProbe runs the first probe on its own, matching the web UI's rules:
// only when signed in, only once, and not while a tunnel is already up — in
// that case the server in use is the answer, so pinging the fleet is wasted
// work. Safe to call repeatedly; every guard is cheap.
func (a *App) maybeAutoProbe() {
	switch a.autoProbeState() {
	case autoProbeRun:
		a.runProbe()
	case autoProbeNeedServers:
		// fetchServers calls back here once the list lands.
		a.fetchServers(false)
	}
}

type autoProbeDecision int

const (
	autoProbeSkip autoProbeDecision = iota
	autoProbeNeedServers
	autoProbeRun
)

// autoProbeState is the decision on its own so the guards can be tested without
// sending a single packet.
func (a *App) autoProbeState() autoProbeDecision {
	if a.probedOnce || a.probing {
		return autoProbeSkip
	}
	if !a.loggedIn() || a.user.ControlServer == nil {
		return autoProbeSkip
	}
	if len(a.myTunnels()) > 0 {
		return autoProbeSkip
	}
	if len(a.servers) == 0 {
		return autoProbeNeedServers
	}
	return autoProbeRun
}

// forceProbe is the button: re-measure even if a probe already ran, and
// re-fetch the server list first if it is empty, as the web UI does.
func (a *App) forceProbe() {
	if len(a.servers) == 0 {
		a.fetchServers(true)
		return
	}
	a.probedOnce = false
	a.runProbe()
}

// runProbe measures every server in the background.
func (a *App) runProbe() {
	if a.probing {
		return
	}
	a.probing = true
	a.probedOnce = true
	servers := append([]types.Server(nil), a.servers...)

	a.runTask("Probing servers", func() error {
		res := client.ProbeServers(servers)
		a.uiDo(func() {
			a.probeResults = res
			a.probeAt = time.Now()
		})
		return nil
	}, func(error) {
		a.probing = false
		if a.current == pageDashboard {
			a.reloadCurrent()
		}
	})
}

func (a *App) bestProbe() (client.ServerProbe, bool) {
	for _, p := range a.probeResults {
		if p.OK {
			return p, true
		}
	}
	return client.ServerProbe{}, false
}

// closestServer is the server the dashboard Connect button would use:
// the fastest probe result that is still in the server list.
func (a *App) closestServer() (types.Server, bool) {
	best, ok := a.bestProbe()
	if !ok {
		return types.Server{}, false
	}
	s := a.serverByID(best.ServerID)
	if s == nil {
		return types.Server{}, false
	}
	return *s, true
}
