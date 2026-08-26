package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

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

	cards := []fyne.CanvasObject{a.dashClosestCard()}
	// Live throughput, one card per active tunnel.
	for _, t := range a.myTunnels() {
		cards = append(cards, a.bandwidthCard(t))
	}

	sub := "Closest server and live throughput"
	return pageShell("Dashboard", sub, actions, scrollBody(cards...))
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

// dashClosestCard highlights the fastest server the probe found.
func (a *App) dashClosestCard() fyne.CanvasObject {
	// Already connected: the server in use is the relevant one, and the web UI
	// takes the same view rather than probing the fleet again.
	if mine := a.myTunnels(); len(mine) > 0 {
		if s := a.serverByID(mine[0].CR.ServerID); s != nil {
			return cardBox("Current server", "", badge("in use", toneSuccess),
				vstack(sp4,
					vstack(1,
						text(s.Tag, fsTitle, pal().Content, true),
						text(countryName(s.Country), fsSmall, pal().Muted, false),
					),
					vstack(0,
						kvRow("Address", s.IP+":"+s.Port, true),
						kvRow("Tunnel", mine[0].CR.Tag, false),
					),
				))
		}
	}
	if a.probing && a.probeResults == nil {
		return card("Closest server", "Measuring round-trip time to every server…", nil)
	}
	best, ok := a.bestProbe()
	if !ok {
		desc := "No server answered a ping."
		if len(a.probeResults) == 0 {
			desc = "Run a probe to find the server with the lowest round-trip time."
		}
		return cardBox("Closest server", desc,
			primaryBtn("Probe", func() { a.forceProbe() }).small(), nil)
	}

	connect := successBtn("Connect", func() {
		if s := a.serverByID(best.ServerID); s != nil {
			srv := *s
			a.confirm("Connect", "Connect to "+srv.Tag+"?", func() { a.connectToServer(srv) })
			return
		}
		a.fail("That server is no longer in your list")
	})

	title := text(best.Tag, fsTitle, pal().Content, true)
	where := text(countryName(best.Country), fsSmall, pal().Muted, false)

	return cardBox("Closest server", "", badge(fmt.Sprintf("%d ms", best.LatencyMS()), toneSuccess),
		vstack(sp4,
			vstack(1, title, where),
			vstack(0,
				kvRow("Address", best.IP, true),
				kvRow("Round trip", fmt.Sprintf("%d ms", best.LatencyMS()), true),
			),
			hstack(sp2, connect),
		))
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
