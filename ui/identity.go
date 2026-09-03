package ui

const (
	appID   = "is.tunnels.desktop"
	appName = "Tunnels"

	// linuxAppID is the Wayland xdg_toplevel app_id and X11 WM_CLASS.
	// The xdg-shell spec requires this to match the .desktop file name with
	// the ".desktop" suffix removed, so Cosmic/GNOME/KDE can resolve Name
	// and Icon for alt-tab and the dock.
	linuxAppID = "is.tunnels"
)
