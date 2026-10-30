//go:build windows

package client

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	killSwitchIPv4Active atomic.Bool
	killSwitchIPv6Active atomic.Bool

	windowsDemoted = map[string]uint32{}
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

func windowsDemoteKey(v6 bool, ifIndex uint32) string {
	if v6 {
		return fmt.Sprintf("6:%d", ifIndex)
	}
	return fmt.Sprintf("4:%d", ifIndex)
}

func ensureWindowsSink(v6 bool) error {
	loop := windowsLoopbackName()
	prefix := "0.0.0.0/0"
	family := "ipv4"
	if v6 {
		prefix = "::/0"
		family = "ipv6"
	}
	out, err := runNetsh(
		"interface", family, "add", "route", prefix,
		`interface=`+loop, "metric="+fmt.Sprint(killSwitchMetric), "store=active",
	)
	if err != nil && !netshAlready(out, err) {
		return err
	}
	if v6 {
		return setWindowsIPv6Metric(prefix, loop, killSwitchMetric)
	}
	return setIPv4RouteMetric(prefix, loop, fmt.Sprint(killSwitchMetric))
}

func setWindowsIPv6Metric(prefix, ifName string, metric int) error {
	cmd := hiddenCommand(
		"netsh", "interface", "ipv6", "set", "route", prefix,
		`interface="`+ifName+`"`,
		"metric="+fmt.Sprint(metric),
		"store=active",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s - out: %s", err, out)
	}
	return nil
}

func demoteWindowsDefaults(v6 bool) {
	family := uint16(windows.AF_INET)
	if v6 {
		family = uint16(windows.AF_INET6)
	}
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(family, &table); err != nil {
		DEBUG("kill switch route table: ", err)
		return
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	byIdx := make(map[int]net.Interface, len(ifaces))
	for _, ifi := range ifaces {
		byIdx[ifi.Index] = ifi
	}
	tunnels := tunnelNameSet()
	loop := windowsLoopbackName()

	for _, row := range table.Rows() {
		ifi, ok := byIdx[int(row.InterfaceIndex)]
		loopback := row.Loopback != 0 || (ok && (ifi.Flags&net.FlagLoopback != 0 || ifi.Name == loop))
		tunnel := ok && func() bool { _, yes := tunnels[ifi.Name]; return yes }()
		if !raisePhysicalDefault(int(row.DestinationPrefix.PrefixLength), int(row.Metric), loopback, tunnel) {
			continue
		}
		if !ok || ifi.Name == "" {
			continue
		}
		key := windowsDemoteKey(v6, row.InterfaceIndex)
		if _, saved := windowsDemoted[key]; !saved {
			windowsDemoted[key] = row.Metric
		}
		var setErr error
		if v6 {
			setErr = setWindowsIPv6Metric("::/0", ifi.Name, killSwitchDemotedMetric)
		} else {
			setErr = setIPv4RouteMetric("0.0.0.0/0", ifi.Name, fmt.Sprint(killSwitchDemotedMetric))
		}
		if setErr != nil {
			DEBUG("kill switch demote ", ifi.Name, ": ", setErr)
		}
	}
}

func restoreWindowsDefaults(v6 bool) {
	prefix := "6:"
	dest := "::/0"
	if !v6 {
		prefix = "4:"
		dest = "0.0.0.0/0"
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	byIdx := make(map[int]net.Interface, len(ifaces))
	for _, ifi := range ifaces {
		byIdx[ifi.Index] = ifi
	}
	for key, metric := range windowsDemoted {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		var idx int
		if _, err := fmt.Sscanf(key, prefix+"%d", &idx); err != nil {
			delete(windowsDemoted, key)
			continue
		}
		ifi, ok := byIdx[idx]
		if ok && ifi.Name != "" {
			var setErr error
			if v6 {
				setErr = setWindowsIPv6Metric(dest, ifi.Name, int(metric))
			} else {
				setErr = setIPv4RouteMetric(dest, ifi.Name, fmt.Sprint(metric))
			}
			if setErr != nil {
				DEBUG("kill switch restore ", ifi.Name, ": ", setErr)
			}
		}
		delete(windowsDemoted, key)
	}
}

func deleteWindowsSink(v6 bool) {
	loop := windowsLoopbackName()
	prefix := "0.0.0.0/0"
	family := "ipv4"
	if v6 {
		prefix = "::/0"
		family = "ipv6"
	}
	_, _ = runNetsh(
		"interface", family, "delete", "route", prefix,
		`interface=`+loop, "store=active",
	)
}

func enableKillSwitchIPv4() error {
	if err := ensureWindowsSink(false); err != nil {
		killSwitchIPv4Active.Store(false)
		return err
	}
	demoteWindowsDefaults(false)
	if killSwitchIPv4Active.CompareAndSwap(false, true) {
		INFO("IPv4 kill switch on (loopback 0.0.0.0/0 metric ", killSwitchMetric, ")")
	}
	return nil
}

func disableKillSwitchIPv4() {
	restoreWindowsDefaults(false)
	deleteWindowsSink(false)
	if killSwitchIPv4Active.CompareAndSwap(true, false) {
		INFO("IPv4 kill switch off")
	}
}

func enableKillSwitchIPv6() error {
	if err := ensureWindowsSink(true); err != nil {
		killSwitchIPv6Active.Store(false)
		DEBUG("IPv6 kill switch: ", err)
		return nil
	}
	demoteWindowsDefaults(true)
	if killSwitchIPv6Active.CompareAndSwap(false, true) {
		INFO("IPv6 kill switch on (loopback ::/0 metric ", killSwitchMetric, ")")
	}
	return nil
}

func disableKillSwitchIPv6() {
	restoreWindowsDefaults(true)
	deleteWindowsSink(true)
	if killSwitchIPv6Active.CompareAndSwap(true, false) {
		INFO("IPv6 kill switch off")
	}
}
