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

func bindCheck(label string, value bool, on func(bool)) *widget.Check {
	c := widget.NewCheck(label, nil)
	c.SetChecked(value)
	c.OnChanged = on
	return c
}

func labeled(label string, obj fyne.CanvasObject) fyne.CanvasObject {
	return kLabeled(label, obj)
}

func labeledEntry(label, placeholder string, password bool) (*widget.Entry, fyne.CanvasObject) {
	e := widget.NewEntry()
	if password {
		e = widget.NewPasswordEntry()
	}
	e.SetPlaceHolder(placeholder)
	cap := widget.NewLabel(label)
	cap.TextStyle = fyne.TextStyle{Bold: true}
	return e, container.NewVBox(cap, e)
}

func infoRow(label, value string) fyne.CanvasObject {
	return kInfo(label, value)
}

func card(title, subtitle string, content fyne.CanvasObject) fyne.CanvasObject {
	return cardBox(title, subtitle, nil, content)
}

func wrapLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

func muted(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Importance = widget.LowImportance
	l.Wrapping = fyne.TextWrapWord
	return l
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
	return img
}

func toolbar(left fyne.CanvasObject, right ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, left, container.NewHBox(right...))
}

func pageScroll(objs ...fyne.CanvasObject) fyne.CanvasObject {
	col := make([]fyne.CanvasObject, 0, len(objs)*2+2)
	col = append(col, vspace(8))
	for i, o := range objs {
		if i > 0 {
			col = append(col, vspace(12))
		}
		col = append(col, o)
	}
	col = append(col, vspace(24))
	return container.NewBorder(nil, nil, hspace(8), hspace(16),
		container.NewVScroll(container.NewVBox(col...)))
}

func copyBtn(a *App, text string) *widget.Button {
	return widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		a.win.Clipboard().SetContent(text)
		a.note("Copied")
	})
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
