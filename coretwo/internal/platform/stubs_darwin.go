//go:build !darwin
// +build !darwin

package platform

import (
	"context"
	"fmt"
	"runtime"
)

func initializeDarwin(ctx context.Context) error {
	return fmt.Errorf("macOS platform not supported on %s", runtime.GOOS)
}

func initializeNetworkDarwin(cfg interface{}) error {
	return fmt.Errorf("macOS network not supported on %s", runtime.GOOS)
}

func checkAdminDarwin() error {
	return fmt.Errorf("macOS admin check not supported on %s", runtime.GOOS)
}
