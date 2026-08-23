package ui

import (
	"encoding/json"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/tunnels-is/tunnels/client"
)

func (a *App) twoFactorPage() fyne.CanvasObject {
	if a.user == nil {
		return container.NewCenter(muted("Not logged in"))
	}

	status := muted("Loading QR code...")
	qrBox := container.NewCenter(status)
	pass := widget.NewPasswordEntry()
	pass.SetPlaceHolder("Your account password")
	digits := widget.NewEntry()
	digits.SetPlaceHolder("6-digit code")
	recovery := widget.NewEntry()
	recovery.SetPlaceHolder("Recovery code")
	codes := widget.NewLabel("")
	codes.Wrapping = fyne.TextWrapWord
	codes.Hide()

	var qrValue string
	go func() {
		qr, err := client.GetQRCode(&client.TWO_FACTOR_CONFIRM{Email: a.user.Email})
		a.uiDo(func() {
			if err != nil {
				status.SetText(err.Error())
				return
			}
			qrValue = qr.Value
			qrBox.Objects = []fyne.CanvasObject{qrImage(qr.Value, 220)}
			qrBox.Refresh()
		})
	}()

	confirm := primaryBtn("Confirm", func() {
		if pass.Text == "" {
			a.fail("Please enter your password")
			return
		}
		if len(digits.Text) != 6 {
			a.fail("Authenticator code must be 6 digits")
			return
		}
		secret := parseOTPSecret(qrValue)
		if secret == "" {
			a.fail("Could not parse authenticator secret")
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
				if resp.Data == "" {
					_ = json.Unmarshal(raw, &map[string]any{})
				}
				codes.SetText("DO NOT STORE THESE CODES WITH YOUR PASSWORD\n\n" + string(raw))
				codes.Show()
			})
		}()
	})
	return pageScroll(
		ghostBtn("Back", func() { a.show(pageAccount) }),
		card("Two-factor authentication", "Scan the QR code with your authenticator app, then confirm.", container.NewVBox(
			qrBox,
			labeled("Password", pass),
			labeled("Authenticator code", digits),
			confirm,
			widget.NewSeparator(),
			muted("Have a recovery code? Enter it below to replace existing 2FA."),
			labeled("Recovery code", recovery),
			codes,
		)),
	)
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
