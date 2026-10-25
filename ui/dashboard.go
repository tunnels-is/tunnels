package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

// dashNameSize is the closest server's name. It stays larger than a card
// title so the name is the first thing you read.
func dashNameSize() float32 { return snapDIP(z(34)) }

// dashLatencySize is the round-trip figure above that name.
func dashLatencySize() float32 { return snapDIP(z(22)) }

// ---------------------------------------------------------------- page

func (a *App) dashboardPage() fyne.CanvasObject {
	// Building a page must not start work. The first probe is chained off the
	// server fetch instead, so it also fires when the list is still in flight.
	a.maybeAutoProbe()
	// Device tags name the direct paths. Skip this in tests, which build the
	// page without an app and must not open a controller request.
	if a.fyneApp != nil && a.loggedIn() && !a.devicesLoaded {
		a.fetchDevices()
	}
	a.armDashWatch()

	col := vstack(sp6, a.dashHero(), a.dashTunnelSection())
	return pageShell("Dashboard", "", nil, scrollBodyOf(capWidth(z(880), col)))
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

// dashPanel is the dashboard card: the same surface as every other panel,
// with a little more air so the hero and the tunnel cards share one rhythm.
func dashPanel(content fyne.CanvasObject) fyne.CanvasObject {
	p := pal()
	return container.NewStack(surface(radLg, p.Base100, p.Base300), inset(sp5, content))
}

// dashHero is the closest server and the one-step connect action.
func (a *App) dashHero() fyne.CanvasObject {
	best, ok := a.bestProbe()
	if a.probing && !ok {
		return dashPanel(vstack(sp2,
			eyebrow("Closest server"),
			text("Measuring", dashNameSize(), pal().Content, true),
			text("Round-trip time to every server.", fsBody, pal().Muted, false),
		))
	}
	if !ok {
		msg := "No server answered"
		sub := "Measure again to find a server that responds."
		if len(a.probeResults) == 0 {
			msg = "Find the closest server"
			sub = "A probe measures round-trip time to every server."
		}
		probe := primaryBtn("Probe", func() { a.forceProbe() }).withIcon(theme.ViewRefreshIcon())
		return dashPanel(splitRow(
			vstack(sp2,
				eyebrow("Closest server"),
				text(msg, dashNameSize(), pal().Content, true),
				text(sub, fsBody, pal().Muted, false),
			),
			probe,
		))
	}

	p := pal()
	head := []fyne.CanvasObject{eyebrow("Closest server")}
	onThis := a.onServer(best.ServerID)
	if onThis {
		head = append(head, badge("Connected", toneSuccess))
	}
	lines := []fyne.CanvasObject{
		hstack(sp2, head...),
		text(fmt.Sprintf("%d ms", best.LatencyMS()), dashLatencySize(), p.Success, true),
		text(best.Tag, dashNameSize(), p.Content, true),
	}
	var meta []fyne.CanvasObject
	if c := countryName(best.Country); c != "" {
		meta = append(meta, text(c, fsBody, p.Muted, false))
	}
	if best.IP != "" {
		meta = append(meta, monoText(best.IP, fsBody, p.Muted))
	}
	if len(meta) > 0 {
		lines = append(lines, hstack(sp2, meta...))
	}
	if age := probeAge(a.probeAt); age != "" {
		lines = append(lines, text(age, fsCaption, p.Faint, false))
	}

	measureLabel := "Measure"
	if a.probing {
		measureLabel = "Measuring"
	}
	measure := ghostBtn(measureLabel, func() {
		if !a.probing {
			a.forceProbe()
		}
	}).small().withIcon(theme.ViewRefreshIcon())
	actions := fyne.CanvasObject(measure)
	if !onThis {
		connect := successBtn("Connect", func() {
			srv, ok := a.closestServer()
			if !ok {
				a.fail("That server is no longer in your list")
				return
			}
			a.confirm("Connect", "Connect to "+srv.Tag+"?", func() { a.connectToServer(srv) })
		})
		actions = hstack(sp2, measure, connect)
	}

	return dashPanel(splitRow(vstack(sp2, lines...), actions))
}

// dashTunnelSection is every tunnel this account has up on this computer.
func (a *App) dashTunnelSection() fyne.CanvasObject {
	mine := a.myTunnels()
	sub := "Connect above to bring one up"
	if n := len(mine); n == 1 {
		sub = "1 tunnel connected"
	} else if n > 1 {
		sub = fmt.Sprintf("%d tunnels connected", n)
	}
	var actions fyne.CanvasObject
	if len(mine) > 0 {
		actions = a.bwRangePicker()
	}
	head := splitRow(vstack(z(2),
		text("Tunnels", fsLarge, pal().Content, true),
		text(sub, fsSmall, pal().Muted, false),
	), actions)

	if len(mine) == 0 {
		return vstack(sp3, head, dashPanel(vstack(sp2,
			text("Nothing connected", fsBody, pal().Content, true),
			text("A connected tunnel shows its throughput and direct paths here.", fsSmall, pal().Muted, false),
		)))
	}
	cards := make([]fyne.CanvasObject, 0, len(mine))
	for _, t := range mine {
		cards = append(cards, a.tunnelDashCard(t))
	}
	return vstack(sp3, append([]fyne.CanvasObject{head}, cards...)...)
}

func (a *App) tunnelDashCard(t *client.TUN) fyne.CanvasObject {
	tag := "Tunnel"
	if t != nil && t.CR != nil && t.CR.Tag != "" {
		tag = t.CR.Tag
	}
	where := ""
	if t != nil && t.CR != nil {
		if s := a.serverByID(t.CR.ServerID); s != nil {
			where = s.Tag
			if c := countryName(s.Country); c != "" {
				if where != "" {
					where += "  ·  "
				}
				where += c
			}
		}
	}
	addr := ""
	if t != nil && t.ServerResponse != nil {
		addr = t.ServerResponse.WireGuardIP
	}

	tun := t
	disc := dangerBtn("Disconnect", func() {
		a.confirm("Disconnect", "Disconnect "+tag+"?", func() { a.disconnectActive(tun) })
	}).small()
	head := cardHead(tag, hstack(sp2, badge("Connected", toneSuccess), disc))

	meta := make([]fyne.CanvasObject, 0, 2)
	if where != "" {
		meta = append(meta, text(where, fsSmall, pal().Muted, false))
	}
	if addr != "" {
		meta = append(meta, monoText(addr, fsSmall, pal().Muted))
	}

	panel := newBandwidthPanel(t, a.bwRangeSeconds())
	a.bwLive = append(a.bwLive, panel)

	rows := []fyne.CanvasObject{head}
	if len(meta) > 0 {
		rows = append(rows, vstack(z(2), meta...))
	}
	rows = append(rows,
		divider(),
		vstack(sp2, eyebrow("Bandwidth"), panel),
		divider(),
		a.directBlock(t),
	)
	return dashPanel(vstack(sp4, rows...))
}

func (a *App) directBlock(t *client.TUN) fyne.CanvasObject {
	peers := t.DirectPeers()
	label := eyebrow("Direct")
	if len(peers) == 0 {
		return vstack(sp2, label, text("No direct paths", fsSmall, pal().Muted, false))
	}
	rows := []fyne.CanvasObject{hstack(sp2, label, badge(fmt.Sprintf("%d", len(peers)), toneSuccess))}
	for i, p := range peers {
		if i > 0 {
			rows = append(rows, divider())
		}
		rows = append(rows, a.directRow(p))
	}
	return vstack(sp2, rows...)
}

func (a *App) directRow(p client.DirectPeer) fyne.CanvasObject {
	name := a.deviceLabel(p.DeviceID, p.TunnelIP)
	lines := []fyne.CanvasObject{text(name, fsBody, pal().Content, true)}
	if name != p.TunnelIP && p.TunnelIP != "" {
		lines = append(lines, monoText(p.TunnelIP, fsSmall, pal().Muted))
	}
	var end fyne.CanvasObject
	if p.Endpoint != "" {
		end = monoText(p.Endpoint, fsSmall, pal().Faint)
	}
	return splitRow(vstack(z(1), lines...), end)
}

func (a *App) deviceLabel(id, fallback string) string {
	for _, d := range a.devices {
		if d.ID.String() == id && d.Tag != "" {
			return d.Tag
		}
	}
	if fallback != "" {
		return fallback
	}
	return "Peer"
}

func (a *App) onServer(id string) bool {
	if id == "" {
		return false
	}
	_, ok := a.activeByServer()[id]
	return ok
}

func probeAge(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 5*time.Second {
		return "Measured just now"
	}
	if d < time.Minute {
		return fmt.Sprintf("Measured %ds ago", int(d.Seconds()))
	}
	return "Measured " + d.Round(time.Minute).String() + " ago"
}

// ---------------------------------------------------------------- actions

// maybeAutoProbe runs the first probe on its own: only when signed in, and
// only once. A tunnel that is already up still gets measured, because the
// closest server stays on screen above the live tunnels. Safe to call
// repeatedly; every guard is cheap.
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

// armDashWatch rebuilds the dashboard when a direct path comes up or drops.
// The bandwidth chart updates itself; this only watches the peer list.
func (a *App) armDashWatch() {
	if a.fyneApp == nil || a.dashWatch != nil {
		return
	}
	stop := make(chan struct{})
	a.dashWatch = stop
	sig := a.directSig()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				fyne.Do(func() {
					select {
					case <-stop:
						return
					default:
					}
					if a.current != pageDashboard || a.reloading {
						return
					}
					next := a.directSig()
					if next == sig {
						return
					}
					sig = next
					a.refreshState()
					a.reloadCurrent()
				})
			}
		}
	}()
}

func (a *App) stopDashWatch() {
	if a.dashWatch == nil {
		return
	}
	close(a.dashWatch)
	a.dashWatch = nil
}

func (a *App) directSig() string {
	var b strings.Builder
	for _, t := range a.myTunnels() {
		if t.CR != nil {
			b.WriteString(t.CR.Tag)
			b.WriteByte('@')
			b.WriteString(t.CR.ServerID)
		}
		b.WriteByte('|')
		for _, p := range t.DirectPeers() {
			b.WriteString(p.DeviceID)
			b.WriteByte('=')
			b.WriteString(p.TunnelIP)
			b.WriteByte('@')
			b.WriteString(p.Endpoint)
			b.WriteByte(';')
		}
		b.WriteByte('\n')
	}
	return b.String()
}
