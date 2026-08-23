package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
		a.loginMode = 1
		a.show(pageLogin)
	})

	cards := []fyne.CanvasObject{toolbar(heading("Accounts"), add)}
	if len(users) == 0 {
		cards = append(cards, muted("No saved accounts"))
	}
	for _, u := range users {
		u := u
		server := "?"
		if u.ControlServer != nil {
			server = u.ControlServer.Host + ":" + u.ControlServer.Port
		}
		email := u.Email
		if email == "" {
			email = "anonymous"
		}
		body := container.NewVBox(
			kInfo("Email", email),
			kInfo("ID", u.ID),
			kInfo("Server", server),
			kInfo("Expiration", fmtTime(u.SubExpiration)),
			vspace(8),
			container.NewHBox(
				primaryBtn("Use this account", func() {
					a.setUser(u)
					a.show(pageServers)
					a.fetchServers(true)
				}),
				iconBtn(theme.DeleteIcon(), func() {
					a.confirm("Remove account", "Delete saved account "+email+" from this device?", func() {
						if u.SaveFileHash != "" {
							_ = client.DeleteUser(u.SaveFileHash)
						}
						a.rebuild()
					})
				}),
			),
		)
		cards = append(cards, card(email, server, body))
	}
	return pageScroll(cards...)
}
