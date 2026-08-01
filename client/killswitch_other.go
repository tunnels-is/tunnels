//go:build !linux

package client

import "errors"

func killSwitchSupported() bool { return false }

func enableKillSwitch() error { return errors.New("kill switch not implemented on this platform") }

func disableKillSwitch() {}
