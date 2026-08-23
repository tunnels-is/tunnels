package ui

import (
	"encoding/json"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) loginPage() fyne.CanvasObject {
	pref := a.fyneApp.Preferences()
	email := widget.NewEntry()
	email.SetPlaceHolder("Email")
	email.SetText(pref.String("default-email"))
	pass := widget.NewPasswordEntry()
	pass.SetPlaceHolder("Password")
	pass2 := widget.NewPasswordEntry()
	pass2.SetPlaceHolder("Confirm password")
	device := widget.NewEntry()
	device.SetPlaceHolder("Device name")
	device.SetText(pref.String("default-device-name"))
	digits := widget.NewEntry()
	digits.SetPlaceHolder("Authenticator code (if enabled)")
	recovery := widget.NewEntry()
	recovery.SetPlaceHolder("Two factor recovery code")
	code := widget.NewEntry()
	code.SetPlaceHolder("Code")

	errLabel := muted("")
	errLabel.Importance = widget.DangerImportance

	remember := bindCheck("Remember", a.loginRemember, func(v bool) { a.loginRemember = v })

	mode := a.loginMode
	if mode == 0 {
		mode = 1
	}

	servers := []*client.ControlServer{}
	if a.config != nil {
		servers = a.config.ControlServers
	}
	opts := make([]string, 0, len(servers))
	optMap := map[string]*client.ControlServer{}
	for _, s := range servers {
		if s == nil {
			continue
		}
		label := s.Host + ":" + s.Port
		opts = append(opts, label)
		optMap[label] = s
	}
	serverSel := widget.NewSelect(opts, func(label string) {
		if s := optMap[label]; s != nil {
			a.loginServerID = s.ID
		}
	})
	if len(opts) > 0 {
		selected := opts[0]
		for _, s := range servers {
			if s != nil && s.ID == a.loginServerID {
				selected = s.Host + ":" + s.Port
				break
			}
		}
		serverSel.SetSelected(selected)
	}

	activeServer := func() *client.ControlServer {
		if serverSel.Selected != "" {
			return optMap[serverSel.Selected]
		}
		if len(servers) > 0 {
			return servers[0]
		}
		return nil
	}

	showToken := mode == 5 || (mode == 2 && a.loginToken)
	showEmail := (mode == 1 || mode == 2 || mode == 4 || mode == 6) && !showToken
	showPass := mode == 1 || mode == 2 || mode == 3 || mode == 4 || mode == 5
	showPass2 := mode == 2 || mode == 4 || mode == 5

	fields := []fyne.CanvasObject{}
	if mode == 5 {
		fields = append(fields, wrapLabel("Save your login token in a secure place. If you lose it the account is gone."))
	}
	if showToken {
		email.SetPlaceHolder("Token")
		fields = append(fields, kLabeled("Token", kInput(email)))
	}
	if showEmail {
		fields = append(fields, kLabeled("Email", kInput(email)))
	}
	if mode == 1 {
		fields = append(fields, kLabeled("Device name", kInput(device)))
	}
	if showPass {
		fields = append(fields, kLabeled("Password", kInput(pass)))
	}
	if showPass2 {
		fields = append(fields, kLabeled("Confirm password", kInput(pass2)))
	}
	if mode == 1 {
		fields = append(fields, kLabeled("2FA code", kInput(digits)))
	}
	if mode == 3 {
		fields = append(fields, kLabeled("Recovery code", kInput(recovery)))
	}
	if mode == 6 {
		fields = append(fields, kLabeled("Code", kInput(code)))
	}
	if mode == 4 {
		fields = append(fields, kLabeled("Reset code", kInput(code)))
	}

	submitLabel := map[int]string{1: "Login", 2: "Register", 3: "Login", 4: "Reset password", 5: "Register", 6: "Enable account"}[mode]

	submit := func() {
		errLabel.SetText("")
		srv := activeServer()
		if srv == nil {
			errLabel.SetText("No control server configured")
			return
		}
		switch mode {
		case 1, 3:
			if strings.TrimSpace(email.Text) == "" || pass.Text == "" {
				errLabel.SetText("Email and password are required")
				return
			}
			if mode == 1 && strings.TrimSpace(device.Text) == "" {
				errLabel.SetText("Device login name missing")
				return
			}
			if mode == 3 && strings.TrimSpace(recovery.Text) == "" {
				errLabel.SetText("Recovery code missing")
				return
			}
			body := map[string]any{
				"Email":      email.Text,
				"Password":   pass.Text,
				"DeviceName": device.Text,
				"Digits":     digits.Text,
				"Recovery":   recovery.Text,
			}
			a.note("Signing in...")
			go func() {
				raw, _, err := a.callControllerOn(srv, "/client/user/login", body, false)
				a.uiDo(func() {
					if err != nil {
						errLabel.SetText(err.Error())
						a.fail(err.Error())
						return
					}
					u := new(client.User)
					if err := json.Unmarshal(raw, u); err != nil {
						errLabel.SetText("Unable to parse login response")
						return
					}
					u.ControlServer = srv
					pref.SetString("default-email", email.Text)
					pref.SetString("default-device-name", device.Text)
					a.setUser(u)
					if a.loginRemember {
						_ = client.SaveUser(u)
					}
					a.note("Signed in")
					a.show(pageServers)
					a.fetchServers(true)
				})
			}()
		case 2, 5:
			if pass.Text == "" || len(pass.Text) < 10 {
				errLabel.SetText("Minimum 10 characters")
				return
			}
			if pass.Text != pass2.Text {
				errLabel.SetText("Passwords do not match")
				return
			}
			body := map[string]any{
				"Email":     email.Text,
				"Password":  pass.Text,
				"Password2": pass2.Text,
			}
			a.note("Creating account...")
			go func() {
				raw, _, err := a.callControllerOn(srv, "/client/user/create", body, false)
				a.uiDo(func() {
					if err != nil {
						errLabel.SetText(err.Error())
						a.fail(err.Error())
						return
					}
					u := new(client.User)
					if err := json.Unmarshal(raw, u); err != nil {
						errLabel.SetText("Unable to parse register response")
						return
					}
					u.ControlServer = srv
					a.setUser(u)
					a.note("Account created")
					a.show(pageServers)
				})
			}()
		case 4:
			if pass.Text == "" || len(pass.Text) < 9 || pass.Text != pass2.Text || code.Text == "" {
				errLabel.SetText("Fill password, confirm, and reset code")
				return
			}
			body := map[string]any{
				"Email":        email.Text,
				"Password":     pass.Text,
				"ResetCode":    code.Text,
				"UseTwoFactor": false,
			}
			go func() {
				_, _, err := a.callControllerOn(srv, "/client/user/reset/password", body, false)
				a.uiDo(func() {
					if err != nil {
						errLabel.SetText(err.Error())
						return
					}
					a.loginMode = 1
					a.note("Password reset")
					a.rebuild()
				})
			}()
		case 6:
			body := map[string]any{"Email": email.Text, "Code": code.Text, "ConfirmCode": code.Text}
			go func() {
				_, _, err := a.callControllerOn(srv, "/client/user/enable", body, false)
				a.uiDo(func() {
					if err != nil {
						errLabel.SetText(err.Error())
						return
					}
					a.loginMode = 1
					a.note("Account enabled")
					a.rebuild()
				})
			}()
		}
	}

	submitBtn := primaryBtn(submitLabel, submit)

	actions := []fyne.CanvasObject{submitBtn}
	if mode == 1 {
		actions = append(actions, remember)
	}
	if mode == 4 {
		actions = append(actions, ghostBtn("Send reset code", func() {
			srv := activeServer()
			if srv == nil {
				return
			}
			go func() {
				_, _, err := a.callControllerOn(srv, "/client/user/reset/code", map[string]any{"Email": email.Text}, false)
				a.uiDo(func() {
					if err != nil {
						a.fail(err.Error())
						return
					}
					a.note("Reset code sent")
				})
			}()
		}))
	}

	modes := []struct {
		v int
		l string
	}{
		{1, "Login"}, {2, "Register"}, {5, "Anonymous"}, {4, "Reset"}, {3, "2FA Recovery"}, {6, "Enable"},
	}
	modeBtns := []fyne.CanvasObject{}
	for _, m := range modes {
		m := m
		tap := func() {
			if m.v == 5 {
				a.loginToken = true
				email.SetText(uuid.New().String())
			} else if a.loginToken {
				a.loginToken = false
			}
			a.loginMode = m.v
			a.rebuild()
		}
		if m.v == mode {
			modeBtns = append(modeBtns, primaryBtn(m.l, tap))
		} else {
			modeBtns = append(modeBtns, ghostBtn(m.l, tap))
		}
	}

	editServer := iconBtn(theme.DocumentCreateIcon(), func() {
		s := activeServer()
		if s == nil {
			s = &client.ControlServer{ID: uuid.New().String(), ValidateCertificate: true}
		}
		a.editAuthServer(s)
	})
	addServer := iconBtn(theme.ContentAddIcon(), func() {
		a.editAuthServer(&client.ControlServer{ID: uuid.New().String(), ValidateCertificate: true})
	})

	dot := canvas.NewCircle(pal().Primary)
	brand := container.NewHBox(titleText("Tunnels"), container.NewCenter(container.NewStack(spacer(6, 6), dot)))
	sub := text("Sign in to your account", 13, pal().Muted, false)

	serverPanel := cardBox("", "", nil, container.NewBorder(nil, nil,
		container.NewHBox(hspace(4), caption("Server")),
		container.NewHBox(addServer, editServer),
		kInput(serverSel),
	))

	form := container.NewVBox(
		vspace(40),
		brand,
		sub,
		vspace(16),
		container.NewVBox(fields...),
		errLabel,
		vspace(8),
		container.NewHBox(actions...),
		vspace(12),
		serverPanel,
		vspace(12),
		container.NewHBox(modeBtns...),
	)
	return pageScroll(form)
}

func (a *App) editAuthServer(s *client.ControlServer) {
	host := widget.NewEntry()
	host.SetText(s.Host)
	port := widget.NewEntry()
	port.SetText(s.Port)
	cert := widget.NewEntry()
	cert.SetText(s.CertificatePath)
	validate := bindCheck("Validate certificate", s.ValidateCertificate, nil)
	warn := muted("Turning verification off lets a man-in-the-middle read login credentials. Only do this for a trusted self-signed server.")

	form := container.NewVBox(
		labeled("Host", host),
		labeled("Port", port),
		labeled("Certificate path", cert),
		validate,
		warn,
	)
	d := dialog.NewCustomConfirm("Auth server", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		s.Host = strings.TrimSpace(host.Text)
		s.Port = strings.TrimSpace(port.Text)
		s.CertificatePath = strings.TrimSpace(cert.Text)
		s.ValidateCertificate = validate.Checked
		cfg := client.CloneConfig()
		list := append([]*client.ControlServer(nil), cfg.ControlServers...)
		found := false
		for i, cs := range list {
			if cs != nil && cs.ID == s.ID {
				cp := *s
				list[i] = &cp
				found = true
				break
			}
		}
		if !found {
			cp := *s
			list = append(list, &cp)
		}
		cfg.ControlServers = list
		if a.saveConfig(cfg) {
			a.loginServerID = s.ID
			a.rebuild()
		}
	}, a.win)
	d.Resize(fyne.NewSize(420, 360))
	d.Show()
}
