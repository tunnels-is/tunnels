//go:build windows
// +build windows

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func init() {
	initializeWindows = func(ctx context.Context) error {
		// Check for administrative privileges
		if err := checkAdminWindows(); err != nil {
			return err
		}

		// Check for WinTun driver
		if err := checkWinTunDriver(); err != nil {
			return fmt.Errorf("WinTun driver check failed: %w", err)
		}

		return nil
	}

	initializeNetworkWindows = func(cfg interface{}) error {
		// Initialize WinTun interface
		if err := initializeWinTunInterface(); err != nil {
			return fmt.Errorf("WinTun interface initialization failed: %w", err)
		}

		// Configure routing
		if err := configureWindowsRouting(); err != nil {
			return fmt.Errorf("routing configuration failed: %w", err)
		}

		return nil
	}

	checkAdminWindows = func() error {
		// Check if running with administrative privileges
		_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
		if err != nil {
			return fmt.Errorf("administrative privileges required")
		}
		return nil
	}
}

func checkWinTunDriver() error {
	// Check if WinTun driver is installed
	driverPath := filepath.Join(os.Getenv("SystemRoot"), "System32", "drivers", "wintun.sys")
	if _, err := os.Stat(driverPath); err != nil {
		return fmt.Errorf("WinTun driver not found: %w", err)
	}
	return nil
}

func initializeWinTunInterface() error {
	// Create WinTun interface using netsh
	cmd := exec.Command("netsh", "interface", "set", "interface", "name=tun0", "admin=enabled")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable WinTun interface: %w", err)
	}

	// Configure interface IP
	cmd = exec.Command("netsh", "interface", "ip", "set", "address", "name=tun0", "static", "10.0.0.2", "255.255.255.0", "10.0.0.1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure WinTun interface IP: %w", err)
	}

	return nil
}

func configureWindowsRouting() error {
	// Add default route through WinTun interface
	cmd := exec.Command("route", "add", "0.0.0.0", "mask", "0.0.0.0", "10.0.0.1", "metric", "1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure routing: %w", err)
	}

	return nil
}
