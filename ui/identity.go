package ui

const (
	appName = "Tunnels"

	// appID is Fyne's unique id. The GLFW driver copies it onto the Wayland
	// xdg_toplevel app_id when the window is created, and the X11 class hints
	// use the same value. It must be the desktop-file name with the
	// ".desktop" suffix removed so Cosmic/GNOME/KDE can resolve Name and Icon.
	// An id of "is.tunnels.desktop" is looked up as is.tunnels.desktop.desktop
	// and Cosmic then shows the trailing word "desktop".
	appID = "is.tunnels"

	// legacyAppID is the previous Fyne id. Preferences, including theme and
	// the active account, were stored in a directory of this name.
	legacyAppID = "is.tunnels.desktop"
)
