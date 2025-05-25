//go:build aix || dragonfly || freebsd || (js && wasm) || linux || nacl || netbsd || openbsd || solaris

package client

import (
	"os/exec"

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

func OSSpecificInit() error {
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
