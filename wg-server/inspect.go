package wgserver

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"

	"golang.zx2c4.com/wireguard/tun"
)

// aclControlPort is the UDP destination port that peers use to send ACL
// updates to the wg-server's own WG-side IP. Packets matching this port are
// consumed by the inspector and never reach the kernel.
const (
	aclControlPort      = 51821
	aclMaxAllowed       = 1024
	aclMaxPayload       = 65536 // hard cap on JSON payload bytes
	protoUDP       byte = 17
)

// inspectingTUN sits between wireguard-go and the kernel TUN device. It is
// always installed, regardless of the EnableFirewall setting.
//
// On Write (decrypted peer packets entering the kernel) it:
//   - consumes ACL-control UDP packets addressed to the server's WG IP,
//   - drops any other packet addressed to the server's WG IP,
//   - drops peer-to-peer packets disallowed by the firewall (when enabled),
//   - passes everything else through.
//
// On Read (kernel packets leaving toward WG for encryption) it applies the
// same filtering. This catches cross-server peer-to-peer traffic that
// arrives via the InternetIface and is routed onto the WG interface.
//
// The per-peer allowlist + connection-tracking state lives in the package
// peer list (peerlist.go), keyed by local-subnet address; the inspector only
// classifies packets and consults it.
type inspectingTUN struct {
	tun.Device
	firewall   bool // when false, peer-to-peer traffic is not policy-checked
	subnet4    netip.Prefix
	subnet6    netip.Prefix
	serverIPv4 netip.Addr
	serverIPv6 netip.Addr
}

func newInspectingTUN(inner tun.Device, cfg *Config) (*inspectingTUN, error) {
	t := &inspectingTUN{Device: inner, firewall: cfg.EnableFirewall}

	if cfg.WireGuardSubnet != "" {
		p, err := netip.ParsePrefix(cfg.WireGuardSubnet)
		if err != nil {
			return nil, fmt.Errorf("parse WireGuardSubnet: %w", err)
		}
		t.subnet4 = p.Masked()
		t.serverIPv4 = p.Addr().Next()
	}
	if cfg.WireGuardSubnet6 != "" {
		p, err := netip.ParsePrefix(cfg.WireGuardSubnet6)
		if err != nil {
			return nil, fmt.Errorf("parse WireGuardSubnet6: %w", err)
		}
		t.subnet6 = p.Masked()
		t.serverIPv6 = p.Addr().Next()
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
		if t.handleControl(pkt) {
			continue
		}
		if t.allow(pkt) {
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

// allow returns true if pkt should be forwarded. It filters traffic where at
// least one endpoint is a local WG resident; traffic with neither end local
// (internet egress, malformed) passes through.
//
// The server's own WG IP is never reachable by peers — this holds whether or
// not the firewall is enabled. The only packets a peer may address to it are
// ACL control messages, which handleControl consumes before allow runs.
// Server-originated traffic (ICMP errors for PMTU discovery, etc.) still
// passes.
//
// When the firewall is on:
//   - a local sender's outbound packet records a flow, opening its own return
//     path (connection tracking) — done even toward another server so the
//     reply is admitted there;
//   - a local receiver's inbound packet is admitted iff the source is in its
//     allowlist or matches a flow it opened (default-deny otherwise).
//
// With the firewall off, peer-to-peer traffic passes freely (the server-IP
// block above still applies).
func (t *inspectingTUN) allow(pkt []byte) bool {
	src, dst, proto, l4, ok := parseIPHeader(pkt)
	if !ok {
		return true
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

	// A local sender opens (or refreshes) its own return path.
	if srcLocal && srcPeer != nil {
		srcPeer.touchFlow(flowKey{remote: dst, rport: dport, lport: sport, proto: proto})
	}

	if !t.firewall {
		return true
	}

	// A local receiver: allowlist, or a reply to a flow it opened.
	if dstLocal {
		if dstPeer == nil {
			return false // in our subnet but nobody connected — default-deny
		}
		if dstPeer.allowedContains(src) {
			return true
		}
		return dstPeer.flowMatch(flowKey{remote: src, rport: sport, lport: dport, proto: proto})
	}
	// src is local, dst is on another server — that server enforces ingress.
	return true
}

func (t *inspectingTUN) isServerIP(a netip.Addr) bool {
	return a == t.serverIPv4 || a == t.serverIPv6
}

// handleControl returns true if pkt was a control message that we consumed.
// The caller MUST NOT forward such a packet to the kernel.
func (t *inspectingTUN) handleControl(pkt []byte) bool {
	src, dst, proto, l4, ok := parseIPHeader(pkt)
	if !ok || proto != protoUDP {
		return false
	}
	if dst != t.serverIPv4 && dst != t.serverIPv6 {
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
		return true // consume but ignore — wrong source
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
	// Entries must parse as IP addresses; invalid ones are skipped without
	// feedback — the control channel is fire-and-forget. No subnet check:
	// peers can communicate across wg-servers, so an allowed IP may belong
	// to another server's WG subnet.
	srcs := make([]netip.Addr, 0, len(msg.Allowed))
	for _, s := range msg.Allowed {
		a, err := netip.ParseAddr(s)
		if err != nil {
			continue
		}
		srcs = append(srcs, a)
	}
	// The announcement comes from a local resident; apply it to that peer's
	// entry. An empty list with AllowAll=false clears the policy (replace-set
	// semantics), so a disconnecting peer removes itself. If no entry exists
	// yet (announce raced the handshake), drop it — the client re-announces on
	// a short retry.
	p, local := fwClassify(src)
	if !local || p == nil {
		return
	}
	p.setAllowed(srcs, msg.AllowAll)
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

// parseIPHeader extracts addressing info from an IPv4 or IPv6 packet.
// Returns ok=false for malformed or unsupported packets. Extension headers
// on IPv6 are not parsed — the next-header byte is reported verbatim, which
// is sufficient for the simple UDP case.
func parseIPHeader(pkt []byte) (src, dst netip.Addr, proto byte, l4 []byte, ok bool) {
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
		proto = pkt[6]
		l4 = pkt[40 : 40+payloadLen]
		ok = true
		return
	}
	return
}
