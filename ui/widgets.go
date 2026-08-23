package ui

import (
	"bytes"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/skip2/go-qrcode"
)

// bindCheck is kept for dialog forms, where a labelled checkbox reads better
// than a bare switch. Page settings use toggleRow instead.
func bindCheck(label string, value bool, on func(bool)) *widget.Check {
	c := widget.NewCheck(label, nil)
	c.SetChecked(value)
	c.OnChanged = on
	return c
}

func muted(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Importance = widget.LowImportance
	l.Wrapping = fyne.TextWrapWord
	return l
}

// notice is an inline callout for warnings and constraints inside a page.
func notice(msg string, t tone) fyne.CanvasObject {
	fg, bg := toneColors(t)
	box := surface(radMd, bg, withAlpha(fg, 70))
	lbl := rich(msg, sizeSmall, colContent, false)
	return container.NewStack(box, insetXY(sp3, sp2+2, lbl))
}

func (a *App) confirm(title, msg string, fn func()) {
	dialog.ShowConfirm(title, msg, func(ok bool) {
		if ok {
			fn()
		}
	}, a.win)
}

func (a *App) openURL(raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		a.fail(err.Error())
		return
	}
	if err := a.fyneApp.OpenURL(u); err != nil {
		a.fail(err.Error())
	}
}

func qrImage(value string, px int) fyne.CanvasObject {
	if value == "" {
		return muted("No QR data")
	}
	png, err := qrcode.Encode(value, qrcode.Medium, px)
	if err != nil {
		return muted(err.Error())
	}
	img := canvas.NewImageFromReader(bytes.NewReader(png), "qr")
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(float32(px), float32(px)))
	// A white quiet zone keeps the code scannable on dark themes.
	frame := surface(radMd, hex(0xff, 0xff, 0xff), nil)
	return container.NewCenter(container.NewStack(frame, inset(sp3, img)))
}

func (a *App) copyBtn(value string) *kBtn {
	return newIconBtn(theme.ContentCopyIcon(), kGhost, func() {
		a.win.Clipboard().SetContent(value)
		a.note("Copied to clipboard")
	}).small()
}

// codeBlock shows a long secret in a scrollable monospace panel.
func codeBlock(value string, rows int) fyne.CanvasObject {
	e := widget.NewMultiLineEntry()
	e.SetText(value)
	e.TextStyle = fyne.TextStyle{Monospace: true}
	e.Wrapping = fyne.TextWrapOff
	e.SetMinRowsVisible(rows)
	return e
}

func filterMatch(q string, parts ...string) bool {
	if q == "" {
		return true
	}
	q = strings.ToLower(q)
	for _, p := range parts {
		if strings.Contains(strings.ToLower(p), q) {
			return true
		}
	}
	return false
}

func countryName(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "UK" {
		code = "GB"
	}
	if code == "" {
		return ""
	}
	if n, ok := countryNames[code]; ok {
		return n
	}
	return code
}

var countryNames = map[string]string{
	"US": "United States", "GB": "United Kingdom", "DE": "Germany", "NL": "Netherlands",
	"SE": "Sweden", "NO": "Norway", "FI": "Finland", "DK": "Denmark", "IS": "Iceland",
	"FR": "France", "ES": "Spain", "IT": "Italy", "CH": "Switzerland", "AT": "Austria",
	"BE": "Belgium", "IE": "Ireland", "PL": "Poland", "CZ": "Czechia", "PT": "Portugal",
	"CA": "Canada", "AU": "Australia", "NZ": "New Zealand", "JP": "Japan", "SG": "Singapore",
	"HK": "Hong Kong", "IN": "India", "BR": "Brazil", "MX": "Mexico", "ZA": "South Africa",
	"AE": "United Arab Emirates", "IL": "Israel", "TR": "Turkey", "RO": "Romania",
	"BG": "Bulgaria", "HU": "Hungary", "SK": "Slovakia", "LT": "Lithuania", "LV": "Latvia",
	"EE": "Estonia", "UA": "Ukraine", "KR": "South Korea", "TW": "Taiwan", "MY": "Malaysia",
}
