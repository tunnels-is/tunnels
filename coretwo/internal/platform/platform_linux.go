//go:build linux
// +build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func init() {
	initializeLinux = func(ctx context.Context) error {
		// Check for root privileges
		if err := checkAdminLinux(); err != nil {
			return err
		}

		// Check for TUN module
		if err := checkTunModule(); err != nil {
			return fmt.Errorf("TUN module check failed: %w", err)
		}

		return nil
	}

	initializeNetworkLinux = func(cfg interface{}) error {
		// Initialize TUN interface
		if err := initializeTunInterface(); err != nil {
			return fmt.Errorf("TUN interface initialization failed: %w", err)
		}

		// Configure routing
		if err := configureLinuxRouting(); err != nil {
			return fmt.Errorf("routing configuration failed: %w", err)
		}

		return nil
	}

	checkAdminLinux = func() error {
		// Check if running with root privileges
		if os.Geteuid() != 0 {
			return fmt.Errorf("root privileges required")
		}
		return nil
	}
}

func checkTunModule() error {
	// Check if TUN module is loaded
	cmd := exec.Command("lsmod", "|", "grep", "tun")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("TUN module not found: %w", err)
	}
	return nil
}

func initializeTunInterface() error {
	// Create TUN interface
	cmd := exec.Command("ip", "tuntap", "add", "mode", "tun", "dev", "tun0")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create TUN interface: %w", err)
	}

	// Configure interface IP
	cmd = exec.Command("ip", "addr", "add", "10.0.0.2/24", "dev", "tun0")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure TUN interface IP: %w", err)
	}

	// Bring interface up
	cmd = exec.Command("ip", "link", "set", "tun0", "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to bring up TUN interface: %w", err)
	}

	return nil
}

func configureLinuxRouting() error {
	// Add default route through TUN interface
	cmd := exec.Command("ip", "route", "add", "default", "via", "10.0.0.1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure routing: %w", err)
	}

	return nil
}
