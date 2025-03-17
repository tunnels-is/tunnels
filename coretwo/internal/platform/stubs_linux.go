//go:build !linux
// +build !linux

package platform

import (
	"context"
	"fmt"
	"runtime"
)

func initializeLinux(ctx context.Context) error {
	return fmt.Errorf("Linux platform not supported on %s", runtime.GOOS)
}

func initializeNetworkLinux(cfg interface{}) error {
	return fmt.Errorf("Linux network not supported on %s", runtime.GOOS)
}

func checkAdminLinux() error {
	return fmt.Errorf("Linux admin check not supported on %s", runtime.GOOS)
}
