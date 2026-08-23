package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) accountsPage() fyne.CanvasObject {
	users, err := client.GetUsers()
	if err != nil {
		a.fail(err.Error())
	}
	a.users = users

	add := primaryBtn("Add account", func() {
		a.loginMode = modeLogin
		a.show(pageLogin)
	}).withIcon(theme.ContentAddIcon())

	if len(users) == 0 {
		return pageShell("Accounts", "Saved on this device", add,
			emptyState("No saved accounts", "Sign in and choose Remember me to keep an account here."))
	}

	cards := make([]fyne.CanvasObject, 0, len(users))
	for _, u := range users {
		u := u
		server := "unknown server"
		if u.ControlServer != nil {
			server = u.ControlServer.Host + ":" + u.ControlServer.Port
		}
		email := u.Email
		if email == "" {
			email = "Anonymous account"
		}

		use := primaryBtn("Use this account", func() {
			a.setUser(u)
			a.show(pageServers)
			a.fetchServers(true)
		})
		remove := newIconBtn(theme.DeleteIcon(), kGhost, func() {
			a.confirm("Remove account", "Delete the saved account "+email+" from this device?", func() {
				if u.SaveFileHash != "" {
					_ = client.DeleteUser(u.SaveFileHash)
				}
				a.note("Account removed")
				a.rebuild()
			})
		})

		cards = append(cards, cardBox(email, server, remove, vstack(sp4,
			vstack(0,
				kvRow("User ID", u.ID, true),
				kvRow("Subscription", fmtTime(u.SubExpiration), false),
			),
			hstack(sp2, use),
		)))
	}

	sub := "1 account saved on this device"
	if len(users) != 1 {
		sub = fmtCount(len(users), "accounts saved on this device")
	}
	return pageShell("Accounts", sub, add, scrollBody(cards...))
}
