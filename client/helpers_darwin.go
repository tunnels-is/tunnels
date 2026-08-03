//go:build darwin

package client

import (
	"fmt"
	"os"
	"os/exec"
)

func OSSpecificInit() error {
	return nil
}

func ValidateAdapterID(meta *TunnelMETA) error {
	return nil
}

// resolveTUNCreateName maps our logical tunnel IFName to a name wireguard-go
// accepts on Darwin. CreateTUN only allows "utun" (auto-assign) or "utunN".
// Logical names like "tunnels" are display/config only; the real kernel name
// comes from Device.Name() after create.
func resolveTUNCreateName(logical string) string {
	if logical == "utun" {
		return "utun"
	}
	var n int
	if _, err := fmt.Sscanf(logical, "utun%d", &n); err == nil && n >= 0 {
		return logical
	}
	return "utun"
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
