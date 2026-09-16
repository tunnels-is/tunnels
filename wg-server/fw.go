package wgserver

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"
)

const (
	flowCleanInterval = 15 * time.Minute

	flowSoftCap = 50000
)

type flowKey struct {
	remote netip.Addr
	rport  uint16
	lport  uint16
	proto  byte
}

type flowRec struct {
	packets atomic.Uint64
	prev    uint64
}

type fragKey struct {
	remote netip.Addr
	id     uint32
}

type fwPeer struct {
	mu       sync.RWMutex
	v6       netip.Addr
	allowAll bool
	allowed  map[netip.Addr]*portSet
	anyPorts map[uint16]struct{}
	flows    map[flowKey]*flowRec

	frags map[fragKey]*flowRec
}

var (
	fwSubnet4 netip.Prefix
	fwBase4   uint32
	fwSubnet6 netip.Prefix

	fwV4Slots []atomic.Pointer[fwPeer]

	fwMu sync.RWMutex
	fwV6 map[netip.Addr]*fwPeer

	fwStop chan struct{}
)

func initFirewall(subnet4, subnet6 string) error {
	p4, err := netip.ParsePrefix(subnet4)
	if err != nil {
		return fmt.Errorf("parse WireGuard subnet: %w", err)
	}
	p4 = p4.Masked()
	if !p4.Addr().Is4() {
		return fmt.Errorf("WireGuard subnet must be IPv4: %s", subnet4)
	}
	if p4.Bits() < 16 {
		return fmt.Errorf("WireGuard subnet larger than /16 is unsupported: %s", subnet4)
	}
	fwSubnet4 = p4
	fwBase4 = binary.BigEndian.Uint32(p4.Addr().AsSlice())
	fwV4Slots = make([]atomic.Pointer[fwPeer], 1<<(32-p4.Bits()))

	fwMu.Lock()
	fwV6 = make(map[netip.Addr]*fwPeer)
	fwMu.Unlock()

	if subnet6 != "" {
		p6, err := netip.ParsePrefix(subnet6)
		if err != nil {
			return fmt.Errorf("parse WireGuard subnet6: %w", err)
		}
		fwSubnet6 = p6.Masked()
	} else {
		fwSubnet6 = netip.Prefix{}
	}
	return nil
}

func startFlowCleaner() {
	fwStop = make(chan struct{})
	go func() {
		t := time.NewTicker(flowCleanInterval)
		defer t.Stop()
		for {
			select {
			case <-fwStop:
				return
			case <-t.C:
				cleanFlows()
			}
		}
	}()
}

func stopFlowCleaner() {
	if fwStop != nil {
		close(fwStop)
		fwStop = nil
	}
}

func v4Offset(a netip.Addr) (uint32, bool) {
	if !fwSubnet4.IsValid() || !a.Is4() || !fwSubnet4.Contains(a) {
		return 0, false
	}
	return binary.BigEndian.Uint32(a.AsSlice()) - fwBase4, true
}

func fwClassify(a netip.Addr) (*fwPeer, bool) {
	if a.Is4() {
		off, ok := v4Offset(a)
		if !ok {
			return nil, false
		}
		return fwV4Slots[off].Load(), true
	}
	if fwSubnet6.IsValid() && fwSubnet6.Contains(a) {
		fwMu.RLock()
		p := fwV6[a]
		fwMu.RUnlock()
		return p, true
	}
	return nil, false
}

func dropPeer(ips ...string) {
	if fwV4Slots == nil {
		return
	}
	for _, s := range ips {
		a, err := netip.ParseAddr(s)
		if err != nil {
			continue
		}
		if a.Is4() {
			off, ok := v4Offset(a)
			if !ok {
				continue
			}
			old := fwV4Slots[off].Swap(nil)
			if old != nil && old.v6.IsValid() {
				fwMu.Lock()
				if fwV6[old.v6] == old {
					delete(fwV6, old.v6)
				}
				fwMu.Unlock()
			}
			continue
		}
		if a.Is6() {
			fwMu.Lock()
			delete(fwV6, a)
			fwMu.Unlock()
		}
	}
}

func resetPeer(ips ...string) {
	if fwV4Slots == nil {
		return
	}
	p := &fwPeer{
		allowed:  make(map[netip.Addr]*portSet),
		anyPorts: make(map[uint16]struct{}),
		flows:    make(map[flowKey]*flowRec),
		frags:    make(map[fragKey]*flowRec),
	}
	var v4 netip.Addr
	for _, s := range ips {
		a, err := netip.ParseAddr(s)
		if err != nil {
			continue
		}
		switch {
		case a.Is4():
			v4 = a
		case a.Is6() && fwSubnet6.IsValid() && fwSubnet6.Contains(a):
			p.v6 = a
		}
	}

	if v4.IsValid() {
		off, ok := v4Offset(v4)
		if ok {
			old := fwV4Slots[off].Swap(p)

			if old != nil && old.v6.IsValid() {
				fwMu.Lock()
				if fwV6[old.v6] == old {
					delete(fwV6, old.v6)
				}
				fwMu.Unlock()
			}
		}
	}
	if p.v6.IsValid() {
		fwMu.Lock()
		fwV6[p.v6] = p
		fwMu.Unlock()
	}
}
