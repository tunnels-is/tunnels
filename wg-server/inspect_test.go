package wgserver

import (
	"encoding/binary"
	"testing"
)

// ---------------------------------------------------------------------------
// parseIPHeader
// ---------------------------------------------------------------------------

func buildIPv4UDP(t *testing.T, src, dst string, sport, dport uint16, payload []byte) []byte {
	t.Helper()
	srcA := mustAddr(t, src).As4()
	dstA := mustAddr(t, dst).As4()
	total := 20 + 8 + len(payload)

	pkt := make([]byte, total)
	pkt[0] = 0x45 // v4, IHL=5
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
	pkt[0] = 0x60 // v6
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

// setFrag marks an IPv4 packet as a fragment: sets the identification field,
// the fragment offset (in 8-byte units), and the MF (more-fragments) bit.
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
	pkt[0] = 0x70 // not v4 or v6
	if _, _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("bad version must be rejected")
	}
}

func TestParseIPHeader_BadIHL(t *testing.T) {
	pkt := make([]byte, 20)
	pkt[0] = 0x40 // IHL=0
	if _, _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("IHL<5 must be rejected")
	}
}

func TestParseIPHeader_BadTotalLen(t *testing.T) {
	pkt := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, []byte("x"))
	binary.BigEndian.PutUint16(pkt[2:4], 9999) // > actual length
	if _, _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("total>len must be rejected")
	}
}

// ---------------------------------------------------------------------------
// inspectingTUN.allow
// ---------------------------------------------------------------------------

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

// allowFor installs a resident entry for dst and sets its allowlist to the
// given bare-host (all-ports) sources.
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
	// The server's own WG IP is unconditionally unreachable by peers — the
	// check runs before any firewall/allowlist logic.
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
	// With the firewall off, peer-to-peer passes without any policy...
	p2p := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, nil)
	if !insp.allow(p2p) {
		t.Fatal("firewall disabled => peer-to-peer must pass")
	}
	// ...but the server's WG IP stays unreachable.
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
	// Server-originated traffic (e.g. ICMP errors) passes without a policy.
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

// Revoking a source from the allowlist tears down its established conntrack
// flow, so it can no longer reach this device by coasting on the flow. This is
// the reported bug: remove the rule → the peer keeps talking via conntrack.
func TestAllow_RevokeDropsConntrackFlow(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.3")               // B
	allowFor(t, "10.0.0.2", "10.0.0.3") // A allows B (all ports)

	// B initiates to A:22 (admitted by the allowlist); A replies, opening A's
	// return-path flow {remote:B, rport:5000, lport:22}.
	if !insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil)) {
		t.Fatal("setup: B should reach A while allowlisted")
	}
	insp.allow(buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 5000, nil)) // A's reply opens the flow

	// Revoke B.
	entry(t, "10.0.0.2").setAllowed(nil, false)

	// B must now be dropped: allowlist denies and the coasting flow is gone.
	if insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil)) {
		t.Fatal("revoked peer must be dropped, not coast on the conntrack flow")
	}
}

// A policy change must not tear down flows for connections this device itself
// initiated to a remote that was never on the allowlist — those are its own
// return paths, unrelated to who is being allowed in.
func TestAllow_RevokeKeepsOwnOutboundFlow(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.2") // A, no allowlist

	// A initiates outbound to C (cross-server, never allowlisted); C's reply is
	// admitted by A's conntrack flow.
	insp.allow(buildIPv4UDP(t, "10.0.0.2", "10.99.0.9", 40000, 80, nil))
	if !insp.allow(buildIPv4UDP(t, "10.99.0.9", "10.0.0.2", 80, 40000, nil)) {
		t.Fatal("setup: C's reply should match A's own outbound flow")
	}

	// An unrelated policy change (allow some other peer) must leave A's own
	// outbound return path to C intact.
	entry(t, "10.0.0.2").setAllowed(hostRules(t, "10.0.0.5"), false)
	if !insp.allow(buildIPv4UDP(t, "10.99.0.9", "10.0.0.2", 80, 40000, nil)) {
		t.Fatal("policy change must not drop A's own initiated-outbound return path")
	}
}

// After revocation, if A itself starts talking to B again, A's outbound re-opens
// the return path and B's replies flow once more — the "slight interruption,
// then re-established" behavior for A-initiated traffic.
func TestAllow_RevokeReestablishesOnOutbound(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.3")
	allowFor(t, "10.0.0.2", "10.0.0.3")

	insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil)) // B->A
	insp.allow(buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 5000, nil)) // A->B reply opens flow
	entry(t, "10.0.0.2").setAllowed(nil, false)                        // revoke B

	if insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil)) {
		t.Fatal("B must be cut off immediately after revoke")
	}

	// A initiates to B again → re-opens the return-path flow.
	insp.allow(buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 5000, nil))
	if !insp.allow(buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 5000, 22, nil)) {
		t.Fatal("A's own outbound must re-establish the return path for B's replies")
	}
}

func TestAllow_HostPortRule(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")
	// 10.0.0.10 admits 10.0.0.5 only on port 22.
	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.5:22"]}`))
	if !insp.handleControl(ctrl) {
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
	// 10.0.0.10 admits ANY source on port 443 only ("*:443").
	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["*:443"]}`))
	if !insp.handleControl(ctrl) {
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
	setFrag(head, 4242, 0, true) // first fragment: MF set, offset 0
	_, _, _, _, frag, ok := parseIPHeader(head)
	if !ok || frag.id != 4242 || !frag.isFragment() || frag.isTrailing() {
		t.Fatalf("first fragment: got id=%d isFragment=%v isTrailing=%v", frag.id, frag.isFragment(), frag.isTrailing())
	}

	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, make([]byte, 8))
	setFrag(tail, 4242, 185, false) // trailing fragment: offset > 0
	_, _, _, _, frag, ok = parseIPHeader(tail)
	if !ok || frag.id != 4242 || !frag.isFragment() || !frag.isTrailing() {
		t.Fatalf("trailing fragment: got id=%d isFragment=%v isTrailing=%v", frag.id, frag.isFragment(), frag.isTrailing())
	}
}

// A fragmented datagram to an allowed port: the first fragment is port-checked
// and admitted, and its trailing fragments then pass by fragment-note.
func TestAllow_FragmentedDatagramAdmitted(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")
	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.5:5000"]}`))
	if !insp.handleControl(ctrl) {
		t.Fatal("expected consume")
	}

	head := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 40000, 5000, make([]byte, 8))
	setFrag(head, 42, 0, true)
	if !insp.allow(head) {
		t.Fatal("first fragment on an allowed port must pass")
	}
	// Trailing fragment carries garbage where the ports would be — irrelevant.
	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 12345, 6789, make([]byte, 8))
	setFrag(tail, 42, 185, false)
	if !insp.allow(tail) {
		t.Fatal("trailing fragment of an admitted datagram must pass")
	}
}

// The evasion case: a trailing fragment with no admitted head must be dropped,
// even when its payload bytes happen to equal an allowed port. This is the
// bug the fix closes — before it, the garbage dport matched the rule.
func TestAllow_OrphanTrailingFragmentDropped(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")
	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.5:5000"]}`))
	if !insp.handleControl(ctrl) {
		t.Fatal("expected consume")
	}
	// dport bytes forged to 5000 (an allowed port), but no first fragment admitted.
	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 5000, make([]byte, 8))
	setFrag(tail, 99, 185, false)
	if insp.allow(tail) {
		t.Fatal("orphan trailing fragment must be dropped even if its bytes look like an allowed port")
	}
}

// A fragmented datagram whose first fragment is to a denied port is dropped and
// leaves no note, so its trailing fragments are dropped too.
func TestAllow_FragmentedToDeniedPortDropped(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")
	ctrl := buildIPv4UDP(t, "10.0.0.10", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.5:5000"]}`))
	if !insp.handleControl(ctrl) {
		t.Fatal("expected consume")
	}
	head := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 40000, 22, make([]byte, 8)) // denied port
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

// A bare-host (all-ports) rule admits every fragment, as before the port
// feature — the head passes via portSet.all, the trailing via the all-ports
// fast-path (no dependence on the note).
func TestAllow_FragmentBareHostRule(t *testing.T) {
	insp := newTestInspector(t)
	allowFor(t, "10.0.0.10", "10.0.0.5")

	head := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 40000, 5000, make([]byte, 8))
	setFrag(head, 5, 0, true)
	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 0, 0, make([]byte, 8))
	setFrag(tail, 5, 185, false)
	if !insp.allow(head) || !insp.allow(tail) {
		t.Fatal("bare-host rule must pass all fragments")
	}
}

// Under an all-ports grant, a trailing fragment is admitted order-independently
// — even if it arrives before its head (or the head is lost). This is the
// pre-port-feature behavior the all-ports fast-path restores; without it, an
// all-ports source's reordered fragment would be wrongly dropped.
func TestAllow_FragmentBareHostOrderIndependent(t *testing.T) {
	insp := newTestInspector(t)
	allowFor(t, "10.0.0.10", "10.0.0.5")

	// Trailing fragment first, with no head ever admitted.
	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 0, 0, make([]byte, 8))
	setFrag(tail, 33, 185, false)
	if !insp.allow(tail) {
		t.Fatal("all-ports source: a headless/reordered trailing fragment must still pass")
	}

	// Same, under allow-all.
	resetPeer("10.0.0.11")
	entry(t, "10.0.0.11").setAllowed(nil, true) // allow-all
	tail2 := buildIPv4UDP(t, "10.0.0.99", "10.0.0.11", 0, 0, make([]byte, 8))
	setFrag(tail2, 34, 185, false)
	if !insp.allow(tail2) {
		t.Fatal("allow-all: a headless trailing fragment must pass")
	}
}

// The complement: under a port-scoped rule, a headless/reordered trailing
// fragment is dropped (it needs the port-checked head's note). This is the
// order-dependence we deliberately keep for port rules.
func TestAllow_FragmentPortRuleNeedsHead(t *testing.T) {
	insp := newTestInspector(t)
	setRule("10.0.0.10", "10.0.0.5:5000")

	tail := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 0, 0, make([]byte, 8))
	setFrag(tail, 55, 185, false)
	if insp.allow(tail) {
		t.Fatal("port-scoped rule: a trailing fragment with no admitted head must be dropped")
	}
}

// Connection-tracked return path: a receiver that initiated a flow admits the
// fragmented reply from a non-allowlisted source — first fragment via flowMatch,
// trailing fragments via the note it seeds.
func TestAllow_FragmentConntrackReturn(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.2") // A: no allowlist
	resetPeer("10.0.0.3") // B: no allowlist

	// A initiates to B:9000, opening A's return-path flow.
	out := buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 40000, 9000, nil)
	insp.allow(out)

	// B replies to A, fragmented. First fragment matches A's flow.
	head := buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 9000, 40000, make([]byte, 8))
	setFrag(head, 77, 0, true)
	if !insp.allow(head) {
		t.Fatal("reply first fragment must be admitted by connection tracking")
	}
	tail := buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 0, 0, make([]byte, 8))
	setFrag(tail, 77, 185, false)
	if !insp.allow(tail) {
		t.Fatal("reply trailing fragment must be admitted via the fragment note")
	}
}

// The SYN/SYN-ACK scenario: a one-sided allowlist plus connection tracking
// lets replies flow back without the other peer allowlisting the initiator.
func TestAllow_ConntrackReturnPath(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.3")               // phone
	allowFor(t, "10.0.0.2", "10.0.0.3") // laptop allows phone; phone allows nobody

	// SYN phone -> laptop:22 (allowed by laptop's allowlist; opens phone's flow)
	syn := buildIPv4UDP(t, "10.0.0.3", "10.0.0.2", 40000, 22, nil)
	if !insp.allow(syn) {
		t.Fatal("SYN to an allowlisted peer must pass")
	}
	// SYN-ACK laptop:22 -> phone:40000 (no allowlist entry on phone; must match
	// the flow the phone opened)
	synack := buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 40000, nil)
	if !insp.allow(synack) {
		t.Fatal("reply must be admitted by connection tracking")
	}
}

func TestAllow_ConntrackNoUnsolicitedReturn(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.3")               // phone, no allowlist, no prior flow
	allowFor(t, "10.0.0.2", "10.0.0.3") // laptop allows phone

	// laptop -> phone with no prior phone-initiated flow: must be denied.
	unsolicited := buildIPv4UDP(t, "10.0.0.2", "10.0.0.3", 22, 40000, nil)
	if insp.allow(unsolicited) {
		t.Fatal("unsolicited packet must be denied without a tracked flow")
	}
}

func TestAllow_AllowAllAdmitsAnySource(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.10")
	entry(t, "10.0.0.10").setAllowed(nil, true) // allow-all, empty list

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
	if !insp.handleControl(pkt) {
		t.Fatal("expected consume")
	}
	if !entry(t, "10.0.0.5").allowedContains(mustAddr(t, "10.0.0.123"), 80) {
		t.Fatal("AllowAll announcement must permit any source")
	}
}

func TestAllow_NonPeerToPeerAlwaysPasses(t *testing.T) {
	insp := newTestInspector(t)
	allowFor(t, "10.0.0.10") // strict (empty allowlist)

	pkt := buildIPv4UDP(t, "10.0.0.5", "8.8.8.8", 1, 2, nil)
	if !insp.allow(pkt) {
		t.Fatal("egress to internet must not be filtered")
	}
}

func TestAllow_MalformedAlwaysPasses(t *testing.T) {
	insp := newTestInspector(t)
	if !insp.allow([]byte{0x00, 0x00}) {
		t.Fatal("malformed packet must not be dropped")
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

// ---------------------------------------------------------------------------
// handleControl
// ---------------------------------------------------------------------------

func TestHandleControl_UpdatesACL(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5") // announcer must be a connected resident
	payload := []byte(`{"Allowed":["10.0.0.10","10.0.0.11"]}`)
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 33333, aclControlPort, payload)

	if !insp.handleControl(pkt) {
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
	if insp.handleControl(pkt) {
		t.Fatal("wrong port must not be consumed")
	}
}

func TestHandleControl_IgnoresWrongDst(t *testing.T) {
	insp := newTestInspector(t)
	pkt := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, aclControlPort, []byte(`{"Allowed":[]}`))
	if insp.handleControl(pkt) {
		t.Fatal("packet not addressed to server WG IP must not be consumed")
	}
}

func TestHandleControl_NonUDPIgnored(t *testing.T) {
	insp := newTestInspector(t)
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, nil)
	pkt[9] = 6 // TCP
	if insp.handleControl(pkt) {
		t.Fatal("non-UDP must not be consumed as control")
	}
}

func TestHandleControl_TrailingFragmentNotControl(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")
	// A trailing fragment addressed to the server IP: its bytes at the dport
	// position happen to equal the control port (buildIPv4UDP writes it there),
	// but a trailing fragment carries no real UDP header, so it must not be
	// treated as control — nor may it alter policy.
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.10"]}`))
	setFrag(pkt, 1234, 185, false) // offset > 0 → trailing fragment
	if insp.handleControl(pkt) {
		t.Fatal("a trailing fragment must not be consumed as a control packet")
	}
	if entry(t, "10.0.0.5").allowedContains(mustAddr(t, "10.0.0.10"), 80) {
		t.Fatal("a trailing fragment must not modify firewall policy")
	}
}

func TestHandleControl_FirstFragmentStillControl(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")
	// The first fragment (offset 0) carries the real UDP header, so it is still
	// classified as control — the guard only rejects trailing fragments.
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort,
		[]byte(`{"Allowed":["10.0.0.10"]}`))
	setFrag(pkt, 1234, 0, true) // offset 0, MF set → first fragment
	if !insp.handleControl(pkt) {
		t.Fatal("a first fragment on the control port must be consumed")
	}
}

func TestHandleControl_SrcOutsideWGSubnet(t *testing.T) {
	insp := newTestInspector(t)
	// src outside subnet should be consumed (because dst+port match) but ignored.
	pkt := buildIPv4UDP(t, "8.8.8.8", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":["10.0.0.10"]}`))
	if !insp.handleControl(pkt) {
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
	if !insp.handleControl(pkt) {
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
	if !insp.handleControl(pkt) {
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
	if !insp.handleControl(set) {
		t.Fatal("expected consume")
	}
	if len(peerListSnapshot()) != 1 {
		t.Fatal("setup: policy should be stored")
	}
	clear := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":[]}`))
	if !insp.handleControl(clear) {
		t.Fatal("expected consume")
	}
	if len(peerListSnapshot()) != 0 {
		t.Fatal("empty allowlist must clear the stored policy")
	}
}

func TestHandleControl_CrossServerIPAccepted(t *testing.T) {
	insp := newTestInspector(t)
	resetPeer("10.0.0.5")
	// Peers can talk across wg-servers — an allowed IP outside this server's
	// subnets must still be stored.
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":["10.99.0.7"]}`))
	if !insp.handleControl(pkt) {
		t.Fatal("expected consume")
	}
	if !entry(t, "10.0.0.5").allowedContains(mustAddr(t, "10.99.0.7"), 80) {
		t.Fatal("cross-server IP must be accepted into the allowlist")
	}
}

func TestHandleControl_AnnounceWithoutEntryDropped(t *testing.T) {
	insp := newTestInspector(t)
	// No resetPeer for 10.0.0.5: an announce from a peer with no installed
	// entry (raced the handshake) is consumed but applies nothing.
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":["10.0.0.10"]}`))
	if !insp.handleControl(pkt) {
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
	if !insp.handleControl(pkt) {
		t.Fatal("v6 control should be consumed")
	}
	p, _ := fwClassify(mustAddr(t, "fd00::5"))
	if p == nil || !p.allowedContains(mustAddr(t, "fd00::10"), 80) {
		t.Fatal("v6 ACL not applied")
	}
}
