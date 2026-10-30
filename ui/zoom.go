package ui

import (
	"fmt"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
)

const (
	zoomMin     float32 = 0.8
	zoomMax     float32 = 2.0
	zoomStep    float32 = 0.1
	zoomDefault float32 = 1.0
	zoomPrefKey         = "ui-zoom"
)

func clampZoom(f float32) float32 {
	if f < zoomMin {
		return zoomMin
	}
	if f > zoomMax {
		return zoomMax
	}
	return float32(int(f*10+0.5)) / 10
}

func loadZoom(app fyne.App) float32 {
	f := clampZoom(float32(app.Preferences().FloatWithFallback(zoomPrefKey, float64(zoomDefault))))
	applyZoomTokens(f)
	return f
}

func (a *App) setZoom(f float32) {
	f = clampZoom(f)
	if f == a.zoom {
		return
	}
	a.zoom = f
	a.fyneApp.Preferences().SetFloat(zoomPrefKey, float64(f))
	applyZoomTokens(f)

	if s := a.fyneApp.Settings(); s != nil {
		s.SetTheme(live)
	}
	a.rebuild()
	a.refreshNav()
	a.note(fmt.Sprintf("Zoom %d%%", zoomPercent(f)))
}

func zoomPercent(f float32) int {
	return int(f*100 + 0.5)
}

func modKeyLabel() string {
	if runtime.GOOS == "darwin" {
		return "Cmd"
	}
	return "Ctrl"
}

func (a *App) registerZoomShortcuts() {
	if a.win == nil || a.win.Canvas() == nil {
		return
	}
	bind := func(key fyne.KeyName, fn func()) {
		a.win.Canvas().AddShortcut(
			&desktop.CustomShortcut{KeyName: key, Modifier: fyne.KeyModifierShortcutDefault},
			func(fyne.Shortcut) { fn() },
		)
	}
	in := func() { a.setZoom(a.zoom + zoomStep) }

	bind(fyne.KeyEqual, in)
	bind(fyne.KeyPlus, in)
	bind(fyne.KeyMinus, func() { a.setZoom(a.zoom - zoomStep) })
	bind(fyne.Key0, func() { a.setZoom(zoomDefault) })
}

func (a *App) zoomStepper() fyne.CanvasObject {
	out := newIconBtn(theme.ContentRemoveIcon(), kOutline, func() {
		a.setZoom(a.zoom - zoomStep)
	}).small()
	in := newIconBtn(theme.ContentAddIcon(), kOutline, func() {
		a.setZoom(a.zoom + zoomStep)
	}).small()
	if a.zoom <= zoomMin {
		out.Disable()
	}
	if a.zoom >= zoomMax {
		in.Disable()
	}

	value := text(fmt.Sprintf("%d%%", zoomPercent(a.zoom)), fsSmall, pal().Content, true)
	value.Alignment = fyne.TextAlignCenter
	readout := fixedWidth(z(48), value)

	items := []fyne.CanvasObject{out, readout, in}
	if a.zoom != zoomDefault {
		items = append(items, hspace(sp1), ghostBtn("Reset", func() { a.setZoom(zoomDefault) }).small())
	}
	return hstack(sp1, items...)
}
