package wgserver

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func handleControl(t *inspectingTUN, pkt []byte) bool {
	src, dst, proto, l4, frag, ok := parseIPHeader(pkt)
	return t.handleControlParsed(src, dst, proto, l4, frag, ok)
}

func peerListSnapshot() map[netip.Addr][]netip.Addr {
	out := make(map[netip.Addr][]netip.Addr)
	for i := range fwV4Slots {
		p := fwV4Slots[i].Load()
		if p == nil {
			continue
		}
		p.mu.RLock()
		if len(p.allowed) > 0 {
			srcs := make([]netip.Addr, 0, len(p.allowed))
			for src := range p.allowed {
				srcs = append(srcs, src)
			}
			var b [4]byte
			binary.BigEndian.PutUint32(b[:], fwBase4+uint32(i))
			out[netip.AddrFrom4(b)] = srcs
		}
		p.mu.RUnlock()
	}
	return out
}

func buildIPv4UDP(t *testing.T, src, dst string, sport, dport uint16, payload []byte) []byte {
	t.Helper()
	srcA := mustAddr(t, src).As4()
	dstA := mustAddr(t, dst).As4()
	total := 20 + 8 + len(payload)

	pkt := make([]byte, total)
	pkt[0] = 0x45
	binary.BigEndian.PutUint16(pkt[2:4], uint16(total))
	pkt[9] = protoUDP
	copy(pkt[12:16], srcA[:])
	copy(pkt[16:20], dstA[:])

	udp := pkt[20:]
	binary.BigEndian.PutUint16(udp[0:2], sport)
	binary.BigEndian.PutUint16(udp[2:4], dport)
	binary.BigEndian.PutUint16(udp[4:6], uint16(8+len(payload)))
	copy(udp[8:], payload)
	return pkt
}

func buildIPv6UDP(t *testing.T, src, dst string, sport, dport uint16, payload []byte) []byte {
	t.Helper()
	srcA := mustAddr(t, src).As16()
	dstA := mustAddr(t, dst).As16()
	payloadLen := 8 + len(payload)

	pkt := make([]byte, 40+payloadLen)
	pkt[0] = 0x60
	binary.BigEndian.PutUint16(pkt[4:6], uint16(payloadLen))
	pkt[6] = protoUDP
	copy(pkt[8:24], srcA[:])
	copy(pkt[24:40], dstA[:])

	udp := pkt[40:]
	binary.BigEndian.PutUint16(udp[0:2], sport)
	binary.BigEndian.PutUint16(udp[2:4], dport)
	binary.BigEndian.PutUint16(udp[4:6], uint16(payloadLen))
	copy(udp[8:], payload)
	return pkt
}

func setFrag(pkt []byte, id, offsetUnits uint16, more bool) {
	binary.BigEndian.PutUint16(pkt[4:6], id)
	ff := offsetUnits & 0x1FFF
	if more {
		ff |= 0x2000
	}
	binary.BigEndian.PutUint16(pkt[6:8], ff)
}

func TestParseIPHeader_IPv4UDP(t *testing.T) {
	pkt := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1234, 5678, []byte("hi"))
	src, dst, proto, l4, frag, ok := parseIPHeader(pkt)
	if !ok {
		t.Fatal("expected ok")
	}
	if src != mustAddr(t, "10.0.0.5") || dst != mustAddr(t, "10.0.0.10") {
		t.Fatalf("addr mismatch: src=%v dst=%v", src, dst)
	}
	if proto != protoUDP {
		t.Fatalf("expected UDP, got %d", proto)
	}
	if binary.BigEndian.Uint16(l4[0:2]) != 1234 || binary.BigEndian.Uint16(l4[2:4]) != 5678 {
		t.Fatalf("l4 ports wrong")
	}
	if frag.isFragment() {
		t.Fatalf("a normal packet must not be flagged as a fragment")
	}
}

func TestParseIPHeader_IPv6UDP(t *testing.T) {
	pkt := buildIPv6UDP(t, "fd00::5", "fd00::10", 1234, 5678, []byte("hi"))
	src, dst, proto, _, _, ok := parseIPHeader(pkt)
	if !ok {
		t.Fatal("expected ok")
	}
	if src != mustAddr(t, "fd00::5") || dst != mustAddr(t, "fd00::10") {
		t.Fatalf("addr mismatch")
	}
	if proto != protoUDP {
		t.Fatalf("proto wrong")
	}
}

func TestParseIPHeader_TooShort(t *testing.T) {
	for _, n := range []int{0, 1, 10, 19} {
		if _, _, _, _, _, ok := parseIPHeader(make([]byte, n)); ok {
			t.Fatalf("len=%d should be invalid", n)
		}
	}
}

func TestParseIPHeader_BadVersion(t *testing.T) {
	pkt := make([]byte, 40)
	pkt[0] = 0x70
	if _, _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("bad version must be rejected")
	}
}

func TestParseIPHeader_BadIHL(t *testing.T) {
	pkt := make([]byte, 20)
	pkt[0] = 0x40
	if _, _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("IHL<5 must be rejected")
	}
}

func TestParseIPHeader_BadTotalLen(t *testing.T) {
	pkt := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, []byte("x"))
	binary.BigEndian.PutUint16(pkt[2:4], 9999)
	if _, _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("total>len must be rejected")
	}
}

func newTestInspector(t *testing.T) *inspectingTUN {
	t.Helper()
	setupFW(t, "10.0.0.0/24", "fd00::/64")
	insp, err := newInspectingTUN(nil, &Config{
		WireGuardSubnet:  "10.0.0.0/24",
		WireGuardSubnet6: "fd00::/64",
		EnableFirewall:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return insp
}

func newTestInspectorNoFirewall(t *testing.T) *inspectingTUN {
	t.Helper()
	setupFW(t, "10.0.0.0/24", "fd00::/64")
	insp, err := newInspectingTUN(nil, &Config{
		WireGuardSubnet:  "10.0.0.0/24",
		WireGuardSubnet6: "fd00::/64",
		EnableFirewall:   false,
	})
	if err != nil {
		t.Fatal(err)
	}
	return insp
}

func allowFor(t *testing.T, dst string, srcs ...string) {
	t.Helper()
	resetPeer(dst)
	entry(t, dst).setAllowed(hostRules(t, srcs...), false)
}

func TestAllow_DefaultDenyForPeerToPeer(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")
	resetPeer("10.0.0.10")
	pkt := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, nil)
	if insp.allow(pkt) {
		t.Fatal("no policy => peer-to-peer must be denied")
	}
}

func TestAllow_ServerIPUnreachableByPeers(t *testing.T) {
	insp := newTestInspector(t)

	toServer := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, 2, nil)
	if insp.allow(toServer) {
		t.Fatal("traffic to server WG IP must be dropped")
	}
	toServer6 := buildIPv6UDP(t, "fd00::5", insp.serverIPv6.String(), 1, 2, nil)
	if insp.allow(toServer6) {
		t.Fatal("v6 traffic to server WG IP must be dropped")
	}
}

func TestAllow_FirewallDisabled(t *testing.T) {
	insp := newTestInspectorNoFirewall(t)

	p2p := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, nil)
	if !insp.allow(p2p) {
		t.Fatal("firewall disabled => peer-to-peer must pass")
	}

	toServer := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, 2, nil)
	if insp.allow(toServer) {
		t.Fatal("server WG IP must be blocked even with firewall disabled")
	}
	toServer6 := buildIPv6UDP(t, "fd00::5", insp.serverIPv6.String(), 1, 2, nil)
	if insp.allow(toServer6) {
		t.Fatal("v6 server WG IP must be blocked even with firewall disabled")
	}
}

func TestAllow_ServerOriginatedPasses(t *testing.T) {
	insp := newTestInspector(t)

	fromServer := buildIPv4UDP(t, insp.serverIPv4.String(), "10.0.0.5", 1, 2, nil)
	if !insp.allow(fromServer) {
		t.Fatal("traffic from server WG IP must not be filtered")
	}
}

func TestAllow_AllowlistedSrcPasses(t *testing.T) {
	insp := newTestInspector(t)
	allowFor(t, "10.0.0.10", "10.0.0.5")

	allowed := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, nil)
	denied := buildIPv4UDP(t, "10.0.0.99", "10.0.0.10", 1, 2, nil)

	if !insp.allow(allowed) {
		t.Fatal("allowed src should pass")
	}
	if insp.allow(denied) {
		t.Fatal("unlisted src should be dropped")
	}
}

func TestAllow_RevokeDropsConntrackFlow(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.3")
	allowFor(t, "10.0.0.2", "10.0.0.3")

	if !insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil)) {
		t.Fatal("setup: B should reach A while allowlisted")
	}
	insp.allow(buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 5000, nil))

	entry(t, "10.0.0.2").setAllowed(nil, false)

	if insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil)) {
		t.Fatal("revoked peer must be dropped, not coast on the conntrack flow")
	}
}

func TestAllow_RevokeKeepsOwnOutboundFlow(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.2")

	insp.allow(buildIPv4UDP(t, "10.0.0.2", "10.99.0.9", 40000, 80, nil))
	if !insp.allow(buildIPv4UDP(t, "10.99.0.9", "10.0.0.2", 80, 40000, nil)) {
		t.Fatal("setup: C's reply should match A's own outbound flow")
	}

	entry(t, "10.0.0.2").setAllowed(hostRules(t, "10.0.0.5"), false)
	if !insp.allow(buildIPv4UDP(t, "10.99.0.9", "10.0.0.2", 80, 40000, nil)) {
		t.Fatal("policy change must not drop A's own initiated-outbound return path")
	}
}

func TestAllow_RevokeReestablishesOnOutbound(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.3")
	allowFor(t, "10.0.0.2", "10.0.0.3")

	insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil))
	insp.allow(buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 5000, nil))
	entry(t, "10.0.0.2").setAllowed(nil, false)

	if insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil)) {
		t.Fatal("B must be cut off immediately after revoke")
	}

	insp.allow(buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 5000, nil))
	if !insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil)) {
		t.Fatal("A's own outbound must re-establish the return path for B's replies")
	}
}

func TestAllow_HostPortRule(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")

	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.5:22"]}`))
	if !handleControl(insp, ctrl) {
		t.Fatal("expected consume")
	}

	onPort := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 40000, 22, nil)
	if !insp.allow(onPort) {
		t.Fatal("allowed src on its permitted port must pass")
	}
	otherPort := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 40000, 80, nil)
	if insp.allow(otherPort) {
		t.Fatal("allowed src on a non-permitted port must be dropped")
	}
}

func TestAllow_AnyHostPortRule(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")

	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["*:443"]}`))
	if !handleControl(insp, ctrl) {
		t.Fatal("expected consume")
	}

	for _, src := range []string{"10.0.0.5", "10.0.0.99"} {
		ok := buildIPv4UDP(t, src, "10.0.0.10", 40000, 443, nil)
		if !insp.allow(ok) {
			t.Fatalf("*:443 must admit %s on port 443", src)
		}
		bad := buildIPv4UDP(t, src, "10.0.0.10", 40000, 22, nil)
		if insp.allow(bad) {
			t.Fatalf("*:443 must not admit %s on port 22", src)
		}
	}
}

func TestParseIPHeader_Fragments(t *testing.T) {
	head := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, make([]byte, 8))
	setFrag(head, 4242, 0, true)
	_, _, _, _, frag, ok := parseIPHeader(head)
	if !ok || frag.id != 4242 || !frag.isFragment() || frag.isTrailing() {
		t.Fatalf("first fragment: got id=%d isFragment=%v isTrailing=%v", frag.id, frag.isFragment(), frag.isTrailing())
	}

	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, make([]byte, 8))
	setFrag(tail, 4242, 185, false)
	_, _, _, _, frag, ok = parseIPHeader(tail)
	if !ok || frag.id != 4242 || !frag.isFragment() || !frag.isTrailing() {
		t.Fatalf("trailing fragment: got id=%d isFragment=%v isTrailing=%v", frag.id, frag.isFragment(), frag.isTrailing())
	}
}

func TestAllow_FragmentsAlwaysDropped(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")
	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.5:5000"]}`))
	if !handleControl(insp, ctrl) {
		t.Fatal("expected consume")
	}

	head := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 40000, 5000, make([]byte, 8))
	setFrag(head, 42, 0, true)
	if insp.allow(head) {
		t.Fatal("IPv4 fragments must be dropped")
	}

	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 12345, 6789, make([]byte, 8))
	setFrag(tail, 42, 185, false)
	if insp.allow(tail) {
		t.Fatal("trailing IPv4 fragments must be dropped")
	}
}

func TestAllow_OrphanTrailingFragmentDropped(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")
	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.5:5000"]}`))
	if !handleControl(insp, ctrl) {
		t.Fatal("expected consume")
	}

	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 5000, make([]byte, 8))
	setFrag(tail, 99, 185, false)
	if insp.allow(tail) {
		t.Fatal("orphan trailing fragment must be dropped even if its bytes look like an allowed port")
	}
}

func TestAllow_FragmentedToDeniedPortDropped(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")
	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.5:5000"]}`))
	if !handleControl(insp, ctrl) {
		t.Fatal("expected consume")
	}
	head := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 40000, 22, make([]byte, 8))
	setFrag(head, 7, 0, true)
	if insp.allow(head) {
		t.Fatal("first fragment to a denied port must be dropped")
	}
	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 40000, 22, make([]byte, 8))
	setFrag(tail, 7, 185, false)
	if insp.allow(tail) {
		t.Fatal("trailing fragment of a denied datagram must be dropped")
	}
}

func TestAllow_FragmentBareHostRule(t *testing.T) {
	insp := newTestInspector(t)
	allowFor(t, "10.0.0.10", "10.0.0.5")

	head := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 40000, 5000, make([]byte, 8))
	setFrag(head, 5, 0, true)
	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 0, 0, make([]byte, 8))
	setFrag(tail, 5, 185, false)
	if insp.allow(head) || insp.allow(tail) {
		t.Fatal("fragments must be dropped even for bare-host rules")
	}
}

func TestAllow_FragmentBareHostOrderIndependent(t *testing.T) {
	insp := newTestInspector(t)
	allowFor(t, "10.0.0.10", "10.0.0.5")

	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 0, 0, make([]byte, 8))
	setFrag(tail, 33, 185, false)
	if insp.allow(tail) {
		t.Fatal("headless fragments must be dropped")
	}

	resetPeer("10.0.0.11")
	entry(t, "10.0.0.11").setAllowed(nil, true)
	tail2 := buildIPv4UDP(t, "10.0.0.99", "10.0.0.11", 0, 0, make([]byte, 8))
	setFrag(tail2, 34, 185, false)
	if insp.allow(tail2) {
		t.Fatal("allow-all still drops fragments")
	}
}

func TestAllow_FragmentPortRuleNeedsHead(t *testing.T) {
	insp := newTestInspector(t)
	setRule("10.0.0.10", "10.0.0.5:5000")

	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 0, 0, make([]byte, 8))
	setFrag(tail, 55, 185, false)
	if insp.allow(tail) {
		t.Fatal("port-scoped rule: a trailing fragment with no admitted head must be dropped")
	}
}

func TestAllow_FragmentConntrackReturn(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.2")
	resetPeer("10.0.0.3")

	out := buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 40000, 9000, nil)
	insp.allow(out)

	head := buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 9000, 40000, make([]byte, 8))
	setFrag(head, 77, 0, true)
	if insp.allow(head) {
		t.Fatal("fragmented replies must be dropped")
	}
	tail := buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 0, 0, make([]byte, 8))
	setFrag(tail, 77, 185, false)
	if insp.allow(tail) {
		t.Fatal("trailing fragmented replies must be dropped")
	}
}

func TestInspector_ServerIPUsesMaskedPrefix(t *testing.T) {
	setupFW(t, "10.0.0.0/22", "")
	insp, err := newInspectingTUN(nil, &Config{
		WireGuardSubnet: "10.0.0.5/22",
		EnableFirewall:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if insp.serverIPv4.String() != "10.0.0.1" {
		t.Fatalf("server IPv4 = %s, want 10.0.0.1", insp.serverIPv4)
	}
}

func TestAllow_ConntrackReturnPath(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.3")
	allowFor(t, "10.0.0.2", "10.0.0.3")

	syn := buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 40000, 22, nil)
	if !insp.allow(syn) {
		t.Fatal("SYN to an allowlisted peer must pass")
	}

	synack := buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 40000, nil)
	if !insp.allow(synack) {
		t.Fatal("reply must be admitted by connection tracking")
	}
}

func TestAllow_ConntrackNoUnsolicitedReturn(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.3")
	allowFor(t, "10.0.0.2", "10.0.0.3")

	unsolicited := buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 40000, nil)
	if insp.allow(unsolicited) {
		t.Fatal("unsolicited packet must be denied without a tracked flow")
	}
}

func TestAllow_AllowAllAdmitsAnySource(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")
	entry(t, "10.0.0.10").setAllowed(nil, true)

	for _, src := range []string{"10.0.0.5", "10.0.0.99"} {
		pkt := buildIPv4UDP(t, src, "10.0.0.10", 1, 2, nil)
		if !insp.allow(pkt) {
			t.Fatalf("allow-all must admit %s", src)
		}
	}
}

func TestHandleControl_AllowAll(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"AllowAll":true,"Allowed":[]}`))
	if !handleControl(insp, pkt) {
		t.Fatal("expected consume")
	}
	if !entry(t, "10.0.0.5").allowedContains(mustAddr(t, "10.0.0.123"), 80) {
		t.Fatal("AllowAll announcement must permit any source")
	}
}

func TestAllow_NonPeerToPeerAlwaysPasses(t *testing.T) {
	insp := newTestInspector(t)
	allowFor(t, "10.0.0.10")

	pkt := buildIPv4UDP(t, "10.0.0.5", "8.8.8.8", 1, 2, nil)
	if !insp.allow(pkt) {
		t.Fatal("egress to internet must not be filtered")
	}
}

func TestAllow_MalformedDropped(t *testing.T) {
	insp := newTestInspector(t)
	if insp.allow([]byte{0x00, 0x00}) {
		t.Fatal("malformed packet must be dropped")
	}
}

func TestAllow_ConntrackOnlyAfterAdmit(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.2")
	resetPeer("10.0.0.3")
	allowFor(t, "10.0.0.3", "10.0.0.99")

	denied := buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 40000, 53, nil)
	if insp.allow(denied) {
		t.Fatal("A→B on a closed port must drop")
	}
	reply := buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 53, 40000, nil)
	if insp.allow(reply) {
		t.Fatal("B must not open a return flow from a dropped attempt")
	}
}

func TestAllow_IPv6(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10", "fd00::10")
	entry(t, "10.0.0.10").setAllowed(hostRules(t, "fd00::5"), false)

	ok := buildIPv6UDP(t, "fd00::5", "fd00::10", 1, 2, nil)
	bad := buildIPv6UDP(t, "fd00::99", "fd00::10", 1, 2, nil)

	if !insp.allow(ok) {
		t.Fatal("v6 allowed src must pass")
	}
	if insp.allow(bad) {
		t.Fatal("v6 unlisted src must drop")
	}
}

func TestHandleControl_UpdatesACL(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")
	payload := []byte(`{"Allowed":["10.0.0.10","10.0.0.11"]}`)
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 33333, aclControlPort, payload)

	if !handleControl(insp, pkt) {
		t.Fatal("expected packet to be consumed")
	}
	p := entry(t, "10.0.0.5")
	if !p.allowedContains(mustAddr(t, "10.0.0.10"), 80) {
		t.Fatal("10.0.0.10 should be allowed after control message")
	}
	if p.allowedContains(mustAddr(t, "10.0.0.99"), 80) {
		t.Fatal("non-listed src should be denied after control message")
	}
}

func TestHandleControl_IgnoresWrongPort(t *testing.T) {
	insp := newTestInspector(t)
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, 12345, []byte(`{"Allowed":[]}`))
	if handleControl(insp, pkt) {
		t.Fatal("wrong port must not be consumed")
	}
}

func TestHandleControl_IgnoresWrongDst(t *testing.T) {
	insp := newTestInspector(t)
	pkt := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, aclControlPort, []byte(`{"Allowed":[]}`))
	if handleControl(insp, pkt) {
		t.Fatal("packet not addressed to server WG IP must not be consumed")
	}
}

func TestHandleControl_NonUDPIgnored(t *testing.T) {
	insp := newTestInspector(t)
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, nil)
	pkt[9] = 6
	if handleControl(insp, pkt) {
		t.Fatal("non-UDP must not be consumed as control")
	}
}

func TestHandleControl_TrailingFragmentNotControl(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")

	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.10"]}`))
	setFrag(pkt, 1234, 185, false)
	if handleControl(insp, pkt) {
		t.Fatal("a trailing fragment must not be consumed as a control packet")
	}
	if entry(t, "10.0.0.5").allowedContains(mustAddr(t, "10.0.0.10"), 80) {
		t.Fatal("a trailing fragment must not modify firewall policy")
	}
}

func TestHandleControl_FirstFragmentStillControl(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")

	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.10"]}`))
	setFrag(pkt, 1234, 0, true)
	if handleControl(insp, pkt) {
		t.Fatal("fragmented control packets must not be consumed")
	}
}

func TestHandleControl_SrcOutsideWGSubnet(t *testing.T) {
	insp := newTestInspector(t)

	pkt := buildIPv4UDP(t, "8.8.8.8", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":["10.0.0.10"]}`))
	if !handleControl(insp, pkt) {
		t.Fatal("matching dst+port should be consumed even if src is invalid")
	}
	if len(peerListSnapshot()) != 0 {
		t.Fatal("no policy must be stored for an outside-subnet src")
	}
}

func TestHandleControl_BadJSON(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte("not json"))
	if !handleControl(insp, pkt) {
		t.Fatal("malformed payload should still be consumed (not forwarded)")
	}
	if len(peerListSnapshot()) != 0 {
		t.Fatal("no policy should be stored after a bad payload")
	}
}

func TestHandleControl_EmptyListIsolatesSender(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":[]}`))
	if !handleControl(insp, pkt) {
		t.Fatal("expected consume")
	}
	if entry(t, "10.0.0.5").allowedContains(mustAddr(t, "10.0.0.10"), 80) {
		t.Fatal("empty allowlist must isolate the sender")
	}
}

func TestHandleControl_EmptyListClearsPolicy(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")
	set := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":["10.0.0.10"]}`))
	if !handleControl(insp, set) {
		t.Fatal("expected consume")
	}
	if len(peerListSnapshot()) != 1 {
		t.Fatal("setup: policy should be stored")
	}
	clear := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":[]}`))
	if !handleControl(insp, clear) {
		t.Fatal("expected consume")
	}
	if len(peerListSnapshot()) != 0 {
		t.Fatal("empty allowlist must clear the stored policy")
	}
}

func TestHandleControl_CrossServerIPAccepted(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")

	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":["10.99.0.7"]}`))
	if !handleControl(insp, pkt) {
		t.Fatal("expected consume")
	}
	if !entry(t, "10.0.0.5").allowedContains(mustAddr(t, "10.99.0.7"), 80) {
		t.Fatal("cross-server IP must be accepted into the allowlist")
	}
}

func TestHandleControl_AnnounceWithoutEntryDropped(t *testing.T) {
	insp := newTestInspector(t)

	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":["10.0.0.10"]}`))
	if !handleControl(insp, pkt) {
		t.Fatal("expected consume")
	}
	if len(peerListSnapshot()) != 0 {
		t.Fatal("announce without an installed entry must not store a policy")
	}
}

func TestHandleControl_IPv6(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5", "fd00::5")
	payload := []byte(`{"Allowed":["fd00::10"]}`)
	pkt := buildIPv6UDP(t, "fd00::5", insp.serverIPv6.String(), 1, aclControlPort, payload)
	if !handleControl(insp, pkt) {
		t.Fatal("v6 control should be consumed")
	}
	p, _ := fwClassify(mustAddr(t, "fd00::5"))
	if p == nil || !p.allowedContains(mustAddr(t, "fd00::10"), 80) {
		t.Fatal("v6 ACL not applied")
	}
}

func buildIPv6FragUDP(t *testing.T, src, dst string, sport, dport uint16, id uint32, offsetUnits uint16, more bool, payload []byte) []byte {
	t.Helper()
	srcA := mustAddr(t, src).As16()
	dstA := mustAddr(t, dst).As16()

	inner := payload
	if offsetUnits == 0 {
		inner = make([]byte, 8+len(payload))
		binary.BigEndian.PutUint16(inner[0:2], sport)
		binary.BigEndian.PutUint16(inner[2:4], dport)
		binary.BigEndian.PutUint16(inner[4:6], uint16(8+len(payload)))
		copy(inner[8:], payload)
	}

	payloadLen := 8 + len(inner)
	pkt := make([]byte, 40+payloadLen)
	pkt[0] = 0x60
	binary.BigEndian.PutUint16(pkt[4:6], uint16(payloadLen))
	pkt[6] = 44
	copy(pkt[8:24], srcA[:])
	copy(pkt[24:40], dstA[:])

	fh := pkt[40:]
	fh[0] = protoUDP
	fo := offsetUnits << 3
	if more {
		fo |= 0x1
	}
	binary.BigEndian.PutUint16(fh[2:4], fo)
	binary.BigEndian.PutUint32(fh[4:8], id)
	copy(fh[8:], inner)
	return pkt
}

func TestParseIPHeader_IPv6FirstFragment(t *testing.T) {
	pkt := buildIPv6FragUDP(t, "fd00::5", "fd00::10", 1234, 5678, 0xDEADBEEF, 0, true, []byte("hi"))
	src, dst, proto, l4, frag, ok := parseIPHeader(pkt)
	if !ok {
		t.Fatal("expected ok")
	}
	if src != mustAddr(t, "fd00::5") || dst != mustAddr(t, "fd00::10") {
		t.Fatalf("addr mismatch")
	}
	if proto != protoUDP {
		t.Fatalf("expected UDP after fragment header, got %d", proto)
	}
	if sp, dp := l4Ports(proto, l4); sp != 1234 || dp != 5678 {
		t.Fatalf("l4 ports wrong: %d,%d", sp, dp)
	}
	if !frag.isFragment() || frag.isTrailing() {
		t.Fatalf("first fragment misclassified: %+v", frag)
	}
	if frag.id != 0xDEADBEEF {
		t.Fatalf("fragment id wrong: %x", frag.id)
	}
}

func TestParseIPHeader_IPv6TrailingFragment(t *testing.T) {
	pkt := buildIPv6FragUDP(t, "fd00::5", "fd00::10", 0, 0, 7, 10, false, []byte("restofdata"))
	_, _, _, _, frag, ok := parseIPHeader(pkt)
	if !ok {
		t.Fatal("expected ok")
	}
	if !frag.isFragment() || !frag.isTrailing() {
		t.Fatalf("trailing fragment misclassified: %+v", frag)
	}
	if frag.id != 7 {
		t.Fatalf("fragment id wrong: %d", frag.id)
	}
}

func TestParseIPHeader_IPv6HopByHopSkipped(t *testing.T) {

	udp := buildIPv6UDP(t, "fd00::5", "fd00::10", 42, 43, nil)
	inner := udp[40:]
	payloadLen := 8 + len(inner)
	pkt := make([]byte, 40+payloadLen)
	copy(pkt, udp[:40])
	binary.BigEndian.PutUint16(pkt[4:6], uint16(payloadLen))
	pkt[6] = 0
	hbh := pkt[40:]
	hbh[0] = protoUDP
	hbh[1] = 0
	copy(hbh[8:], inner)

	_, _, proto, l4, _, ok := parseIPHeader(pkt)
	if !ok {
		t.Fatal("expected ok")
	}
	if proto != protoUDP {
		t.Fatalf("expected UDP after hop-by-hop, got %d", proto)
	}
	if sp, dp := l4Ports(proto, l4); sp != 42 || dp != 43 {
		t.Fatalf("l4 ports wrong: %d,%d", sp, dp)
	}
}

func TestParseIPHeader_IPv6TruncatedExtHeader(t *testing.T) {
	pkt := buildIPv6UDP(t, "fd00::5", "fd00::10", 1, 2, nil)
	pkt[6] = 44
	binary.BigEndian.PutUint16(pkt[4:6], 4)
	pkt = pkt[:44]
	if _, _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("truncated extension header must be rejected")
	}
}

func TestAllow_IPv6FragmentedDatagramAdmitted(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5", "fd00::5")
	allowFor(t, "fd00::5", "fd00::10")

	head := buildIPv6FragUDP(t, "fd00::10", "fd00::5", 1111, 2222, 99, 0, true, []byte("head"))
	if insp.allow(head) {
		t.Fatal("IPv6 fragments must be dropped")
	}
	tail := buildIPv6FragUDP(t, "fd00::10", "fd00::5", 0, 0, 99, 12, false, []byte("tail"))
	if insp.allow(tail) {
		t.Fatal("trailing IPv6 fragments must be dropped")
	}
}
