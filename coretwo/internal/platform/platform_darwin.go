//go:build darwin
// +build darwin

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

func init() {
	initializeDarwin = func(ctx context.Context) error {
		// Check for administrative privileges
		if err := checkAdminDarwin(); err != nil {
			return err
		}

		// Check for utun driver
		if err := checkUtunDriver(); err != nil {
			return fmt.Errorf("utun driver check failed: %w", err)
		}

		return nil
	}

	initializeNetworkDarwin = func(cfg interface{}) error {
		// Initialize utun interface
		if err := initializeUtunInterface(); err != nil {
			return fmt.Errorf("utun interface initialization failed: %w", err)
		}

		// Configure routing
		if err := configureDarwinRouting(); err != nil {
			return fmt.Errorf("routing configuration failed: %w", err)
		}

		return nil
	}

	checkAdminDarwin = func() error {
		// Check if running with root privileges
		if os.Geteuid() != 0 {
			return fmt.Errorf("root privileges required")
		}
		return nil
	}
}

func checkUtunDriver() error {
	// Check if utun driver is available
	cmd := exec.Command("kextstat", "-b", "com.apple.driver.utun")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("utun driver not found: %w", err)
	}
	return nil
}

func initializeUtunInterface() error {
	// Create utun interface using ifconfig
	cmd := exec.Command("ifconfig", "utun0", "create")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create utun interface: %w", err)
	}

	// Configure interface IP
	cmd = exec.Command("ifconfig", "utun0", "inet", "10.0.0.2", "10.0.0.1", "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure utun interface IP: %w", err)
	}

	return nil
}

func configureDarwinRouting() error {
	// Add default route through utun interface
	cmd := exec.Command("route", "add", "default", "10.0.0.1")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure routing: %w", err)
	}

	return nil
}
