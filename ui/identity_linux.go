//go:build linux

package ui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-gl/glfw/v3.4/glfw"
	"golang.org/x/sys/unix"
)

func setLinuxWindowIdentity() {
	glfw.WindowHintString(glfw.WaylandAppID, appID)
	glfw.WindowHintString(glfw.X11ClassName, appID)
	glfw.WindowHintString(glfw.X11InstanceName, appID)
}

func registerLinuxDesktop(icon []byte) {
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = resolved
	}
	entry := []byte(linuxDesktopEntry(execPath))

	if os.Geteuid() == 0 {
		uid, gid, home, err := elevatedSessionOwner()
		if err != nil {
			return
		}
		_ = installOwnedFile(home, uid, gid, []string{".local", "share", "applications"}, appID+".desktop", entry, 0o644)
		if len(icon) > 0 {
			_ = installOwnedFile(home, uid, gid, []string{".local", "share", "icons", "hicolor", "512x512", "apps"}, appID+".png", icon, 0o644)
		}
		return
	}

	dataHome := sessionDataHome()
	if dataHome == "" {
		return
	}
	apps := filepath.Join(dataHome, "applications")
	icons := filepath.Join(dataHome, "icons", "hicolor", "512x512", "apps")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		return
	}
	if err := os.MkdirAll(icons, 0o755); err != nil {
		return
	}
	_ = writeFileIfChanged(filepath.Join(apps, appID+".desktop"), entry, 0o644)
	if len(icon) > 0 {
		_ = writeFileIfChanged(filepath.Join(icons, appID+".png"), icon, 0o644)
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
		"Icon="+appID,
		"Terminal=false",
		"Categories=Network;Security;",
		"StartupWMClass="+appID,
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
	if os.Geteuid() == 0 {
		return ""
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	home := sessionHome()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share")
}

func sessionHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func elevatedSessionOwner() (uid, gid int, home string, err error) {
	var pw *user.User
	if name := os.Getenv("SUDO_USER"); name != "" && name != "root" {
		pw, err = user.Lookup(name)
	} else if id := os.Getenv("PKEXEC_UID"); id != "" && id != "0" {
		pw, err = user.LookupId(id)
	} else {
		err = fmt.Errorf("no session user")
	}
	if err != nil {
		return 0, 0, "", err
	}
	uid64, err := strconv.ParseInt(pw.Uid, 10, 32)
	if err != nil {
		return 0, 0, "", err
	}
	gid64, err := strconv.ParseInt(pw.Gid, 10, 32)
	if err != nil {
		return 0, 0, "", err
	}
	if uid64 <= 0 || pw.HomeDir == "" {
		return 0, 0, "", fmt.Errorf("refusing session user %s", pw.Username)
	}
	return int(uid64), int(gid64), pw.HomeDir, nil
}

func installOwnedFile(home string, uid, gid int, parts []string, name string, data []byte, mode os.FileMode) error {
	if err := validPathPart(name); err != nil {
		return err
	}
	dir, err := walkOwnedDirs(home, uid, gid, parts)
	if err != nil {
		return err
	}
	defer unix.Close(dir)
	return writeOwnedFile(dir, name, data, mode, uid, gid)
}

func walkOwnedDirs(home string, uid, gid int, parts []string) (int, error) {
	fd, err := openOwnedHome(home, uid)
	if err != nil {
		return -1, err
	}
	for _, part := range parts {
		if err := validPathPart(part); err != nil {
			unix.Close(fd)
			return -1, err
		}
		next, err := openOrCreateOwnedDir(fd, part, uid, gid)
		unix.Close(fd)
		if err != nil {
			return -1, err
		}
		fd = next
	}
	return fd, nil
}

func validPathPart(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '/') || strings.ContainsRune(name, 0) {
		return fmt.Errorf("refusing path component %q", name)
	}
	return nil
}

func openOwnedHome(home string, uid int) (int, error) {
	home = filepath.Clean(home)
	if !filepath.IsAbs(home) || home == "/" {
		return -1, fmt.Errorf("refusing session home")
	}
	fd, err := unix.Open(home, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	return fdAfterOwnerCheck(fd, uid, home)
}

func openOrCreateOwnedDir(parent int, name string, uid, gid int) (int, error) {
	fd, err := unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err == nil {
		return fdAfterOwnerCheck(fd, uid, name)
	}
	if !errors.Is(err, unix.ENOENT) {
		return -1, err
	}
	created := false
	if mkErr := unix.Mkdirat(parent, name, 0o755); mkErr == nil {
		created = true
	} else if !errors.Is(mkErr, unix.EEXIST) {
		return -1, mkErr
	}
	fd, err = unix.Openat(parent, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		unix.Close(fd)
		return -1, err
	}
	if int(st.Uid) == uid {
		return fd, nil
	}
	if !created || int(st.Uid) != os.Geteuid() {
		unix.Close(fd)
		return -1, fmt.Errorf("refusing directory %s", name)
	}
	empty, err := dirFDEmpty(fd)
	if err != nil || !empty {
		unix.Close(fd)
		if err == nil {
			err = fmt.Errorf("refusing directory %s", name)
		}
		return -1, err
	}
	if err := unix.Fchown(fd, uid, gid); err != nil {
		unix.Close(fd)
		_ = unix.Unlinkat(parent, name, unix.AT_REMOVEDIR)
		return -1, err
	}
	if err := unix.Fchmod(fd, 0o755); err != nil {
		unix.Close(fd)
		return -1, err
	}
	return fd, nil
}

func dirFDEmpty(fd int) (bool, error) {
	dup, err := unix.Dup(fd)
	if err != nil {
		return false, err
	}
	f := os.NewFile(uintptr(dup), "")
	defer f.Close()
	names, err := f.Readdirnames(1)
	if err != nil && err != io.EOF {
		return false, err
	}
	return len(names) == 0, nil
}

func fdAfterOwnerCheck(fd int, uid int, name string) (int, error) {
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		unix.Close(fd)
		return -1, err
	}
	if int(st.Uid) != uid {
		unix.Close(fd)
		return -1, fmt.Errorf("refusing directory %s", name)
	}
	return fd, nil
}

func writeOwnedFile(dirfd int, name string, data []byte, mode os.FileMode, uid, gid int) error {
	fd, err := unix.Openat(dirfd, name, unix.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) {
		fd, err = unix.Openat(dirfd, name, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, uint32(mode.Perm()))
		if err != nil {
			return err
		}
		f := os.NewFile(uintptr(fd), name)
		defer f.Close()
		if err := f.Chown(uid, gid); err != nil {
			_ = unix.Unlinkat(dirfd, name, 0)
			return err
		}
		if err := f.Chmod(mode.Perm()); err != nil {
			return err
		}
		n, err := f.Write(data)
		if err != nil {
			return err
		}
		if n != len(data) {
			return io.ErrShortWrite
		}
		return nil
	}
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG || int(st.Uid) != uid {
		return fmt.Errorf("refusing file %s", name)
	}
	if st.Size > 1<<20 {
		return fmt.Errorf("refusing file %s", name)
	}
	existing := make([]byte, st.Size)
	if _, err := io.ReadFull(f, existing); err != nil {
		return err
	}
	if bytes.Equal(existing, data) {
		return nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	n, err := f.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
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
