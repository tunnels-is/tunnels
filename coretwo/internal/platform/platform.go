package platform

import (
	"context"
	"fmt"
	"runtime"
)

// Platform-specific function declarations
var (
	initializeWindows        func(context.Context) error
	initializeDarwin         func(context.Context) error
	initializeLinux          func(context.Context) error
	initializeNetworkWindows func(interface{}) error
	initializeNetworkDarwin  func(interface{}) error
	initializeNetworkLinux   func(interface{}) error
	checkAdminWindows        func() error
	checkAdminDarwin         func() error
	checkAdminLinux          func() error
)

// Initialize performs platform-specific initialization
func Initialize(ctx context.Context) error {
	switch runtime.GOOS {
	case "windows":
		return initializeWindows(ctx)
	case "darwin":
		return initializeDarwin(ctx)
	case "linux":
		return initializeLinux(ctx)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// InitializeNetwork initializes platform-specific network components
func InitializeNetwork(cfg interface{}) error {
	switch runtime.GOOS {
	case "windows":
		return initializeNetworkWindows(cfg)
	case "darwin":
		return initializeNetworkDarwin(cfg)
	case "linux":
		return initializeNetworkLinux(cfg)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// CheckAdmin checks if the process has administrative privileges
func CheckAdmin() error {
	switch runtime.GOOS {
	case "windows":
		return checkAdminWindows()
	case "darwin":
		return checkAdminDarwin()
	case "linux":
		return checkAdminLinux()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}
