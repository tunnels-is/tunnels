//go:build darwin

package client

import (
	"os/exec"
	"strings"
	"sync/atomic"
)

var (
	killSwitchIPv4Active atomic.Bool
	killSwitchIPv6Active atomic.Bool
)

func killSwitchSupported() bool { return true }

func runRoute(args ...string) (string, error) {
	cmd := exec.Command("route", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func routeExists(out string, err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(out + " " + err.Error())
	return strings.Contains(s, "exists") || strings.Contains(s, "file exists")
}

func routeMissing(out string, err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(out + " " + err.Error())
	return strings.Contains(s, "not in table") || strings.Contains(s, "no such process")
}

// routeGetFlagsBlackhole reports whether `route -n get …` selected a blackhole.
// On Darwin, `route delete -inet 0.0.0.0/0 -blackhole` can still match the real
// default gateway when no blackhole exists — wiping internet on every app start
// when kill switch is off. Only delete when get(default) is actually BLACKHOLE.
func routeGetFlagsBlackhole(getArgs ...string) bool {
	out, err := runRoute(append([]string{"-n", "get"}, getArgs...)...)
	if err != nil {
		return false
	}
	return routeGetOutputHasBlackhole(out)
}

func routeGetOutputHasBlackhole(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "flags:") && strings.Contains(line, "BLACKHOLE") {
			return true
		}
	}
	return false
}

func enableKillSwitchIPv4() error {
	if !killSwitchIPv4Active.CompareAndSwap(false, true) {
		return nil
	}
	out, err := runRoute("-n", "add", "-inet", "0.0.0.0/0", "-blackhole")
	if err != nil && !routeExists(out, err) {
		killSwitchIPv4Active.Store(false)
		return err
	}
	INFO("IPv4 kill switch on (blackhole 0.0.0.0/0)")
	return nil
}

func disableKillSwitchIPv4() {
	defer killSwitchIPv4Active.Store(false)
	if !routeGetFlagsBlackhole("-inet", "default") {
		return
	}
	out, err := runRoute("-n", "delete", "-inet", "0.0.0.0/0", "-blackhole")
	if err != nil && !routeMissing(out, err) {
		ERROR("IPv4 kill switch off: ", err, " out: ", out)
		return
	}
	INFO("IPv4 kill switch off")
}

func enableKillSwitchIPv6() error {
	if !killSwitchIPv6Active.CompareAndSwap(false, true) {
		return nil
	}
	out, err := runRoute("-n", "add", "-inet6", "::/0", "-blackhole")
	if err != nil && !routeExists(out, err) {
		killSwitchIPv6Active.Store(false)
		DEBUG("IPv6 kill switch: ", err, " out: ", out)
		return nil
	}
	INFO("IPv6 kill switch on (blackhole ::/0)")
	return nil
}

func disableKillSwitchIPv6() {
	defer killSwitchIPv6Active.Store(false)
	if !routeGetFlagsBlackhole("-inet6", "default") {
		return
	}
	out, err := runRoute("-n", "delete", "-inet6", "::/0", "-blackhole")
	if err != nil && !routeMissing(out, err) {
		DEBUG("IPv6 kill switch off: ", err, " out: ", out)
		return
	}
	INFO("IPv6 kill switch off")
}
