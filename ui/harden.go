package ui

import (
	"fmt"
	"os"
	"runtime/debug"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/tunnels-is/tunnels/client"
)

// fyneDisableDPIEnv is Fyne's switch to skip glfw.GetVideoMode() while
// resolving window scale. During a macOS display reconfigure that call
// returns nil and Fyne panics on the main thread (see getMonitorScale).
const fyneDisableDPIEnv = "FYNE_DISABLE_DPI_DETECTION"

// hardenFyne must run before app.NewWithID / NewWindow. It does not
// override a value the user already set.
func hardenFyne() {
	if os.Getenv(fyneDisableDPIEnv) == "" {
		_ = os.Setenv(fyneDisableDPIEnv, "1")
	}
}

func recoverRun(where string) {
	rec := recover()
	if rec == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "tunnels ui panic in %s: %v\n%s\n", where, rec, debug.Stack())
}

func (a *App) shutdown() {
	if client.CancelFunc != nil {
		client.CancelFunc()
	}
	client.ResetEverything()
	if a.fyneApp != nil {
		a.fyneApp.Quit()
	}
}

// surviveWindowClose keeps the Fyne run loop up when GLFW delivers a close
// because the display went away. App.Quit() still destroys the window
// (Close bypasses the intercept).
func (a *App) surviveWindowClose() {
	if a.win == nil {
		return
	}
	a.win.SetCloseIntercept(func() {
		a.win.Hide()
	})
	if life := a.fyneApp.Lifecycle(); life != nil {
		life.SetOnEnteredForeground(func() {
			if a.win != nil {
				a.win.Show()
			}
		})
	}
}

func (a *App) installQuitMenu() {
	if a.win == nil {
		return
	}
	quit := fyne.NewMenuItem("Quit", a.shutdown)
	quit.IsQuit = true
	a.win.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("File", quit)))
	if a.win.Canvas() == nil {
		return
	}
	a.win.Canvas().AddShortcut(
		&desktop.CustomShortcut{KeyName: fyne.KeyQ, Modifier: fyne.KeyModifierShortcutDefault},
		func(fyne.Shortcut) { a.shutdown() },
	)
}

func (a *App) keepDisplayAwake() {
	if a.fyneApp == nil {
		return
	}
	if d := a.fyneApp.Driver(); d != nil {
		d.SetDisableScreenBlanking(true)
	}
}
