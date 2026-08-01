//go:build darwin

package client

import (
	"os"
	"os/exec"
)

func OSSpecificInit() error {
	return nil
}

func ValidateAdapterID(meta *TunnelMETA) error {
	return nil
}

func AdminCheck() {
	DEBUG("Admin check")
	s := STATE.Load()

	if os.Geteuid() == 0 {
		s.adminState = true
		return
	}

	cmd := exec.Command("sudo", "-n", "true")
	if err := cmd.Run(); err == nil {
		s.adminState = true
		return
	}

	s.adminState = false
}
