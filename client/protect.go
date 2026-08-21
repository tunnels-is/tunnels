package client

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	wgconn "golang.zx2c4.com/wireguard/conn"
)

var protectWatchOnce sync.Once

// wgProtectMark is applied to the userspace WireGuard UDP socket (UAPI fwmark).
// Packets with this mark look up wgProtectTable, which holds the original
// default route, so handshake/data packets never re-enter the TUN.
const (
	wgProtectMark  = 0x7475 // 29813
	wgProtectTable = 21820
	wgProtectPrio  = 21820
)

func registerProtectHost(tun *TUN, ip string) {
	if tun == nil {
		return
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return
	}
	v4 := parsed.To4()
	if v4 == nil {
		return
	}
	s := v4.String()
	for _, h := range tun.protectHosts {
		if h == s {
			return
		}
	}
	tun.protectHosts = append(tun.protectHosts, s)
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

func pinProtectBind(b wgconn.Bind, ifIndex uint32) error {
	if b == nil || ifIndex == 0 {
		return nil
	}
	if wrapped, ok := b.(*ifaceProtectBind); ok {
		wrapped.ifIndex = ifIndex
		return bindProtectToInterface(wrapped.Bind, ifIndex)
	}
	return bindProtectToInterface(b, ifIndex)
}

func startProtectWatcher() {
	protectWatchOnce.Do(func() {
		go runProtectWatcher()
	})
}

func protectTickerLoop() {
	defer RecoverAndLog()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var stop <-chan struct{}
	if CancelContext != nil {
		stop = CancelContext.Done()
	} else {
		stop = make(chan struct{})
	}
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			refreshEndpointProtect()
		}
	}
}

func tunnelNameSet() map[string]struct{} {
	m := make(map[string]struct{})
	tunnelMapRange(func(tun *TUN) bool {
		if t := tun.tunnel.Load(); t != nil && t.Name != "" {
			m[t.Name] = struct{}{}
		}
		if meta := tun.meta.Load(); meta != nil && meta.IFName != "" {
			m[meta.IFName] = struct{}{}
		}
		return true
	})
	return m
}

func tunnelGatewaySet() map[string]struct{} {
	m := make(map[string]struct{})
	tunnelMapRange(func(tun *TUN) bool {
		if t := tun.tunnel.Load(); t != nil && t.IPv4Address != "" {
			m[t.IPv4Address] = struct{}{}
		}
		return true
	})
	return m
}

func fallbackPhysicalFromState() (ifName string, gw net.IP, ifIndex int, err error) {
	s := STATE.Load()
	nameP := s.DefaultInterfaceName.Load()
	gwP := s.DefaultGateway.Load()
	if nameP == nil || *nameP == "" || gwP == nil || (*gwP).To4() == nil {
		return "", nil, 0, fmt.Errorf("no stored physical default")
	}
	iface, err := net.InterfaceByName(*nameP)
	if err != nil {
		return "", nil, 0, err
	}
	if iface.Flags&net.FlagUp == 0 {
		return "", nil, 0, fmt.Errorf("stored physical interface %s is down", *nameP)
	}
	idx := iface.Index
	if id := s.DefaultInterfaceID.Load(); id > 0 {
		idx = int(id)
	}
	return *nameP, append(net.IP(nil), (*gwP).To4()...), idx, nil
}

func parseRouteGetGateway(out string) net.IP {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "gateway:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := net.ParseIP(fields[1])
		if ip == nil {
			continue
		}
		if v4 := ip.To4(); v4 != nil {
			return append(net.IP(nil), v4...)
		}
	}
	return nil
}

func collectProtectHosts() []string {
	seen := make(map[string]struct{})
	var hosts []string
	tunnelMapRange(func(tun *TUN) bool {
		for _, h := range tun.protectHosts {
			if _, ok := seen[h]; ok {
				continue
			}
			seen[h] = struct{}{}
			hosts = append(hosts, h)
		}
		return true
	})
	return hosts
}

func refreshEndpointProtect() {
	hasTunnel := false
	tunnelMapRange(func(_ *TUN) bool {
		hasTunnel = true
		return false
	})
	if !hasTunnel {
		return
	}

	ifName, gw, ifIndex, err := physicalIPv4Default()
	if err != nil {
		ifName, gw, ifIndex, err = fallbackPhysicalFromState()
		if err != nil {
			DEBUG("protect refresh: ", err)
			return
		}
	}

	hosts := collectProtectHosts()
	if protectRoutesPresent(ifName, gw, ifIndex, hosts) {
		return
	}

	if protErr := applyEndpointProtect(ifName, gw); protErr != nil {
		ERROR("protect refresh: ", protErr)
	}

	gw4 := gw.To4().String()
	tunnelMapRange(func(tun *TUN) bool {
		for _, host := range tun.protectHosts {
			if rerr := IP_AddRoute(host+"/32", ifName, gw4, "0"); rerr != nil {
				ERROR("protect refresh host route ", host, ": ", rerr)
			}
		}
		if pinErr := pinProtectBind(tun.wgBind, uint32(ifIndex)); pinErr != nil {
			ERROR("protect refresh bind: ", pinErr)
		}
		return true
	})

	s := STATE.Load()
	gwCopy := append(net.IP(nil), gw.To4()...)
	s.DefaultGateway.Store(&gwCopy)
	name := ifName
	s.DefaultInterfaceName.Store(&name)
	s.DefaultInterfaceID.Store(int32(ifIndex))
	DEBUG("protect refresh: restored pin via ", gw4, " dev ", ifName, " idx ", ifIndex)
}
