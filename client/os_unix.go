//go:build aix || dragonfly || freebsd || (js && wasm) || linux || nacl || netbsd || openbsd || solaris

package client

import (
	"github.com/tunnels-is/tunnels/setcap"
)

// OSSpecificInit ...
func OSSpecificInit() error {
	return AdjustRoutersForTunneling()
}

// ValidateAdapterID ...
func ValidateAdapterID(meta *TunnelMeta) error {
	return nil
}

func resolveTUNCreateName(logical string) string {
	return logical
}

// AdminCheck ...
func AdminCheck() {
	err := setcap.CheckCapabilities()
	if err != nil {
		ERROR("Tunnels does not have the proper permissions: ", err)
	} else {
		s := STATE.Load()
		s.adminState = true
	}
}
