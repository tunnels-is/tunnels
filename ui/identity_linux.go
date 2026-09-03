//go:build linux

package ui

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/go-gl/glfw/v3.4/glfw"
)

func setLinuxWindowIdentity() {
	glfw.WindowHintString(glfw.WaylandAppID, linuxAppID)
	glfw.WindowHintString(glfw.X11ClassName, linuxAppID)
	glfw.WindowHintString(glfw.X11InstanceName, linuxAppID)
}

func registerLinuxDesktop(icon []byte) {
	dataHome := sessionDataHome()
	if dataHome == "" {
		return
	}
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}

	apps := filepath.Join(dataHome, "applications")
	icons := filepath.Join(dataHome, "icons", "hicolor", "512x512", "apps")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		return
	}
	if err := os.MkdirAll(icons, 0o755); err != nil {
		return
	}

	desktopPath := filepath.Join(apps, linuxAppID+".desktop")
	_ = writeFileIfChanged(desktopPath, []byte(linuxDesktopEntry(execPath)), 0o644)
	if len(icon) > 0 {
		_ = writeFileIfChanged(filepath.Join(icons, linuxAppID+".png"), icon, 0o644)
	}
}

func linuxDesktopEntry(execPath string) string {
	lines := []string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=" + appName,
		"Comment=Tunnels VPN client",
		"Exec=" + desktopQuote(execPath),
	}
	if !strings.ContainsAny(execPath, " \t") {
		lines = append(lines, "TryExec="+execPath)
	}
	lines = append(lines,
		"Icon="+linuxAppID,
		"Terminal=false",
		"Categories=Network;Security;",
		"StartupWMClass="+linuxAppID,
		"StartupNotify=true",
		"Keywords=vpn;wireguard;tunnels;",
		"",
	)
	return strings.Join(lines, "\n")
}

func desktopQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func sessionDataHome() string {
	home := sessionHome()
	if home == "" {
		return ""
	}
	if os.Geteuid() != 0 {
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return d
		}
	}
	return filepath.Join(home, ".local", "share")
}

func sessionHome() string {
	if os.Geteuid() == 0 {
		if u := os.Getenv("SUDO_USER"); u != "" && u != "root" {
			if pw, err := user.Lookup(u); err == nil && pw.HomeDir != "" {
				return pw.HomeDir
			}
		}
		if uid := os.Getenv("PKEXEC_UID"); uid != "" && uid != "0" {
			if pw, err := user.LookupId(uid); err == nil && pw.HomeDir != "" {
				return pw.HomeDir
			}
		}
		return ""
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func writeFileIfChanged(path string, data []byte, mode os.FileMode) error {
	if prev, err := os.ReadFile(path); err == nil && string(prev) == string(data) {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install %s: %w", path, err)
	}
	return nil
}
