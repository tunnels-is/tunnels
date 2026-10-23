package client

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wgconn "golang.zx2c4.com/wireguard/conn"
	wgdevice "golang.zx2c4.com/wireguard/device"
)

var protectWatchOnce sync.Once

// protectPinnedIndex is the physical ifindex last applied to the WireGuard
// sockets. Zero means the sockets still need a pin (or a sticky-source clear).
var protectPinnedIndex atomic.Uint32

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
	// Linux must pass *conn.StdNetBind through. wireguard-go starts the
	// route listener that clears the sticky source only for that concrete
	// type. A wrapper makes every later send keep the pre-suspend ifindex,
	// and that PKTINFO overrides IP_UNICAST_IF. The socket is pinned after Up.
	if ifIndex == 0 || !protectBindWraps() {
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
			func() {
				defer RecoverAndLog()
				refreshEndpointProtect()
			}()
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
	if !physicalIfaceUsable(iface.Flags) {
		return "", nil, 0, fmt.Errorf("stored physical interface %s is not running", *nameP)
	}
	// Use the live index. The stored id goes stale if the NIC is recreated
	// across suspend, and pinning that id sends WireGuard at the wrong device.
	return *nameP, append(net.IP(nil), (*gwP).To4()...), iface.Index, nil
}

// physicalIfaceUsable requires carrier, not only admin-up. During suspend
// the NIC stays IFF_UP while NetworkManager has already deleted its routes;
// installing against it returns ENETUNREACH.
func physicalIfaceUsable(flags net.Flags) bool {
	return flags&net.FlagUp != 0 && flags&net.FlagRunning != 0
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

// hostProtectRouteRequired reports whether a protect host still needs a
// via-gateway /32. On-link destinations are skipped by addIPv4Route; the
// 5s watcher must treat those as already satisfied or it will reinstall
// every tick.
func hostProtectRouteRequired(host string) bool {
	if host == "" {
		return false
	}
	return !ipv4HostOnLink(host + "/32")
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
		protectPinnedIndex.Store(0)
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
	if gw.To4() == nil || ifIndex <= 0 {
		DEBUG("protect refresh: physical default is not usable")
		return
	}

	hosts := collectProtectHosts()
	installed := false
	if !protectRoutesPresent(ifName, gw, ifIndex, hosts) {
		if installErr := installEndpointProtectRoutes(ifName, gw); installErr == nil {
			installed = true
		}
	}

	pinErr := pinAllProtectBinds(ifIndex)
	pinned := pinErr == nil
	// Clear the sticky source only after the socket is bound to the physical
	// NIC. IP_PKTINFO from the last receive overrides that bind, so a source
	// learned while the default route was the tunnel keeps sending into it.
	if pinned && (installed || protectPinnedIndex.Load() != uint32(ifIndex)) {
		clearAllWGSticky()
		protectPinnedIndex.Store(uint32(ifIndex))
	}
	if !installed {
		return
	}

	s := STATE.Load()
	gwCopy := append(net.IP(nil), gw.To4()...)
	s.DefaultGateway.Store(&gwCopy)
	name := ifName
	s.DefaultInterfaceName.Store(&name)
	s.DefaultInterfaceID.Store(int32(ifIndex))
	if !pinned {
		return
	}
	DEBUG("protect refresh: restored pin via ", gw.To4().String(), " dev ", ifName, " idx ", ifIndex)
}

func installEndpointProtectRoutes(ifName string, gw net.IP) error {
	if err := applyEndpointProtect(ifName, gw); err != nil {
		ERROR("protect refresh: ", err)
		return err
	}
	gw4 := gw.To4().String()
	var failed error
	tunnelMapRange(func(tun *TUN) bool {
		for _, host := range tun.protectHosts {
			if !hostProtectRouteRequired(host) {
				continue
			}
			if rerr := addIPv4Route(host+"/32", ifName, gw4, "0"); rerr != nil {
				ERROR("protect refresh host route ", host, ": ", rerr)
				failed = rerr
			}
		}
		return true
	})
	return failed
}

func pinAllProtectBinds(ifIndex int) error {
	if ifIndex <= 0 {
		return fmt.Errorf("interface index is 0")
	}
	var failed error
	tunnelMapRange(func(tun *TUN) bool {
		if pinErr := pinProtectBind(tun.wgBind, uint32(ifIndex)); pinErr != nil {
			ERROR("protect refresh bind: ", pinErr)
			failed = pinErr
		}
		return true
	})
	return failed
}

func clearAllWGSticky() {
	tunnelMapRange(func(tun *TUN) bool {
		clearWGStickySrc(tun.wgDevice)
		return true
	})
}

// clearWGStickySrc drops the cached PKTINFO source. BindSetMark clears it
// only when the mark changes, so the mark is toggled and put back.
func clearWGStickySrc(dev *wgdevice.Device) {
	if dev == nil {
		return
	}
	if err := dev.BindSetMark(0); err != nil {
		DEBUG("clear wireguard sticky source: ", err)
		return
	}
	if err := dev.BindSetMark(uint32(wgProtectMark)); err != nil {
		ERROR("restore wireguard fwmark after sticky clear: ", err)
	}
}
