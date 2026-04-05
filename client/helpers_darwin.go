//go:build darwin

package client

import (
	"os"
	"os/exec"
)

func openURL(url string) error {
	var cmd string
	var args []string
	cmd = "open"
	args = []string{url}
	if len(args) > 1 {
		args = append(args[:1], append([]string{""}, args[1:]...)...)
	}
	return exec.Command(cmd, args...).Start()
}

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
