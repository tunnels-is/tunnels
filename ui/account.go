package ui

import (
	"encoding/json"
	"sort"
	"strings"

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
	}, a.accountTab, func(key string) {
		a.accountTab = key
		a.reloadCurrent()
	})

	var content fyne.CanvasObject
	switch a.accountTab {
	case "logins":
		content = a.accountLoginsTab(u)
	default:
		content = a.accountInfoTab(u)
	}

	title := u.Email
	if title == "" {
		title = "Anonymous account"
	}
	return pageShell(title, "Access until "+fmtTime(u.SubExpiration), tabs, content)
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
		kvRow("Access until", fmtTime(u.SubExpiration), false),
	))

	apiKey := cardBox("API key",
		"Used by the CLI and the local API. Regenerating replaces the old key immediately. The server stores only a hash, so copy the key when it is shown.",
		apiKeyActions(a, u),
		vstack(sp3, apiKeyBody(a, u)...))

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

func apiKeyVisible(key string) bool {
	return key != "" && !strings.HasPrefix(key, "h1:")
}

func apiKeyActions(a *App, u *client.User) fyne.CanvasObject {
	if !apiKeyVisible(u.APIKey) {
		return nil
	}
	return a.copyBtn(u.APIKey)
}

func apiKeyBody(a *App, u *client.User) []fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, 3)
	if apiKeyVisible(u.APIKey) {
		rows = append(rows,
			hint("Copy this key if you need it outside this device. Signing in again does not fetch it from the server."),
			container.NewStack(surface(radSm, pal().Elevate, pal().Base300),
				insetXY(sp3, sp2+2, monoText(u.APIKey, fsSmall, pal().Content))),
		)
	} else {
		rows = append(rows, hint("No API key is saved on this device. Generate one and copy it now. The server cannot show it again."))
	}
	rows = append(rows, hstack(sp2, outlineBtn("Regenerate key", func() {
		a.confirm("Regenerate API key", "The current key stops working immediately. The new key is shown once, so copy it before you leave. Continue?", func() {
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
					a.note("Copy the new API key now. The server will not show it again.")
					a.rebuild()
				})
			}()
		})
	})))
	return rows
}

func (a *App) accountLoginsTab(u *client.User) fyne.CanvasObject {
	tokens := append([]*client.DeviceToken(nil), u.Tokens...)
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
					next := []*client.DeviceToken{}
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
