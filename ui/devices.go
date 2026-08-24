package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
	"github.com/tunnels-is/tunnels/types"
)

func (a *App) recomputeDeviceView() {
	var shown []types.Device
	for _, d := range a.devices {
		if filterMatch(a.filterDevices, d.Tag, d.WireGuardIP) {
			shown = append(shown, d)
		}
	}
	a.deviceView = shown
}

func (a *App) devicesPage() fyne.CanvasObject {
	if a.loggedIn() && !a.devicesLoaded {
		a.fetchDevices()
	}
	a.recomputeDeviceView()

	_, search := searchField("Filter devices", a.filterDevices, func(s string) {
		a.filterDevices = s
	}, func(s string) {
		a.filterDevices = s
		a.reloadCurrent()
	})
	create := primaryBtn("New device", func() { a.createDeviceDialog() }).withIcon(theme.ContentAddIcon())
	actions := hstackFlex(sp2, 0, search, create)

	sub := fmt.Sprintf("%d registered", len(a.deviceView))
	if a.devicesFetching && len(a.deviceView) == 0 {
		sub = "Loading…"
	}

	if len(a.deviceView) == 0 {
		msg, desc := "No devices", "Nothing matched this filter."
		if a.filterDevices == "" {
			msg, desc = "No devices yet", "Create a device to get a WireGuard config for it."
		}
		return pageShell("Devices", sub, actions, emptyState(msg, desc))
	}

	a.deviceList = newRowList(
		func() int { return len(a.deviceView) },
		a.bindDeviceRow,
	)

	return pageShell("Devices", sub, actions, listBody(a.deviceList))
}

func (a *App) bindDeviceRow(id widget.ListItemID, row *kRow) {
	if id < 0 || id >= len(a.deviceView) {
		return
	}
	d := a.deviceView[id]

	connectedIPs := map[string]struct{}{}
	for _, at := range a.active {
		if at != nil && at.ServerResponse != nil && at.ServerResponse.WireGuardIP != "" {
			connectedIPs[at.ServerResponse.WireGuardIP] = struct{}{}
		}
	}
	localIDs := map[string]struct{}{}
	localPubs := map[string]struct{}{}
	for _, ld := range a.localDevices {
		if ld.ID != "" {
			localIDs[ld.ID] = struct{}{}
		}
		if ld.WireGuardPubKey != "" {
			localPubs[ld.WireGuardPubKey] = struct{}{}
		}
	}
	_, isConn := connectedIPs[d.WireGuardIP]
	_, locID := localIDs[d.ID.String()]
	_, locPub := localPubs[d.WireGuardKey]

	mine := locID || locPub
	pill, t := "Remote", toneNeutral
	if mine {
		pill, t = "This device", tonePrimary
	}
	if isConn {
		pill, t = "Connected", toneSuccess
	}

	row.SetRow(d.Tag, d.WireGuardIP+"  ·  added "+fmtTime(d.CreatedAt), isConn, pill, t)
	row.ghost.SetHidden(true)
	row.iconA.SetHidden(true)
	row.main.SetHidden(true)

	dev := d
	row.iconB.SetIconOnly(theme.DeleteIcon(), kGhost, func() {
		a.confirm("Delete device", `Delete "`+dev.Tag+`"? This cannot be undone.`, func() {
			go func() {
				_, _, err := a.callController("/client/device/delete", map[string]any{"DeviceID": dev.ID.String()}, true)
				a.uiDo(func() {
					if err != nil {
						a.fail(err.Error())
						return
					}
					a.note("Device deleted")
					a.fetchDevices()
				})
			}()
		})
	})
}

func (a *App) createDeviceDialog() {
	a.fetchServers(false)
	tag := kEntry("e.g. my-laptop", "")
	opts := []string{}
	ids := map[string]string{}
	for _, s := range a.servers {
		label := s.Tag
		if c := countryName(s.Country); c != "" {
			label += " · " + c
		}
		opts = append(opts, label)
		ids[label] = s.ID.String()
	}
	sel := widget.NewSelect(opts, nil)
	if len(opts) > 0 {
		sel.SetSelected(opts[0])
	}

	form := container.New(fixedLayout{w: z(360)},
		vstack(sp3, field("Device name", tag), field("Server", sel)))

	d := dialog.NewCustomConfirm("New device", "Create", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		if strings.TrimSpace(tag.Text) == "" {
			a.fail("Please enter a device name")
			return
		}
		sid := ids[sel.Selected]
		if sid == "" {
			a.fail("Please select a server")
			return
		}
		if a.user == nil || a.user.DeviceToken == nil {
			a.fail("You are not logged in")
			return
		}
		a.note("Creating device…")
		req := &client.CreateDeviceWithKeysForm{
			Server:      a.user.ControlServer,
			Tag:         strings.TrimSpace(tag.Text),
			ServerID:    sid,
			DeviceToken: a.user.DeviceToken.DT,
			UID:         a.user.ID,
		}
		go func() {
			data, code := client.CreateDeviceWithKeys(req)
			a.uiDo(func() {
				if code != 200 {
					if er, ok := data.(*client.ErrorResponse); ok {
						a.fail(er.Error)
					} else {
						a.fail("Unable to create device")
					}
					return
				}
				res, _ := data.(*client.CreateDeviceResult)
				if res == nil || res.WGConfig == "" {
					a.fail("No config returned")
					return
				}
				a.showDeviceConfig(strings.TrimSpace(tag.Text), res.WGConfig)
				a.fetchDevices()
			})
		}()
	}, a.win)
	d.Resize(fyne.NewSize(z(420), z(300)))
	d.Show()
}

func (a *App) showDeviceConfig(tag, cfg string) {
	save := outlineBtn("Save .conf", func() {
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()
			_, _ = uc.Write([]byte(cfg))
			a.note("Config saved")
		}, a.win)
		fd.SetFileName(tag + ".conf")
		if home, err := os.UserHomeDir(); err == nil {
			if lister, err := storage.ListerForURI(storage.NewFileURI(filepath.Join(home))); err == nil {
				fd.SetLocation(lister)
			}
		}
		fd.Show()
	}).withIcon(theme.DocumentSaveIcon())

	content := vstack(sp4,
		notice("Save this config now — it cannot be shown again.", toneWarning),
		qrImage(cfg, int(z(200))),
		codeBlock(cfg, 8),
		hstack(sp2, save, a.copyBtn(cfg)),
	)
	d := dialog.NewCustom("Device config · "+tag, "Done",
		container.NewVScroll(inset(sp1, content)), a.win)
	d.Resize(fyne.NewSize(z(480), z(660)))
	d.Show()
}
