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

// resolveTUNCreateName returns the kernel TUN name. On Linux/BSD the logical
// IFName is the real interface name.
func resolveTUNCreateName(logical string) string {
	return logical
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
