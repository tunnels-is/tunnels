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

// fyneDisableDPIEnv is Fyne's switch to skip glfw.GetVideoMode() while
// resolving window scale. During a macOS display reconfigure that call
// returns nil and Fyne panics on the main thread (see getMonitorScale).
const fyneDisableDPIEnv = "FYNE_DISABLE_DPI_DETECTION"

// fynePlatformEnv selects GLFW's backend. Fyne otherwise forces X11 on
// compositors without server-side decorations (GNOME), and the compositor
// then bilinear-scales that 1× XWayland buffer at 125%/150% — the blur
// that goes away when display scale is set back to 100%. Native Wayland
// gets a framebuffer at the compositor scale (texScale) instead.
const fynePlatformEnv = "FYNE_PLATFORM"

// hardenFyne must run before app.NewWithID / NewWindow. It does not
// override a value the user already set.
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

// preferCompositorDecorations drops GLFW's libdecor CSD — the GTK/cairo
// title bar that ignores Cosmic/KDE/Sway themes — when the compositor
// can draw its own decorations. Must run before fyne initializes GLFW.
//
// GNOME/Mutter has no xdg-decoration, so libdecor stays there.
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
	// COSMIC can carry leftover GNOME tokens; it advertises SSD.
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

// installQuitMenu wires Cmd/Ctrl+Q. On macOS, SetMainMenu lands in the native
// menu bar. On Linux and Windows it would paint a second in-window File bar
// on top of the system title bar, so those platforms keep the shortcut only.
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
