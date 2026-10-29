package setcap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
	"kernel.org/pub/linux/libs/security/libcap/cap"
)

const capabilitySet = "cap_net_raw,cap_net_bind_service,cap_net_admin+eip"

// runCommand is replaced in tests.
var runCommand = func(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func CheckCapabilities() (err error) {
	orig := cap.GetProc()
	defer orig.SetProc()

	c, err := orig.Dup()
	if err != nil {
		return fmt.Errorf("failed to get capabilities, err: %s", err)
	}

	missingFlags := false
	for _, flag := range []cap.Value{cap.NET_BIND_SERVICE, cap.NET_ADMIN, cap.NET_RAW} {
		on, _ := c.GetFlag(cap.Permitted, flag)
		if !on {
			missingFlags = true
		}
	}
	if !missingFlags {
		return nil
	}

	target, err := resolveCapabilityTarget()
	if err != nil {
		fmt.Printf("\n\033[1;31m  ✗  %s\033[0m\n\n"+
			"  Install the binary as root, in a root-owned directory, then run:\n"+
			"  \033[1;34m→\033[0m  sudo setcap '%s' <binary>\n\n", err, capabilitySet)
		return err
	}

	fmt.Print("\n" +
		"\033[1;34m╔══════════════════════════════════════════════════════════╗\n" +
		"║              Network Permissions Required                ║\n" +
		"╚══════════════════════════════════════════════════════════╝\033[0m\n" +
		"\n  Tunnels needs capabilities to manage network interfaces.\n" +
		"  This does NOT grant root/sudo access to the system.\n\n" +
		"  🔑 Password: ")

	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return err
	}
	defer clearBytes(bytePassword)

	cmd := exec.Command("sudo", "-S", "/usr/sbin/setcap", capabilitySet, target)
	cmd.Stdin = strings.NewReader(string(bytePassword) + "\n")
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	clearSudoTimestamp()
	if err != nil {
		fmt.Printf("\n\033[1;31m  ✗  Unable to set capabilities: %s\033[0m\n\n"+
			"  To grant permissions manually, run:\n"+
			"  \033[1;34m→\033[0m  sudo setcap '%s' %s\n\n", err, capabilitySet, quotedPath(target))
		os.Exit(1)
	}

	argv := append([]string{target}, os.Args[1:]...)
	if execErr := syscall.Exec(target, argv, os.Environ()); execErr != nil {
		return fmt.Errorf("re-exec %s: %w", target, execErr)
	}
	return nil
}

func clearSudoTimestamp() {
	_ = runCommand("sudo", "-kK")
}

func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func quotedPath(path string) string {
	if path == "" {
		path = "[BINARY]"
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

// resolveCapabilityTarget returns the real binary path. The file and its
// directory must be owned by root and not writable by group or other, so
// setcap cannot be pointed at a path a local user can replace.
func resolveCapabilityTarget() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve binary: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve binary: %w", err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve binary: %w", err)
	}
	if err := validateCapabilityTarget(resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func validateCapabilityTarget(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("resolve binary: %w", err)
	}
	parent := filepath.Dir(path)
	parentResolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("resolve binary directory: %w", err)
	}
	parentInfo, err := os.Lstat(parentResolved)
	if err != nil {
		return fmt.Errorf("resolve binary directory: %w", err)
	}
	return validateCapabilityInfo(info, parentInfo)
}

func validateCapabilityInfo(file, parent os.FileInfo) error {
	if !file.Mode().IsRegular() {
		return fmt.Errorf("refusing to set capabilities: target is not a regular file")
	}
	if err := rootOwnedNotWritable(file, "binary"); err != nil {
		return err
	}
	if !parent.IsDir() {
		return fmt.Errorf("refusing to set capabilities: binary directory is not a directory")
	}
	return rootOwnedNotWritable(parent, "binary directory")
}

func rootOwnedNotWritable(info os.FileInfo, what string) error {
	uid, ok := fileUID(info)
	if !ok {
		return fmt.Errorf("refusing to set capabilities: %s owner is unknown", what)
	}
	if uid != 0 {
		return fmt.Errorf("refusing to set capabilities: %s must be owned by root", what)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("refusing to set capabilities: %s is writable by a non-root user", what)
	}
	return nil
}

func fileUID(info os.FileInfo) (uint32, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return 0, false
	}
	return stat.Uid, true
}
