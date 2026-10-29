//go:build darwin || freebsd || openbsd

package client

import (
	"os/exec"
	"strings"
	"sync/atomic"
)

var (
	killSwitchIPv4Active atomic.Bool
	killSwitchIPv6Active atomic.Bool

	removedDefaultsV4 []string
	removedDefaultsV6 []string
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

func routeMissing(out string, err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(out + " " + err.Error())
	return strings.Contains(s, "not in table") || strings.Contains(s, "no such process")
}

func routeGetFlagsBlackhole(getArgs ...string) bool {
	out, err := runRoute(append([]string{"-n", "get"}, getArgs...)...)
	if err != nil {
		return false
	}
	return routeGetOutputHasBlackhole(out)
}

func routeTableHasBlackhole(family string) bool {
	out, err := exec.Command("netstat", "-rn", "-f", family).CombinedOutput()
	if err != nil {
		return false
	}
	return netstatHasBlackholeDefault(string(out))
}

func rememberRemovedDefault(v6 bool, gw string) {
	list := &removedDefaultsV4
	if v6 {
		list = &removedDefaultsV6
	}
	for _, have := range *list {
		if have == gw {
			return
		}
	}
	*list = append(*list, gw)
}

func suppressPhysicalDefaults(v6 bool) {
	inet := "-inet"
	if v6 {
		inet = "-inet6"
	}
	tunnels := tunnelNameSet()
	gateways := tunnelGatewaySet()
	for i := 0; i < 8; i++ {
		out, err := runRoute("-n", "get", inet, "default")
		if err != nil {
			return
		}
		switch classifyDefaultRoute(out, tunnels, gateways) {
		case defaultChoiceBlackhole, defaultChoiceTunnel, defaultChoiceNone:
			return
		}
		gw := routeGetGateway(out)
		if gw == nil {
			return
		}
		rememberRemovedDefault(v6, gw.String())
		_, _ = runRoute("-n", "delete", inet, "default", gw.String())
	}
}

func ensureBlackhole(args ...string) error {
	out, err := runRoute(args...)
	if err != nil && !routeExists(out, err) {
		return err
	}
	return nil
}

func deleteBlackhole(v6 bool) {
	inet := "-inet"
	family := "inet"
	dst := "0.0.0.0/0"
	if v6 {
		inet = "-inet6"
		family = "inet6"
		dst = "::/0"
	}
	if !routeGetFlagsBlackhole(inet, "default") && !routeTableHasBlackhole(family) {
		return
	}
	out, err := runRoute("-n", "delete", inet, dst, "-blackhole")
	if err != nil && !routeMissing(out, err) {
		DEBUG("kill switch blackhole delete: ", err, " out: ", out)
	}
}

func restoreRemovedDefaults(v6 bool) {
	inet := "-inet"
	list := &removedDefaultsV4
	if v6 {
		inet = "-inet6"
		list = &removedDefaultsV6
	}
	for _, gw := range *list {
		out, err := runRoute("-n", "add", inet, "default", gw)
		if err != nil && !routeExists(out, err) {
			DEBUG("kill switch restore default ", gw, ": ", err, " out: ", out)
		}
	}
	*list = nil
}

func enableKillSwitchIPv4() error {
	if err := ensureBlackhole("-n", "add", "-inet", "0.0.0.0/0", "-blackhole"); err != nil {
		killSwitchIPv4Active.Store(false)
		return err
	}
	suppressPhysicalDefaults(false)
	if killSwitchIPv4Active.CompareAndSwap(false, true) {
		INFO("IPv4 kill switch on (blackhole 0.0.0.0/0)")
	}
	return nil
}

func disableKillSwitchIPv4() {
	deleteBlackhole(false)
	restoreRemovedDefaults(false)
	if killSwitchIPv4Active.CompareAndSwap(true, false) {
		INFO("IPv4 kill switch off")
	}
}

func enableKillSwitchIPv6() error {
	if err := ensureBlackhole("-n", "add", "-inet6", "::/0", "-blackhole"); err != nil {
		killSwitchIPv6Active.Store(false)
		DEBUG("IPv6 kill switch: ", err)
		return nil
	}
	suppressPhysicalDefaults(true)
	if killSwitchIPv6Active.CompareAndSwap(false, true) {
		INFO("IPv6 kill switch on (blackhole ::/0)")
	}
	return nil
}

func disableKillSwitchIPv6() {
	deleteBlackhole(true)
	restoreRemovedDefaults(true)
	if killSwitchIPv6Active.CompareAndSwap(true, false) {
		INFO("IPv6 kill switch off")
	}
}
