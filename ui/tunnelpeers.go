package ui

import (
	"fmt"
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

func looksLikePeer(s string) bool {
	s = strings.TrimSpace(s)
	if regexp.MustCompile(`^\*:\d{1,5}$`).MatchString(s) {
		return true
	}
	if regexp.MustCompile(`^[0-9a-fA-F:.[\]]+$`).MatchString(s) && (strings.Contains(s, ".") || strings.Contains(s, ":")) {
		return true
	}
	return false
}

func (a *App) tunnelPeersPage() fyne.CanvasObject {
	meta := client.FindTunnel(a.peersTag)
	if meta == nil {
		return container.NewVBox(
			widget.NewButton("Back", func() { a.show(pageTunnels) }),
			muted(`Tunnel "`+a.peersTag+`" not found.`),
		)
	}
	connected := a.activeByTag()[meta.Tag] != nil
	peers := append([]string(nil), meta.AllowedHosts...)
	allowAll := meta.AllowAll

	status := "disconnected"
	if connected {
		status = "connected"
	}
	head := widget.NewLabel(fmt.Sprintf("%s  ·  firewall  ·  %s", meta.Tag, status))
	head.TextStyle = fyne.TextStyle{Bold: true}

	desc := "Enforcing allowlist"
	if allowAll {
		desc = "Firewall disabled for this device — any VPN peer can reach it."
	} else if len(peers) == 0 {
		desc = "No devices can reach this device while the server firewall is on."
	} else {
		desc = fmt.Sprintf("%d peer(s) allowed to reach this device.", len(peers))
	}

	toggle := bindCheck("Allow all", allowAll, func(v bool) {
		go func() {
			_, err := client.SetTunnelPeers(meta.Tag, peers, v)
			a.uiDo(func() {
				if err != nil {
					a.fail(err.Error())
					return
				}
				a.refreshState()
				a.note("Peer list updated")
				a.rebuild()
			})
		}()
	})

	entry := widget.NewEntry()
	entry.SetPlaceHolder("IP, IP:PORT, or *:PORT")
	add := primaryBtn("Add", func() {
		ip := strings.TrimSpace(entry.Text)
		if ip == "" {
			return
		}
		if !looksLikePeer(ip) {
			a.fail("Peer must be an IP, IP:PORT, or *:PORT")
			return
		}
		for _, p := range peers {
			if p == ip {
				a.fail("Peer is already in the list")
				return
			}
		}
		next := append(peers, ip)
		go func() {
			_, err := client.SetTunnelPeers(meta.Tag, next, allowAll)
			a.uiDo(func() {
				if err != nil {
					a.fail(err.Error())
					return
				}
				a.refreshState()
				a.note("Peer list updated")
				a.rebuild()
			})
		}()
	})

	rows := []fyne.CanvasObject{}
	if len(peers) == 0 {
		rows = append(rows, muted("No peers"))
	}
	for _, p := range peers {
		p := p
		del := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
			next := make([]string, 0, len(peers))
			for _, x := range peers {
				if x != p {
					next = append(next, x)
				}
			}
			go func() {
				_, err := client.SetTunnelPeers(meta.Tag, next, allowAll)
				a.uiDo(func() {
					if err != nil {
						a.fail(err.Error())
						return
					}
					a.refreshState()
					a.note("Peer list updated")
					a.rebuild()
				})
			}()
		})
		rows = append(rows, container.NewBorder(nil, nil, nil, del, widget.NewLabel(p)))
	}

	return pageScroll(
		toolbar(head, ghostBtn("Back", func() { a.show(pageTunnels) })),
		card("Firewall", desc, toggle),
		card("Allowed peers", "", container.NewVBox(
			container.NewBorder(nil, nil, nil, add, entry),
			container.NewVBox(rows...),
		)),
	)
}
