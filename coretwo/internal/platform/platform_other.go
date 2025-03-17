//go:build !(windows || darwin || linux)
// +build !windows,!darwin,!linux

package platform

import (
	"context"
	"fmt"
)

// initializeWindows is a stub for non-Windows platforms
func initializeWindows(ctx context.Context) error {
	return fmt.Errorf("Windows platform not supported on this OS")
}

// initializeDarwin is a stub for non-Darwin platforms
func initializeDarwin(ctx context.Context) error {
	return fmt.Errorf("macOS platform not supported on this OS")
}

// initializeLinux is a stub for non-Linux platforms
func initializeLinux(ctx context.Context) error {
	return fmt.Errorf("Linux platform not supported on this OS")
}

// initializeNetworkWindows is a stub for non-Windows platforms
func initializeNetworkWindows(cfg interface{}) error {
	return fmt.Errorf("Windows network not supported on this OS")
}

// initializeNetworkDarwin is a stub for non-Darwin platforms
func initializeNetworkDarwin(cfg interface{}) error {
	return fmt.Errorf("macOS network not supported on this OS")
}

// initializeNetworkLinux is a stub for non-Linux platforms
func initializeNetworkLinux(cfg interface{}) error {
	return fmt.Errorf("Linux network not supported on this OS")
}

// checkAdminWindows is a stub for non-Windows platforms
func checkAdminWindows() error {
	return fmt.Errorf("Windows admin check not supported on this OS")
}

// checkAdminDarwin is a stub for non-Darwin platforms
func checkAdminDarwin() error {
	return fmt.Errorf("macOS admin check not supported on this OS")
}

// checkAdminLinux is a stub for non-Linux platforms
func checkAdminLinux() error {
	return fmt.Errorf("Linux admin check not supported on this OS")
}
