package ui

import (
	"fmt"
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/client"
)

var (
	rePeerWildcard = regexp.MustCompile(`^\*:\d{1,5}$`)
	rePeerAddress  = regexp.MustCompile(`^[0-9a-fA-F:.[\]]+$`)
)

func looksLikePeer(s string) bool {
	s = strings.TrimSpace(s)
	if rePeerWildcard.MatchString(s) {
		return true
	}
	return rePeerAddress.MatchString(s) && (strings.Contains(s, ".") || strings.Contains(s, ":"))
}

func (a *App) tunnelPeersPage() fyne.CanvasObject {
	back := outlineBtn("Back", func() { a.show(pageTunnels) }).withIcon(theme.NavigateBackIcon())

	meta := client.FindTunnel(a.peersTag)
	if meta == nil {
		return pageShell("Firewall", a.peersTag, back,
			emptyState("Tunnel not found", `No tunnel named "`+a.peersTag+`" exists any more.`))
	}
	connected := a.activeByTag()[meta.Tag] != nil
	peers := append([]string(nil), meta.AllowedHosts...)

	apply := func(next []string) {
		go func() {
			_, err := client.SetTunnelPeers(meta.Tag, next, false)
			a.uiDo(func() {
				if err != nil {
					a.fail(err.Error())
					return
				}
				a.refreshState()
				a.note("Firewall updated")
				a.rebuild()
			})
		}()
	}

	desc := fmt.Sprintf("%d peer(s) may reach this device. Only these addresses may open connections.", len(peers))
	statusTone := toneSuccess
	if len(peers) == 0 {
		desc = "No peers can reach this device."
		statusTone = toneNeutral
	}

	entry := kEntry("IP, IP:PORT or *:PORT", "")
	add := primaryBtn("Add peer", func() {
		ip := strings.TrimSpace(entry.Text)
		if ip == "" {
			return
		}
		if !looksLikePeer(ip) {
			a.fail("A peer must be an IP, IP:PORT or *:PORT")
			return
		}
		for _, p := range peers {
			if p == ip {
				a.fail("That peer is already in the list")
				return
			}
		}
		apply(append(peers, ip))
	})
	entry.OnSubmitted = func(string) { add.Tapped(nil) }

	rows := []fyne.CanvasObject{}
	for _, p := range peers {
		p := p
		del := newIconBtn(theme.DeleteIcon(), kGhost, func() {
			next := make([]string, 0, len(peers))
			for _, x := range peers {
				if x != p {
					next = append(next, x)
				}
			}
			apply(next)
		}).small()
		rows = append(rows, insetEach(sp2, 0, sp2, 0,
			splitRow(monoText(p, fsBody, pal().Content), del)))
	}
	if len(rows) == 0 {
		rows = append(rows, emptyRow("No peers allowed."))
	}

	list := cardBox("Allowed peers", desc, nil,
		vstack(sp4,
			capWidth(formWidth, container.NewBorder(nil, nil, nil, hstack(0, hspace(sp2), add), entry)),
			settingList(rows...),
		))

	sub := meta.Tag + " · disconnected"
	if connected {
		sub = meta.Tag + " · connected"
	}
	head := hstack(sp2, badge("enforcing", statusTone), back)
	return pageShell("Firewall", sub, head, scrollBody(list))
}
