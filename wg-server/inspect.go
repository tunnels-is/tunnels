package wgserver

import (
	"encoding/binary"
	"encoding/json"
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

func (t *inspectingTUN) applyControl(src netip.Addr, payload []byte) {
	if len(payload) == 0 || len(payload) > aclMaxPayload {
		return
	}
	var msg struct {
		AllowAll bool     `json:"AllowAll"`
		Allowed  []string `json:"Allowed"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		return
	}
	if len(msg.Allowed) > aclMaxAllowed {
		return
	}

	entries := make([]aclEntry, 0, len(msg.Allowed))
	for _, s := range msg.Allowed {
		if e, ok := parseACLEntry(s); ok {
			entries = append(entries, e)
		}
	}

	p, local := fwClassify(src)
	if !local || p == nil {
		return
	}
	p.setAllowed(entries, msg.AllowAll)
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

type fragInfo struct {
	id     uint32
	offset uint16
	more   bool
}

func (f fragInfo) isFragment() bool { return f.more || f.offset != 0 }

func (f fragInfo) isTrailing() bool { return f.offset != 0 }

func parseIPHeader(pkt []byte) (src, dst netip.Addr, proto byte, l4 []byte, frag fragInfo, ok bool) {
	if len(pkt) < 1 {
		return
	}
	switch pkt[0] >> 4 {
	case 4:
		if len(pkt) < 20 {
			return
		}
		ihl := int(pkt[0]&0x0F) * 4
		if ihl < 20 || len(pkt) < ihl {
			return
		}
		total := int(binary.BigEndian.Uint16(pkt[2:4]))
		if total < ihl || total > len(pkt) {
			return
		}
		var s, d [4]byte
		copy(s[:], pkt[12:16])
		copy(d[:], pkt[16:20])
		src = netip.AddrFrom4(s)
		dst = netip.AddrFrom4(d)
		proto = pkt[9]
		l4 = pkt[ihl:total]
		ff := binary.BigEndian.Uint16(pkt[6:8])
		frag = fragInfo{
			id:     uint32(binary.BigEndian.Uint16(pkt[4:6])),
			offset: ff & 0x1FFF,
			more:   ff&0x2000 != 0,
		}
		ok = true
		return
	case 6:
		if len(pkt) < 40 {
			return
		}
		payloadLen := int(binary.BigEndian.Uint16(pkt[4:6]))
		if 40+payloadLen > len(pkt) {
			return
		}
		var s, d [16]byte
		copy(s[:], pkt[8:24])
		copy(d[:], pkt[24:40])
		src = netip.AddrFrom16(s)
		dst = netip.AddrFrom16(d)

		next := pkt[6]
		off := 40
		end := 40 + payloadLen
	extLoop:
		for {
			switch next {
			case 0, 43, 60:
				if off+8 > end {
					return
				}
				hlen := 8 + int(pkt[off+1])*8
				if off+hlen > end {
					return
				}
				next = pkt[off]
				off += hlen
			case 44:
				if off+8 > end {
					return
				}
				fo := binary.BigEndian.Uint16(pkt[off+2 : off+4])
				frag = fragInfo{
					id:     binary.BigEndian.Uint32(pkt[off+4 : off+8]),
					offset: fo >> 3,
					more:   fo&0x1 != 0,
				}
				next = pkt[off]
				off += 8
			default:
				break extLoop
			}
		}
		proto = next
		l4 = pkt[off:end]
		ok = true
		return
	}
	return
}
