package ui

import (
	"encoding/json"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/client"
)

// Login modes. Kept as the original integers because they map to controller
// endpoints and preferences already stored on disk.
const (
	modeLogin     = 1
	modeRegister  = 2
	modeRecovery  = 3
	modeReset     = 4
	modeAnonymous = 5
	modeEnable    = 6
)

var loginTitles = map[int]struct{ title, sub string }{
	modeLogin:     {"Welcome back", "Sign in to your Tunnels account."},
	modeRegister:  {"Create account", "You only need an email and a password."},
	modeRecovery:  {"Two-factor recovery", "Sign in with a recovery code instead of your authenticator."},
	modeReset:     {"Reset password", "Request a code, then choose a new password."},
	modeAnonymous: {"Anonymous account", "No email. Keep the token safe — it is the only way back in."},
	modeEnable:    {"Enable account", "Confirm the code we emailed you."},
}

func (a *App) loginPage() fyne.CanvasObject {
	pref := a.fyneApp.Preferences()
	mode := a.loginMode
	if mode == 0 {
		mode = modeLogin
	}

	email := kEntry("you@example.com", pref.String("default-email"))
	pass := kPassword("••••••••••")
	pass2 := kPassword("Repeat password")
	device := kEntry("e.g. workstation", pref.String("default-device-name"))
	digits := kEntry("123456", "")
	recovery := kEntry("Recovery code", "")
	code := kEntry("Code", "")

	errBox := container.NewStack()
	setErr := func(msg string) {
		if msg == "" {
			errBox.Objects = nil
		} else {
			errBox.Objects = []fyne.CanvasObject{notice(msg, toneDanger)}
		}
		errBox.Refresh()
	}

	remember := newSwitch(a.loginRemember, func(v bool) { a.loginRemember = v })

	// Control server picker.
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
	serverSel.PlaceHolder = "No server configured"
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

	showToken := mode == modeAnonymous || (mode == modeRegister && a.loginToken)
	showEmail := (mode == modeLogin || mode == modeRegister || mode == modeReset || mode == modeEnable) && !showToken
	showPass := mode != modeEnable
	showPass2 := mode == modeRegister || mode == modeReset || mode == modeAnonymous

	fields := []fyne.CanvasObject{}
	if showToken {
		email.SetPlaceHolder("Token")
		fields = append(fields, fieldWith("Login token", "Store this somewhere safe before continuing.", email))
	}
	if showEmail {
		fields = append(fields, field("Email", email))
	}
	if mode == modeLogin {
		fields = append(fields, field("Device name", device))
	}
	if showPass {
		fields = append(fields, field("Password", pass))
	}
	if showPass2 {
		fields = append(fields, field("Confirm password", pass2))
	}
	if mode == modeLogin {
		fields = append(fields, fieldWith("Two-factor code", "Leave empty if 2FA is off.", digits))
	}
	if mode == modeRecovery {
		fields = append(fields, field("Recovery code", recovery))
	}
	if mode == modeEnable || mode == modeReset {
		lbl := "Confirmation code"
		if mode == modeReset {
			lbl = "Reset code"
		}
		fields = append(fields, field(lbl, code))
	}

	submitLabel := map[int]string{
		modeLogin: "Sign in", modeRegister: "Create account", modeRecovery: "Sign in",
		modeReset: "Set new password", modeAnonymous: "Create account", modeEnable: "Enable account",
	}[mode]

	submit := func() {
		setErr("")
		srv := activeServer()
		if srv == nil {
			setErr("No control server configured")
			return
		}
		switch mode {
		case modeLogin, modeRecovery:
			if strings.TrimSpace(email.Text) == "" || pass.Text == "" {
				setErr("Email and password are required")
				return
			}
			if mode == modeLogin && strings.TrimSpace(device.Text) == "" {
				setErr("Device name is required")
				return
			}
			if mode == modeRecovery && strings.TrimSpace(recovery.Text) == "" {
				setErr("Recovery code is required")
				return
			}
			body := map[string]any{
				"Email":      email.Text,
				"Password":   pass.Text,
				"DeviceName": device.Text,
				"Digits":     digits.Text,
				"Recovery":   recovery.Text,
			}
			a.note("Signing in…")
			go func() {
				raw, _, err := a.callControllerOn(srv, "/client/user/login", body, false)
				a.uiDo(func() {
					if err != nil {
						setErr(err.Error())
						return
					}
					u := new(client.User)
					if err := json.Unmarshal(raw, u); err != nil {
						setErr("Unable to parse login response")
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
		case modeRegister, modeAnonymous:
			if len(pass.Text) < 10 {
				setErr("Password must be at least 10 characters")
				return
			}
			if pass.Text != pass2.Text {
				setErr("Passwords do not match")
				return
			}
			body := map[string]any{
				"Email":     email.Text,
				"Password":  pass.Text,
				"Password2": pass2.Text,
			}
			a.note("Creating account…")
			go func() {
				raw, _, err := a.callControllerOn(srv, "/client/user/create", body, false)
				a.uiDo(func() {
					if err != nil {
						setErr(err.Error())
						return
					}
					u := new(client.User)
					if err := json.Unmarshal(raw, u); err != nil {
						setErr("Unable to parse register response")
						return
					}
					u.ControlServer = srv
					a.setUser(u)
					a.note("Account created")
					a.show(pageServers)
				})
			}()
		case modeReset:
			if len(pass.Text) < 9 || pass.Text != pass2.Text || code.Text == "" {
				setErr("Fill in the password, confirmation and reset code")
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
						setErr(err.Error())
						return
					}
					a.loginMode = modeLogin
					a.note("Password reset")
					a.rebuild()
				})
			}()
		case modeEnable:
			body := map[string]any{"Email": email.Text, "Code": code.Text, "ConfirmCode": code.Text}
			go func() {
				_, _, err := a.callControllerOn(srv, "/client/user/enable", body, false)
				a.uiDo(func() {
					if err != nil {
						setErr(err.Error())
						return
					}
					a.loginMode = modeLogin
					a.note("Account enabled")
					a.rebuild()
				})
			}()
		}
	}
	email.OnSubmitted = func(string) { submit() }
	pass.OnSubmitted = func(string) { submit() }

	setMode := func(m int) {
		if m == modeAnonymous {
			a.loginToken = true
			email.SetText(uuid.New().String())
		} else {
			a.loginToken = false
		}
		a.loginMode = m
		a.rebuild()
	}

	// Primary switch between the two everyday flows; everything else is a link.
	tabs := newSegmented([]segItem{
		{"login", "Sign in"},
		{"register", "Register"},
	}, map[bool]string{true: "register", false: "login"}[mode == modeRegister],
		func(key string) {
			if key == "register" {
				setMode(modeRegister)
			} else {
				setMode(modeLogin)
			}
		})

	body := []fyne.CanvasObject{}
	if mode == modeLogin || mode == modeRegister {
		body = append(body, container.NewCenter(tabs), vspace(sp1))
	}
	body = append(body, vstack(sp3, fields...))
	body = append(body, errBox)

	actions := []fyne.CanvasObject{primaryBtn(submitLabel, submit)}
	if mode == modeReset {
		actions = append(actions, outlineBtn("Email me a code", func() {
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
	actionRow := fyne.CanvasObject(hstack(sp2, actions...))
	if mode == modeLogin {
		actionRow = splitRow(hstack(sp2, actions...),
			hstack(sp2, text("Remember me", fsSmall, pal().Muted, false), remember))
	}
	body = append(body, actionRow)

	// Secondary flows.
	var links []fyne.CanvasObject
	switch mode {
	case modeLogin:
		links = []fyne.CanvasObject{
			ghostBtn("Forgot password", func() { setMode(modeReset) }).small(),
			ghostBtn("Recovery code", func() { setMode(modeRecovery) }).small(),
			ghostBtn("Anonymous", func() { setMode(modeAnonymous) }).small(),
			ghostBtn("Enable account", func() { setMode(modeEnable) }).small(),
		}
	case modeRegister:
		links = []fyne.CanvasObject{
			ghostBtn("Anonymous instead", func() { setMode(modeAnonymous) }).small(),
		}
	default:
		links = []fyne.CanvasObject{
			ghostBtn("Back to sign in", func() { setMode(modeLogin) }).small(),
		}
	}

	titles := loginTitles[mode]
	brand := container.NewCenter(vstack(sp2,
		container.NewCenter(brandMark(z(40))),
		container.NewCenter(text(titles.title, fsTitle, pal().Content, true)),
		container.NewCenter(text(titles.sub, fsSmall, pal().Muted, false)),
	))

	editServer := newIconBtn(theme.DocumentCreateIcon(), kGhost, func() {
		s := activeServer()
		if s == nil {
			s = &client.ControlServer{ID: uuid.New().String(), ValidateCertificate: true}
		}
		a.editAuthServer(s)
	}).small()
	addServer := newIconBtn(theme.ContentAddIcon(), kGhost, func() {
		a.editAuthServer(&client.ControlServer{ID: uuid.New().String(), ValidateCertificate: true})
	}).small()

	serverCard := card("", "", splitRow(
		vstack(2, text("Control server", fsSmall, pal().Muted, false), fixedWidth(z(230), serverSel)),
		hstack(sp1, addServer, editServer),
	))

	return centredBody(z(400),
		vspace(sp6),
		brand,
		card("", "", vstack(sp3, body...)),
		container.NewCenter(hstack(sp1, links...)),
		serverCard,
		vspace(sp6),
	)
}

func (a *App) editAuthServer(s *client.ControlServer) {
	host := kEntry("api.tunnels.is", s.Host)
	port := kEntry("443", s.Port)
	cert := kEntry("optional", s.CertificatePath)
	validate := bindCheck("Validate TLS certificate", s.ValidateCertificate, nil)

	form := container.New(fixedLayout{w: z(380)}, vstack(sp3,
		formPair(field("Host", host), field("Port", port)),
		field("Certificate path", cert),
		validate,
		notice("Turning verification off lets a man-in-the-middle read your login credentials. Only do this for a trusted self-signed server.", toneWarning),
	))

	d := dialog.NewCustomConfirm("Control server", "Save", "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		s.Host = strings.TrimSpace(host.Text)
		s.Port = strings.TrimSpace(port.Text)
		s.CertificatePath = strings.TrimSpace(cert.Text)
		s.ValidateCertificate = validate.Checked
		a.updateConfig("Saving control server", func(c *client.Config) {
			list := append([]*client.ControlServer(nil), c.ControlServers...)
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
			c.ControlServers = list
		}, func() {
			a.loginServerID = s.ID
			a.note("Control server saved")
			a.rebuild()
		})
	}, a.win)
	d.Resize(fyne.NewSize(z(440), z(400)))
	d.Show()
}
