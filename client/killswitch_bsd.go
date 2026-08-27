//go:build freebsd || openbsd

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
	out, err := exec.Command("route", args...).CombinedOutput()
	return string(out), err
}

func routeExists(out string, err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(out + " " + err.Error())
	return strings.Contains(s, "exists") || strings.Contains(s, "file exists")
}

func enableKillSwitchIPv4() error {
	if !killSwitchIPv4Active.CompareAndSwap(false, true) {
		return nil
	}
	out, err := runRoute("add", "-inet", "0.0.0.0/0", "-blackhole")
	if err != nil && !routeExists(out, err) {
		killSwitchIPv4Active.Store(false)
		return err
	}
	INFO("IPv4 kill switch on (blackhole 0.0.0.0/0)")
	return nil
}

func disableKillSwitchIPv4() {
	defer killSwitchIPv4Active.Store(false)
	// Same Darwin hazard: unconditional delete of 0.0.0.0/0 can remove the
	// real default. Only tear down when get(default) is a blackhole.
	out, err := runRoute("-n", "get", "-inet", "default")
	if err != nil || !strings.Contains(out, "BLACKHOLE") {
		return
	}
	_, _ = runRoute("delete", "-inet", "0.0.0.0/0", "-blackhole")
	INFO("IPv4 kill switch off")
}

func enableKillSwitchIPv6() error {
	if !killSwitchIPv6Active.CompareAndSwap(false, true) {
		return nil
	}
	out, err := runRoute("add", "-inet6", "::/0", "-blackhole")
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
	out, err := runRoute("-n", "get", "-inet6", "default")
	if err != nil || !strings.Contains(out, "BLACKHOLE") {
		return
	}
	_, _ = runRoute("delete", "-inet6", "::/0", "-blackhole")
	INFO("IPv6 kill switch off")
}
