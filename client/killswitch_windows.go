//go:build windows

package client

import (
	"net"
	"strings"
	"sync/atomic"
)

var (
	killSwitchIPv4Active atomic.Bool
	killSwitchIPv6Active atomic.Bool
)

func killSwitchSupported() bool { return true }

func windowsLoopbackName() string {
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, ifi := range ifaces {
			if ifi.Flags&net.FlagLoopback != 0 && ifi.Name != "" {
				return ifi.Name
			}
		}
	}
	return "Loopback Pseudo-Interface 1"
}

func runNetsh(args ...string) (string, error) {
	cmd := hiddenCommand("netsh", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func netshAlready(out string, err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(out + " " + err.Error())
	return strings.Contains(s, "already exists") || strings.Contains(s, "object already exists")
}

func enableKillSwitchIPv4() error {
	if !killSwitchIPv4Active.CompareAndSwap(false, true) {
		return nil
	}
	loop := windowsLoopbackName()
	out, err := runNetsh(
		"interface", "ipv4", "add", "route", "0.0.0.0/0",
		`interface=`+loop, "metric=1", "store=active",
	)
	if err != nil && !netshAlready(out, err) {
		killSwitchIPv4Active.Store(false)
		return err
	}
	INFO("IPv4 kill switch on (loopback 0.0.0.0/0 metric 1)")
	return nil
}

func disableKillSwitchIPv4() {
	loop := windowsLoopbackName()
	_, _ = runNetsh(
		"interface", "ipv4", "delete", "route", "0.0.0.0/0",
		`interface=`+loop, "store=active",
	)
	if killSwitchIPv4Active.CompareAndSwap(true, false) {
		INFO("IPv4 kill switch off")
	}
}

func enableKillSwitchIPv6() error {
	if !killSwitchIPv6Active.CompareAndSwap(false, true) {
		return nil
	}
	loop := windowsLoopbackName()
	out, err := runNetsh(
		"interface", "ipv6", "add", "route", "::/0",
		`interface=`+loop, "metric=1", "store=active",
	)
	if err != nil && !netshAlready(out, err) {
		killSwitchIPv6Active.Store(false)
		DEBUG("IPv6 kill switch: ", err, " out: ", out)
		return nil
	}
	INFO("IPv6 kill switch on (loopback ::/0 metric 1)")
	return nil
}

func disableKillSwitchIPv6() {
	loop := windowsLoopbackName()
	_, _ = runNetsh(
		"interface", "ipv6", "delete", "route", "::/0",
		`interface=`+loop, "store=active",
	)
	if killSwitchIPv6Active.CompareAndSwap(true, false) {
		INFO("IPv6 kill switch off")
	}
}
