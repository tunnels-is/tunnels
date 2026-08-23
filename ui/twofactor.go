package ui

import (
	"encoding/json"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) twoFactorPage() fyne.CanvasObject {
	if a.user == nil {
		return pageShell("Two-factor authentication", "", nil,
			emptyState("Not signed in", "Sign in to configure two-factor authentication."))
	}

	back := outlineBtn("Back to account", func() { a.show(pageAccount) }).withIcon(theme.NavigateBackIcon())

	status := text("Loading QR code…", fsSmall, pal().Muted, false)
	qrBox := container.NewCenter(status)
	pass := kPassword("Your account password")
	digits := kEntry("123456", "")
	recovery := kEntry("Existing recovery code", "")

	codesBox := container.NewStack()

	var qrValue string
	go func() {
		qr, err := client.GetQRCode(&client.TWO_FACTOR_CONFIRM{Email: a.user.Email})
		a.uiDo(func() {
			if err != nil {
				status.Text = err.Error()
				status.Color = pal().Error
				status.Refresh()
				return
			}
			qrValue = qr.Value
			qrBox.Objects = []fyne.CanvasObject{qrImage(qr.Value, int(z(190)))}
			qrBox.Refresh()
		})
	}()

	confirm := primaryBtn("Enable two-factor", func() {
		if pass.Text == "" {
			a.fail("Please enter your password")
			return
		}
		if len(digits.Text) != 6 {
			a.fail("The authenticator code must be 6 digits")
			return
		}
		secret := parseOTPSecret(qrValue)
		if secret == "" {
			a.fail("Could not read the authenticator secret")
			return
		}
		body := map[string]any{
			"Password": pass.Text,
			"Digits":   digits.Text,
			"Recovery": recovery.Text,
			"Code":     secret,
		}
		go func() {
			raw, _, err := a.callController("/client/user/2fa/confirm", body, true)
			a.uiDo(func() {
				if err != nil {
					a.fail(err.Error())
					return
				}
				var resp struct{ Data string }
				_ = json.Unmarshal(raw, &resp)
				a.note("Two-factor authentication enabled")
				codesBox.Objects = []fyne.CanvasObject{
					card("Recovery codes",
						"Store these away from your password manager's password entry. Each code works once.",
						vstack(sp3,
							notice("Do not store these codes alongside your password.", toneWarning),
							codeBlock(string(raw), 6),
							hstack(sp2, a.copyBtn(string(raw))),
						)),
				}
				codesBox.Refresh()
			})
		}()
	})

	setup := card("Scan the code",
		"Add the code to your authenticator app, then confirm with a generated code.",
		capWidth(formWidth, vstack(sp5,
			qrBox,
			vstack(sp3,
				field("Account password", pass),
				field("Authenticator code", digits),
				fieldWith("Recovery code", "Only needed when replacing existing two-factor setup.", recovery),
				vspace(sp1),
				hstack(0, confirm),
			),
		)))

	return pageShell("Two-factor authentication", a.user.Email, back,
		scrollBody(setup, codesBox))
}

func parseOTPSecret(uri string) string {
	// otpauth://...&secret=XXXX or secret=XXXX&...
	for _, part := range strings.Split(uri, "&") {
		if i := strings.Index(strings.ToLower(part), "secret="); i >= 0 {
			return part[i+len("secret="):]
		}
	}
	if i := strings.Index(uri, "secret="); i >= 0 {
		rest := uri[i+len("secret="):]
		if j := strings.IndexAny(rest, "&"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return ""
}
