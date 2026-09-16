package client

import (
	"runtime/debug"
	"strings"
)

func IsDefaultConnection(IFName string) bool {
	return strings.EqualFold(IFName, DefaultTunnelName)
}

func RecoverAndLog() {
	if r := recover(); r != nil {
		ERROR(r, string(debug.Stack()))
	}
}

func tunnelMapRange(do func(tun *TUN) bool) {
	TunnelMap.Range(func(key string, value *TUN) bool {
		return do(value)
	})
}

func tunnelMetaMapRange(do func(tun *TunnelMeta) bool) {
	TunnelMetaMap.Range(func(key string, value *TunnelMeta) bool {
		return do(value)
	})
}
