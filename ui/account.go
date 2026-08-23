package ui

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) accountPage() fyne.CanvasObject {
	if a.user == nil {
		return container.NewCenter(muted("Not logged in"))
	}
	u := a.user

	tabs := container.NewAppTabs(
		container.NewTabItem("Account", a.accountInfoTab(u)),
		container.NewTabItem("Logins", a.accountLoginsTab(u)),
		container.NewTabItem("License", a.accountLicenseTab(u)),
	)
	switch a.accountTab {
	case "logins":
		tabs.SelectIndex(1)
	case "license":
		tabs.SelectIndex(2)
	default:
		tabs.SelectIndex(0)
	}
	tabs.OnSelected = func(t *container.TabItem) {
		switch t.Text {
		case "Logins":
			a.accountTab = "logins"
		case "License":
			a.accountTab = "license"
		default:
			a.accountTab = "account"
		}
	}
	return container.NewPadded(tabs)
}

func (a *App) accountInfoTab(u *client.User) fyne.CanvasObject {
	return pageScroll(
		card("Account", "", container.NewVBox(
			infoRow("User", nonempty(u.Email, "anonymous")),
			infoRow("ID", u.ID),
			infoRow("Updated", fmtTime(u.Updated)),
			infoRow("Subscription", fmtTime(u.SubExpiration)),
			infoRow("API key", u.APIKey),
			infoRow("Trial", boolLabel(u.Trial)),
		)),
		container.NewHBox(
			ghostBtn("Switch account", func() {
				a.clearSession()
				a.show(pageAccounts)
			}),
			outlineBtn("Re-generate API key", func() {
				go func() {
					raw, _, err := a.callController("/client/user/update", map[string]any{"APIKey": "generate"}, true)
					a.uiDo(func() {
						if err != nil {
							a.fail(err.Error())
							return
						}
						var resp struct{ APIKey string }
						_ = json.Unmarshal(raw, &resp)
						if resp.APIKey != "" {
							u.APIKey = resp.APIKey
							a.setUser(u)
							a.note("User updated")
							a.rebuild()
							return
						}
						a.fail("Key refresh returned no key")
					})
				}()
			}),
			outlineBtn("Two-factor auth", func() { a.show(pageTwoFactor) }),
			dangerBtn("Log out all devices", func() { a.logout(true) }),
			dangerBtn("Logout", func() { a.logout(false) }),
		),
	)
}

func (a *App) accountLoginsTab(u *client.User) fyne.CanvasObject {
	tokens := append([]*client.DEVICE_TOKEN(nil), u.Tokens...)
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].Created.After(tokens[j].Created) })
	rows := []fyne.CanvasObject{}
	if len(tokens) == 0 {
		rows = append(rows, muted("No active sessions"))
	}
	for _, t := range tokens {
		t := t
		name := t.N
		if sameSession(t, u.DeviceToken) {
			name += "  (current)"
		}
		btn := dangerBtn("Logout", func() {
			body := map[string]any{}
			if t.DT != "" {
				body["LogoutToken"] = t.DT
			} else {
				body["LogoutName"] = t.N
				body["LogoutCreated"] = t.Created
			}
			own := sameSession(t, u.DeviceToken)
			go func() {
				_, code, err := a.callController("/client/user/logout", body, true)
				a.uiDo(func() {
					if err != nil && code != 401 {
						a.fail("Unable to log out device")
						return
					}
					a.note("Device logged out")
					if own || code == 401 {
						if u.SaveFileHash != "" {
							_ = client.DeleteUser(u.SaveFileHash)
						}
						a.clearSession()
						a.show(pageLogin)
						return
					}
					next := []*client.DEVICE_TOKEN{}
					for _, x := range u.Tokens {
						if !sameSession(x, t) {
							next = append(next, x)
						}
					}
					u.Tokens = next
					a.setUser(u)
					a.rebuild()
				})
			}()
		})
		rows = append(rows, container.NewBorder(nil, nil, nil, btn,
			container.NewVBox(widget.NewLabel(name), muted(fmtTime(t.Created)))))
	}
	return pageScroll(card("Logins", "", container.NewVBox(rows...)))
}

func (a *App) accountLicenseTab(u *client.User) fyne.CanvasObject {
	current := ""
	if u.Key != nil {
		current = u.Key.Key
	}
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Insert license key")
	act := primaryBtn("Activate", func() {
		key := strings.TrimSpace(entry.Text)
		if key == "" {
			a.fail("License key is required")
			return
		}
		go func() {
			_, _, err := a.callController("/client/key/activate", map[string]any{"Key": key}, true)
			a.uiDo(func() {
				if err != nil {
					a.fail(err.Error())
					return
				}
				u.Key = &client.LicenseKey{Key: "[shown on next login]", Created: time.Now()}
				a.setUser(u)
				a.note("License activated")
				a.rebuild()
			})
		}()
	})
	objs := []fyne.CanvasObject{}
	if current != "" {
		objs = append(objs, card("Current", "", infoRow("Key", current)))
	}
	objs = append(objs, card("Activate license key", "", container.NewBorder(nil, nil, nil, act, kInput(entry))))
	return pageScroll(objs...)
}

func nonempty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func boolLabel(v bool) string {
	if v {
		return "Active"
	}
	return "—"
}
