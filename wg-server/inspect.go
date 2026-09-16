package wgserver

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"os"
	"sync/atomic"

	"golang.zx2c4.com/wireguard/tun"
)

const (
	aclControlPort      = 51821
	aclMaxAllowed       = 1024
	aclMaxPayload       = 65536
	protoUDP       byte = 17
)

var inspectDevice atomic.Pointer[inspectingTUN]

type inspectingTUN struct {
	tun.Device
	firewall   atomic.Bool
	subnet4    netip.Prefix
	subnet6    netip.Prefix
	serverIPv4 netip.Addr
	serverIPv6 netip.Addr
}

func newInspectingTUN(inner tun.Device, cfg *Config) (*inspectingTUN, error) {
	t := &inspectingTUN{Device: inner}
	t.firewall.Store(cfg.EnableFirewall)

	if cfg.WireGuardSubnet != "" {
		p, err := netip.ParsePrefix(cfg.WireGuardSubnet)
		if err != nil {
			return nil, fmt.Errorf("parse WireGuardSubnet: %w", err)
		}
		t.subnet4 = p.Masked()
		t.serverIPv4 = t.subnet4.Addr().Next()
	}
	if cfg.WireGuardSubnet6 != "" {
		p, err := netip.ParsePrefix(cfg.WireGuardSubnet6)
		if err != nil {
			return nil, fmt.Errorf("parse WireGuardSubnet6: %w", err)
		}
		t.subnet6 = p.Masked()
		t.serverIPv6 = t.subnet6.Addr().Next()
	}
	if !t.subnet4.IsValid() && !t.subnet6.IsValid() {
		return nil, fmt.Errorf("inspector requires at least one WireGuard subnet")
	}
	return t, nil
}

func (t *inspectingTUN) Write(bufs [][]byte, offset int) (int, error) {
	kept := bufs[:0]
	for _, buf := range bufs {
		pkt := buf[offset:]

		src, dst, proto, l4, frag, ok := parseIPHeader(pkt)
		if t.handleControlParsed(src, dst, proto, l4, frag, ok) {
			continue
		}
		if t.allowParsed(src, dst, proto, l4, frag, ok) {
			kept = append(kept, buf)
		}
	}
	if len(kept) == 0 {
		return 0, nil
	}
	return t.Device.Write(kept, offset)
}

func (t *inspectingTUN) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	n, err := t.Device.Read(bufs, sizes, offset)
	if err != nil {
		return n, err
	}
	out := 0
	for i := 0; i < n; i++ {
		if t.allow(bufs[i][offset : offset+sizes[i]]) {
			if out != i {
				copy(bufs[out][offset:offset+sizes[i]], bufs[i][offset:offset+sizes[i]])
				sizes[out] = sizes[i]
			}
			out++
		}
	}
	return out, nil
}

func (t *inspectingTUN) File() *os.File {
	return t.Device.File()
}

func (t *inspectingTUN) allow(pkt []byte) bool {
	src, dst, proto, l4, frag, ok := parseIPHeader(pkt)
	return t.allowParsed(src, dst, proto, l4, frag, ok)
}

func (t *inspectingTUN) allowParsed(src, dst netip.Addr, proto byte, l4 []byte, frag fragInfo, ok bool) bool {
	if !ok {
		return false
	}
	if frag.isFragment() {
		return false
	}
	srcPeer, srcLocal := fwClassify(src)
	dstPeer, dstLocal := fwClassify(dst)
	if !srcLocal && !dstLocal {
		return true
	}
	if t.isServerIP(dst) {
		return false
	}
	if t.isServerIP(src) {
		return true
	}

	sport, dport := l4Ports(proto, l4)

	admit := false
	switch {
	case !t.firewall.Load():
		admit = true
	case dstLocal:
		if dstPeer == nil {
			admit = false
			break
		}
		admit = dstPeer.allowedContains(src, dport) ||
			dstPeer.flowMatch(flowKey{remote: src, rport: sport, lport: dport, proto: proto})
	default:
		admit = true
	}

	if admit && srcLocal && srcPeer != nil {
		srcPeer.touchFlow(flowKey{remote: dst, rport: dport, lport: sport, proto: proto})
	}
	return admit
}

func (t *inspectingTUN) isServerIP(a netip.Addr) bool {
	return a == t.serverIPv4 || a == t.serverIPv6
}

func (t *inspectingTUN) handleControlParsed(src, dst netip.Addr, proto byte, l4 []byte, frag fragInfo, ok bool) bool {
	if !ok || proto != protoUDP {
		return false
	}
	if dst != t.serverIPv4 && dst != t.serverIPv6 {
		return false
	}

	if frag.isFragment() {
		return false
	}
	if len(l4) < 8 {
		return false
	}
	dport := binary.BigEndian.Uint16(l4[2:4])
	if dport != aclControlPort {
		return false
	}
	if !t.inWGSubnet(src) {
		return true
	}
	payload := l4[8:]
	t.applyControl(src, payload)
	return true
}

func (t *inspectingTUN) inWGSubnet(a netip.Addr) bool {
	if a.Is4() && t.subnet4.IsValid() {
		return t.subnet4.Contains(a)
	}
	if a.Is6() && t.subnet6.IsValid() {
		return t.subnet6.Contains(a)
	}
	return false
}
