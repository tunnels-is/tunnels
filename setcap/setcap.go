package setcap

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/term"
	"kernel.org/pub/linux/libs/security/libcap/cap"
)

func CheckCapabilities() (err error) {
	orig := cap.GetProc()
	defer orig.SetProc() // restore original caps on exit.

	c, err := orig.Dup()
	if err != nil {
		return fmt.Errorf("failed to get capabilities, err: %s", err)
	}

	missingFlags := false
	on, _ := c.GetFlag(cap.Permitted, cap.NET_BIND_SERVICE)
	if !on {
		missingFlags = true
	}

	on, _ = c.GetFlag(cap.Permitted, cap.NET_ADMIN)
	if !on {
		missingFlags = true
	}

	on, _ = c.GetFlag(cap.Permitted, cap.NET_RAW)
	if !on {
		missingFlags = true
	}

	if !missingFlags {
		return
	}

	fmt.Print("\n" +
		"\033[1;34m╔══════════════════════════════════════════════════════════╗\n" +
		"║              Network Permissions Required                ║\n" +
		"╚══════════════════════════════════════════════════════════╝\033[0m\n" +
		"\n  Tunnels needs capabilities to manage network interfaces.\n" +
		"  This does NOT grant root/sudo access to the system.\n\n" +
		"  🔑 Password: ")

	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err == nil {
		password := string(bytePassword)
		cmd := exec.Command("sudo", "-S", "/usr/sbin/setcap", "cap_net_raw,cap_net_bind_service,cap_net_admin+eip", os.Args[0])
		cmd.Stdin = strings.NewReader(password + "\n")
		cmd.Stdout = os.Stdout
		err = cmd.Run()
		_ = exec.Command("sudo", "-kK")
		if err != nil {
			fmt.Printf("\n\033[1;31m  ✗  Unable to set capabilities: %s\033[0m\n\n"+
				"  To grant permissions manually, run:\n"+
				"  \033[1;34m→\033[0m  sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' [BINARY]\n\n", err)
		} else {
			// Reload binary after applying set cap
			argv0, _ := exec.LookPath(os.Args[0])
			syscall.Exec(argv0, os.Args, os.Environ())
		}
		os.Exit(1)
	}
	return
}
