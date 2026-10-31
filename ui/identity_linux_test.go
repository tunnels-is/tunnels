//go:build linux

package ui

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// TestLinuxDesktopEntryIdentifiesApp ...
func TestLinuxDesktopEntryIdentifiesApp(t *testing.T) {
	got := linuxDesktopEntry("/opt/tunnels/tunnels-app")
	for _, want := range []string{
		"Name=Tunnels",
		`Exec="/opt/tunnels/tunnels-app"`,
		"Icon=" + appID,
		"StartupWMClass=" + appID,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("desktop entry missing %q\n%s", want, got)
		}
	}
	if strings.Contains(got, "StartupWMClass=Tunnels\n") {
		t.Fatal("StartupWMClass must be the Wayland app_id, not the window title")
	}
}

// TestDesktopQuoteEscapes ...
func TestDesktopQuoteEscapes(t *testing.T) {
	got := desktopQuote(`/tmp/foo"bar`)
	if got != `"/tmp/foo\"bar"` {
		t.Fatalf("got %s", got)
	}
}

func TestInstallOwnedFileWritesAndKeepsOwner(t *testing.T) {
	home := t.TempDir()
	uid, gid := os.Getuid(), os.Getgid()
	parts := []string{".local", "share", "applications"}
	name := appID + ".desktop"
	if err := installOwnedFile(home, uid, gid, parts, name, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(append([]string{home}, append(parts, name)...)...)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one" {
		t.Fatalf("content = %q", got)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != uid {
		t.Fatalf("owner = %#v", info.Sys())
	}
	if err := installOwnedFile(home, uid, gid, parts, name, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := installOwnedFile(home, uid, gid, parts, name, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two" {
		t.Fatalf("updated content = %q", got)
	}
}

func TestInstallOwnedFileRejectsDirectorySymlink(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(home, ".local")); err != nil {
		t.Fatal(err)
	}
	err := installOwnedFile(home, os.Getuid(), os.Getgid(), []string{".local", "share", "applications"}, appID+".desktop", []byte("x"), 0o644)
	if err == nil {
		t.Fatal("directory symlink was accepted")
	}
	if _, err := os.Lstat(filepath.Join(outside, "share")); !os.IsNotExist(err) {
		t.Fatal("followed the directory symlink")
	}
}

func TestInstallOwnedFileRejectsFileSymlink(t *testing.T) {
	home := t.TempDir()
	secretDir := t.TempDir()
	secret := filepath.Join(secretDir, "secret")
	if err := os.WriteFile(secret, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	apps := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(apps, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(apps, appID+".desktop")); err != nil {
		t.Fatal(err)
	}
	err := installOwnedFile(home, os.Getuid(), os.Getgid(), []string{".local", "share", "applications"}, appID+".desktop", []byte("replaced"), 0o644)
	if err == nil {
		t.Fatal("file symlink was accepted")
	}
	got, err := os.ReadFile(secret)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("symlink target changed to %q", got)
	}
}

func TestInstallOwnedFileRejectsHomeSymlink(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	err := installOwnedFile(link, os.Getuid(), os.Getgid(), []string{".local"}, "f.desktop", []byte("x"), 0o644)
	if err == nil {
		t.Fatal("home symlink was accepted")
	}
	if _, err := os.Lstat(filepath.Join(real, ".local")); !os.IsNotExist(err) {
		t.Fatal("followed the home symlink")
	}
}

func TestInstallOwnedFileRejectsDotDot(t *testing.T) {
	home := t.TempDir()
	err := installOwnedFile(home, os.Getuid(), os.Getgid(), []string{"..", "tunnels-escape-probe"}, "f.desktop", []byte("x"), 0o644)
	if err == nil {
		t.Fatal("dotdot was accepted")
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(home), "tunnels-escape-probe")); !os.IsNotExist(err) {
		t.Fatal("dotdot left the home directory")
	}
}

func TestInstallOwnedFileRejectsForeignHome(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root owns the probe directory")
	}
	err := installOwnedFile("/usr", os.Getuid(), os.Getgid(), []string{"tunnels-owned-probe"}, "f.desktop", []byte("x"), 0o644)
	if err == nil {
		t.Fatal("foreign home was accepted")
	}
	if _, err := os.Lstat("/usr/tunnels-owned-probe"); !os.IsNotExist(err) {
		t.Fatal("created a directory outside the session home")
	}
}

func TestElevatedSessionOwnerIgnoresRoot(t *testing.T) {
	t.Setenv("SUDO_USER", "root")
	t.Setenv("PKEXEC_UID", "0")
	if _, _, _, err := elevatedSessionOwner(); err == nil {
		t.Fatal("root session was accepted")
	}
}

func TestElevatedSessionOwnerUsesSudoUser(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if u.Uid == "0" || u.Username == "" || u.Username == "root" {
		t.Skip("current user is root")
	}
	t.Setenv("SUDO_USER", u.Username)
	t.Setenv("PKEXEC_UID", "")
	uid, _, home, err := elevatedSessionOwner()
	if err != nil {
		t.Fatal(err)
	}
	if home != u.HomeDir {
		t.Fatalf("home = %s, want %s", home, u.HomeDir)
	}
	want, err := strconv.Atoi(u.Uid)
	if err != nil {
		t.Fatal(err)
	}
	if want != uid {
		t.Fatalf("uid = %d, user = %s", uid, u.Uid)
	}
}
