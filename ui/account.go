package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) accountPage() fyne.CanvasObject {
	if a.user == nil {
		return pageShell("Account", "", nil, emptyState("Not signed in", "Sign in to manage your account."))
	}
	u := a.user

	tabs := newSegmented([]segItem{
		{"account", "Overview"},
		{"logins", "Sessions"},
		{"license", "License"},
	}, a.accountTab, func(key string) {
		a.accountTab = key
		a.reloadCurrent()
	})

	var content fyne.CanvasObject
	switch a.accountTab {
	case "logins":
		content = a.accountLoginsTab(u)
	case "license":
		content = a.accountLicenseTab(u)
	default:
		content = a.accountInfoTab(u)
	}

	title := u.Email
	if title == "" {
		title = "Anonymous account"
	}
	sub := "Trial"
	if !u.Trial {
		sub = "Subscription until " + fmtTime(u.SubExpiration)
	}
	return pageShell(title, sub, tabs, content)
}

func (a *App) accountInfoTab(u *client.User) fyne.CanvasObject {
	email := u.Email
	if email == "" {
		email = "anonymous"
	}

	details := card("Details", "", vstack(0,
		kvRow("Email", email, false),
		kvRow("User ID", u.ID, true),
		kvRow("Last updated", fmtTime(u.Updated), false),
		kvRow("Subscription", fmtTime(u.SubExpiration), false),
		kvRow("Trial", boolLabel(u.Trial), false),
	))

	apiKey := cardBox("API key",
		"Used by the CLI and the local API. Regenerating invalidates the old key immediately.",
		a.copyBtn(u.APIKey),
		vstack(sp3,
			container.NewStack(surface(radSm, pal().Elevate, pal().Base300),
				insetXY(sp3, sp2+2, monoText(u.APIKey, fsSmall, pal().Content))),
			hstack(sp2, outlineBtn("Regenerate key", func() {
				a.confirm("Regenerate API key", "The current key stops working immediately. Continue?", func() {
					go func() {
						raw, _, err := a.callController("/client/user/update", map[string]any{"APIKey": "generate"}, true)
						a.uiDo(func() {
							if err != nil {
								a.fail(err.Error())
								return
							}
							var resp struct{ APIKey string }
							_ = json.Unmarshal(raw, &resp)
							if resp.APIKey == "" {
								a.fail("Key refresh returned no key")
								return
							}
							u.APIKey = resp.APIKey
							a.setUser(u)
							a.note("API key regenerated")
							a.rebuild()
						})
					}()
				})
			})),
		))

	security := card("Security", "",
		settingList(
			settingRow("Two-factor authentication",
				"Protect sign-in with an authenticator app.",
				outlineBtn("Configure", func() { a.show(pageTwoFactor) })),
			settingRow("Switch account",
				"Sign in as a different saved account on this device.",
				outlineBtn("Switch", func() {
					a.clearSession()
					a.show(pageAccounts)
				})),
		))

	danger := card("Sign out", "",
		settingList(
			settingRow("This device", "Ends the session on this machine only.",
				dangerBtn("Sign out", func() { a.logout(false) })),
			settingRow("All devices", "Revokes every session token on the account.",
				dangerBtn("Sign out everywhere", func() {
					a.confirm("Sign out everywhere", "Every device will need to sign in again. Continue?",
						func() { a.logout(true) })
				})),
		))

	return scrollBody(details, apiKey, security, danger)
}

func (a *App) accountLoginsTab(u *client.User) fyne.CanvasObject {
	tokens := append([]*client.DEVICE_TOKEN(nil), u.Tokens...)
	sort.Slice(tokens, func(i, j int) bool { return tokens[i].Created.After(tokens[j].Created) })

	if len(tokens) == 0 {
		return scrollBody(card("Sessions", "", emptyRow("No active sessions.")))
	}

	rows := make([]fyne.CanvasObject, 0, len(tokens))
	for _, t := range tokens {
		t := t
		current := sameSession(t, u.DeviceToken)
		name := t.N
		if name == "" {
			name = "unnamed device"
		}

		title := []fyne.CanvasObject{text(name, fsBody, pal().Content, false)}
		if current {
			title = append(title, badge("this device", tonePrimary))
		}
		left := vstack(2,
			hstack(sp2, title...),
			text("Signed in "+fmtTime(t.Created), fsSmall, pal().Faint, false),
		)

		btn := dangerBtn("Revoke", func() {
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
						a.fail("Unable to revoke session")
						return
					}
					a.note("Session revoked")
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
		rows = append(rows, insetEach(sp3, 0, sp3, 0, splitRow(left, btn)))
	}

	return scrollBody(card("Active sessions",
		"Every signed-in device holds its own token. Revoke any you do not recognise.",
		settingList(rows...)))
}

func (a *App) accountLicenseTab(u *client.User) fyne.CanvasObject {
	entry := kEntry("TUN-XXXX-XXXX-XXXX", "")
	activate := primaryBtn("Activate", func() {
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
				u.Key = &client.LicenseKey{Key: "[shown on next sign-in]", Created: time.Now()}
				a.setUser(u)
				a.note("License activated")
				a.rebuild()
			})
		}()
	})

	cards := []fyne.CanvasObject{}
	if u.Key != nil && u.Key.Key != "" {
		cards = append(cards, card("Active license", "", vstack(0,
			kvRow("Key", u.Key.Key, true),
			kvRow("Activated", fmtTime(u.Key.Created), false),
			kvRow("Term", monthsLabel(u.Key.Months), false),
		)))
	}
	cards = append(cards, card("Add a license key",
		"Paste a key to extend your subscription.",
		capWidth(formWidth, vstack(sp3, field("License key", entry), hstack(0, activate)))))
	return scrollBody(cards...)
}

func monthsLabel(m int) string {
	switch {
	case m <= 0:
		return "—"
	case m == 1:
		return "1 month"
	default:
		return fmt.Sprintf("%d months", m)
	}
}

func boolLabel(v bool) string {
	if v {
		return "Active"
	}
	return "—"
}
