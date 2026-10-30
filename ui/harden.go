package ui

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/go-gl/glfw/v3.4/glfw"
	"github.com/tunnels-is/tunnels/client"
)

const fyneDisableDPIEnv = "FYNE_DISABLE_DPI_DETECTION"

const fynePlatformEnv = "FYNE_PLATFORM"

func hardenFyne() {
	if runtime.GOOS == "darwin" {
		if os.Getenv(fyneDisableDPIEnv) == "" {
			_ = os.Setenv(fyneDisableDPIEnv, "1")
		}
		return
	}
	preferWayland()
	preferCompositorDecorations()
}

func preferWayland() {
	if runtime.GOOS != "linux" {
		return
	}
	if os.Getenv(fynePlatformEnv) != "" {
		return
	}
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return
	}
	_ = os.Setenv(fynePlatformEnv, "wayland")
}

func preferCompositorDecorations() {
	if !shouldDisableLibdecor() {
		return
	}
	glfw.InitHint(glfw.WaylandLibdecor, glfw.WaylandDisableLibdecor)
}

func shouldDisableLibdecor() bool {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}
	return !gnomeNeedsLibdecor(os.Getenv("XDG_CURRENT_DESKTOP"))
}

func gnomeNeedsLibdecor(desktop string) bool {
	u := strings.ToUpper(desktop)
	if !strings.Contains(u, "GNOME") {
		return false
	}
	if strings.Contains(u, "COSMIC") {
		return false
	}
	return true
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
	if runtime.GOOS == "darwin" {
		quit := fyne.NewMenuItem("Quit", a.shutdown)
		quit.IsQuit = true
		a.win.SetMainMenu(fyne.NewMainMenu(fyne.NewMenu("File", quit)))
	}
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
