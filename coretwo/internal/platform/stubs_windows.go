//go:build !windows
// +build !windows

package platform

import (
	"context"
	"fmt"
	"runtime"
)

func initializeWindows(ctx context.Context) error {
	return fmt.Errorf("Windows platform not supported on %s", runtime.GOOS)
}

func initializeNetworkWindows(cfg interface{}) error {
	return fmt.Errorf("Windows network not supported on %s", runtime.GOOS)
}

func checkAdminWindows() error {
	return fmt.Errorf("Windows admin check not supported on %s", runtime.GOOS)
}
