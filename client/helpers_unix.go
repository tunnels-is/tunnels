//go:build aix || dragonfly || freebsd || (js && wasm) || linux || nacl || netbsd || openbsd || solaris

package client

import (
	"github.com/tunnels-is/tunnels/setcap"
)

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
