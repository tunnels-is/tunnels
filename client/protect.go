package client

import (
	"fmt"

	wgconn "golang.zx2c4.com/wireguard/conn"
)

// wgProtectMark is applied to the userspace WireGuard UDP socket (UAPI fwmark).
// Packets with this mark look up wgProtectTable, which holds the original
// default route, so handshake/data packets never re-enter the TUN.
const (
	wgProtectMark  = 0x7475 // 29813
	wgProtectTable = 21820
	wgProtectPrio  = 21820
)

func wgIPCConfig(privHex, pubHex, endpointIP, endpointPort string) string {
	return fmt.Sprintf(
		"private_key=%s\nfwmark=%d\npublic_key=%s\nendpoint=%s:%s\nallowed_ip=0.0.0.0/0\npersistent_keepalive_interval=25\n\n",
		privHex, wgProtectMark, pubHex, endpointIP, endpointPort,
	)
}

type ifaceProtectBind struct {
	wgconn.Bind
	ifIndex uint32
}

func newProtectBind(ifIndex uint32) wgconn.Bind {
	inner := wgconn.NewDefaultBind()
	if ifIndex == 0 {
		return inner
	}
	return &ifaceProtectBind{Bind: inner, ifIndex: ifIndex}
}

func (b *ifaceProtectBind) Open(port uint16) (fns []wgconn.ReceiveFunc, actualPort uint16, err error) {
	fns, actualPort, err = b.Bind.Open(port)
	if err != nil {
		return fns, actualPort, err
	}
	if bindErr := bindProtectToInterface(b.Bind, b.ifIndex); bindErr != nil {
		ERROR("unable to bind WireGuard socket to physical interface index ", b.ifIndex, ": ", bindErr)
	} else {
		DEBUG("bound WireGuard socket to physical interface index ", b.ifIndex)
	}
	return fns, actualPort, nil
}
