//go:build !linux && !darwin && !windows && !freebsd && !openbsd

package client

import "errors"

func killSwitchSupported() bool { return false }

func enableKillSwitchIPv4() error {
	return errors.New("IPv4 kill switch is not implemented on this platform")
}

func disableKillSwitchIPv4() {}

func enableKillSwitchIPv6() error {
	return errors.New("IPv6 kill switch is not implemented on this platform")
}

func disableKillSwitchIPv6() {}
