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

	fmt.Println("Tunnels needs access to manage network capabilities, this access does NOT include root/sudo access to the system")
	fmt.Println("Enter Password: ")

	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err == nil {
		password := string(bytePassword)
		cmd := exec.Command("sudo", "-S", "/usr/sbin/setcap", "cap_net_raw,cap_net_bind_service,cap_net_admin+eip", os.Args[0])
		cmd.Stdin = strings.NewReader(password + "\n")
		cmd.Stdout = os.Stdout
		err = cmd.Run()
		_ = exec.Command("sudo", "-kK")
		if err != nil {
			fmt.Println("")
			fmt.Println("Unable to setcap: ", err)
			fmt.Println("")
			fmt.Println("RUN: `sudo setcap 'cap_net_raw,cap_net_bind_service,cap_net_admin+eip' [BINARY]` in order to give it permissions to change and manage networks")
			fmt.Println("")
		} else {
			// Reload binary after applying set cap
			argv0, _ := exec.LookPath(os.Args[0])
			syscall.Exec(argv0, os.Args, os.Environ())
		}
		os.Exit(1)
	}
	return
}
