//go:build !linux

package client

import "errors"

// The route-based kill switch is only implemented on Linux. On other platforms
// killSwitchSupported() returns false and the connect path emits a non-
// suppressed SECURITY warning, so the toggle can't silently imply protection it
// doesn't provide.

func killSwitchSupported() bool { return false }

func enableKillSwitch() error { return errors.New("kill switch not implemented on this platform") }

func disableKillSwitch() {}
