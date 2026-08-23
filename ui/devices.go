package ui

import (
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

	filter := kSearch("Filter by tag or IP…", a.filterDevices, func(s string) {
		a.filterDevices = s
	}, func(s string) {
		a.filterDevices = s
		a.reloadCurrent()
	})
	create := primaryBtn("Create", func() { a.createDeviceDialog() })
	head := pageHeader("Devices", "", kSearchBox(filter), hspace(8), create)

	a.deviceList = widget.NewList(
		func() int { return len(a.deviceView) },
		func() fyne.CanvasObject { return newKRow() },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			row, ok := obj.(*kRow)
			if !ok {
				return
			}
			a.bindDeviceRow(id, row)
		},
	)

	return listPage(head, a.deviceList)
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
	badge := "Other machine"
	if locID || locPub {
		badge = "This device"
	}
	pill := badge
	if isConn {
		pill = "Connected"
	}
	row.SetTitleMeta(d.Tag, d.WireGuardIP+"  ·  "+fmtTime(d.CreatedAt)+"  ·  "+badge, isConn, pill)
	row.ghost.SetHidden(true)
	row.iconA.SetHidden(true)
	row.main.SetHidden(true)
	dev := d
	row.iconB.SetIconOnly(theme.DeleteIcon(), kDanger, func() {
		a.confirm("Delete device", `Delete "`+dev.Tag+`"? This cannot be undone.`, func() {
			go func() {
				_, _, err := a.callController("/client/device/delete", map[string]any{"DeviceID": dev.ID.String()}, true)
				a.uiDo(func() {
					if err != nil {
						a.fail(err.Error())
						return
					}
					a.fetchDevices()
				})
			}()
		})
	})
}

func (a *App) createDeviceDialog() {
	a.fetchServers(false)
	tag := widget.NewEntry()
	tag.SetPlaceHolder("e.g. my-laptop")
	opts := []string{}
	ids := map[string]string{}
	for _, s := range a.servers {
		label := s.Tag + " (" + countryName(s.Country) + ")"
		opts = append(opts, label)
		ids[label] = s.ID.String()
	}
	sel := widget.NewSelect(opts, nil)
	if len(opts) > 0 {
		sel.SetSelected(opts[0])
	}
	form := container.NewVBox(labeled("Tag", tag), labeled("Server", sel))
	d := dialog.NewCustomConfirm("New device", "Create", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		if strings.TrimSpace(tag.Text) == "" {
			a.fail("Please enter a device tag")
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
		a.note("Creating device...")
		form := &client.CreateDeviceWithKeysForm{
			Server:      a.user.ControlServer,
			Tag:         strings.TrimSpace(tag.Text),
			ServerID:    sid,
			DeviceToken: a.user.DeviceToken.DT,
			UID:         a.user.ID,
		}
		go func() {
			data, code := client.CreateDeviceWithKeys(form)
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
	d.Resize(fyne.NewSize(400, 280))
	d.Show()
}

func (a *App) showDeviceConfig(tag, cfg string) {
	save := widget.NewButton("Save .conf", func() {
		fd := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if err != nil || uc == nil {
				return
			}
			defer uc.Close()
			_, _ = uc.Write([]byte(cfg))
			a.note("Saved")
		}, a.win)
		fd.SetFileName(tag + ".conf")
		if home, err := os.UserHomeDir(); err == nil {
			if lister, err := storage.ListerForURI(storage.NewFileURI(filepath.Join(home))); err == nil {
				fd.SetLocation(lister)
			}
		}
		fd.Show()
	})
	content := container.NewVBox(
		wrapLabel("Save this config — it cannot be shown again"),
		qrImage(cfg, 220),
		widget.NewLabel(cfg),
		save,
	)
	d := dialog.NewCustom("Device config", "Done", container.NewVScroll(content), a.win)
	d.Resize(fyne.NewSize(480, 640))
	d.Show()
}
