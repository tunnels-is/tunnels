//go:build aix || dragonfly || freebsd || (js && wasm) || linux || nacl || netbsd || openbsd || solaris

package client

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/tunnels-is/tunnels/setcap"
)

func openURL(url string) error {
	var cmd string
	var args []string
	cmd = "xdg-open"
	args = []string{url}
	if len(args) > 1 {
		args = append(args[:1], append([]string{""}, args[1:]...)...)
	}
	return exec.Command(cmd, args...).Start()
}

func tuneNetworkSysctls() {
	if runtime.GOOS != "linux" {
		return
	}

	sysctls := map[string]string{
		"/proc/sys/net/core/rmem_max":                    "26214400",
		"/proc/sys/net/core/wmem_max":                    "26214400",
		"/proc/sys/net/core/rmem_default":                "1048576",
		"/proc/sys/net/core/wmem_default":                "1048576",
		"/proc/sys/net/ipv4/tcp_congestion_control":      "bbr",
		"/proc/sys/net/core/default_qdisc":               "fq",
		"/proc/sys/net/ipv4/tcp_rmem":                    "4096\t1048576\t26214400",
		"/proc/sys/net/ipv4/tcp_wmem":                    "4096\t1048576\t26214400",
		"/proc/sys/net/ipv4/tcp_slow_start_after_idle":   "0",
		"/proc/sys/net/ipv4/tcp_mtu_probing":             "1",
	}

	for path, value := range sysctls {
		err := os.WriteFile(path, []byte(value), 0644)
		if err != nil {
			DEBUG("sysctl tune", path, ":", err)
		} else {
			DEBUG("sysctl set", path, "=", value)
		}
	}
}

func OSSpecificInit() error {
	tuneNetworkSysctls()
	return AdjustRoutersForTunneling()
}

func ValidateAdapterID(meta *TunnelMETA) error {
	return nil
}

func AdminCheck() {
	err := setcap.CheckCapabilities()
	if err != nil {
		ERROR("Tunnels does not have the proper permissions: ", err)
	} else {
		s := STATE.Load()
		s.adminState = true
	}
}
