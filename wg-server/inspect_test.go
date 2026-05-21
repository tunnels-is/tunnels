package wgserver

import (
	"encoding/binary"
	"net/netip"
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

func TestParseIPHeader_IPv4UDP(t *testing.T) {
	pkt := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1234, 5678, []byte("hi"))
	src, dst, proto, l4, ok := parseIPHeader(pkt)
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
}

func TestParseIPHeader_IPv6UDP(t *testing.T) {
	pkt := buildIPv6UDP(t, "fd00::5", "fd00::10", 1234, 5678, []byte("hi"))
	src, dst, proto, _, ok := parseIPHeader(pkt)
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
		if _, _, _, _, ok := parseIPHeader(make([]byte, n)); ok {
			t.Fatalf("len=%d should be invalid", n)
		}
	}
}

func TestParseIPHeader_BadVersion(t *testing.T) {
	pkt := make([]byte, 40)
	pkt[0] = 0x70 // not v4 or v6
	if _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("bad version must be rejected")
	}
}

func TestParseIPHeader_BadIHL(t *testing.T) {
	pkt := make([]byte, 20)
	pkt[0] = 0x40 // IHL=0
	if _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("IHL<5 must be rejected")
	}
}

func TestParseIPHeader_BadTotalLen(t *testing.T) {
	pkt := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, []byte("x"))
	binary.BigEndian.PutUint16(pkt[2:4], 9999) // > actual length
	if _, _, _, _, ok := parseIPHeader(pkt); ok {
		t.Fatal("total>len must be rejected")
	}
}

// ---------------------------------------------------------------------------
// inspectingTUN.allow
// ---------------------------------------------------------------------------

func newTestInspector(t *testing.T) *inspectingTUN {
	t.Helper()
	insp, err := newInspectingTUN(nil, NewACLStore(), &Config{
		WireGuardSubnet:  "10.0.0.0/24",
		WireGuardSubnet6: "fd00::/64",
	})
	if err != nil {
		t.Fatal(err)
	}
	return insp
}

func TestAllow_DefaultOpenForPeerToPeer(t *testing.T) {
	insp := newTestInspector(t)
	pkt := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, nil)
	if !insp.allow(pkt) {
		t.Fatal("no policy => must allow")
	}
}

func TestAllow_DeniesUnlistedSrc(t *testing.T) {
	insp := newTestInspector(t)
	insp.acl.Set(mustAddr(t, "10.0.0.10"), []netip.Addr{mustAddr(t, "10.0.0.5")})

	allowed := buildIPv4UDP(t, "10.0.0.5", "10.0.0.10", 1, 2, nil)
	denied := buildIPv4UDP(t, "10.0.0.99", "10.0.0.10", 1, 2, nil)

	if !insp.allow(allowed) {
		t.Fatal("allowed src should pass")
	}
	if insp.allow(denied) {
		t.Fatal("unlisted src should be dropped")
	}
}

func TestAllow_NonPeerToPeerAlwaysPasses(t *testing.T) {
	insp := newTestInspector(t)
	// Even with a strict ACL, traffic where one side is outside the WG
	// subnet is untouched (internet egress, server-originated, etc.).
	insp.acl.Set(mustAddr(t, "10.0.0.10"), nil) // total isolation

	pkt := buildIPv4UDP(t, "10.0.0.5", "8.8.8.8", 1, 2, nil)
	if !insp.allow(pkt) {
		t.Fatal("egress to internet must not be filtered")
	}
}

func TestAllow_MalformedAlwaysPasses(t *testing.T) {
	insp := newTestInspector(t)
	if !insp.allow([]byte{0x00, 0x00}) {
		t.Fatal("malformed packet must not be dropped by ACL")
	}
}

func TestAllow_IPv6(t *testing.T) {
	insp := newTestInspector(t)
	insp.acl.Set(mustAddr(t, "fd00::10"), []netip.Addr{mustAddr(t, "fd00::5")})

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
	payload := []byte(`{"Allowed":["10.0.0.10","10.0.0.11"]}`)
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 33333, aclControlPort, payload)

	if !insp.handleControl(pkt) {
		t.Fatal("expected packet to be consumed")
	}
	dst := mustAddr(t, "10.0.0.5")
	if !insp.acl.Allowed(mustAddr(t, "10.0.0.10"), dst) {
		t.Fatal("10.0.0.10 should be allowed after control message")
	}
	if insp.acl.Allowed(mustAddr(t, "10.0.0.99"), dst) {
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

func TestHandleControl_SrcOutsideWGSubnet(t *testing.T) {
	insp := newTestInspector(t)
	// src outside subnet should be consumed (because dst+port match) but ignored.
	pkt := buildIPv4UDP(t, "8.8.8.8", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":["10.0.0.10"]}`))
	if !insp.handleControl(pkt) {
		t.Fatal("matching dst+port should be consumed even if src is invalid")
	}
	if !insp.acl.Allowed(mustAddr(t, "10.0.0.99"), mustAddr(t, "8.8.8.8")) {
		t.Fatal("ACL must not have been written for an outside-subnet src")
	}
}

func TestHandleControl_BadJSON(t *testing.T) {
	insp := newTestInspector(t)
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte("not json"))
	if !insp.handleControl(pkt) {
		t.Fatal("malformed payload should still be consumed (not forwarded)")
	}
	// ACL unchanged — no policy stored for src.
	if !insp.acl.Allowed(mustAddr(t, "10.0.0.99"), mustAddr(t, "10.0.0.5")) {
		t.Fatal("no policy should be stored after a bad payload")
	}
}

func TestHandleControl_EmptyListIsolatesSender(t *testing.T) {
	insp := newTestInspector(t)
	pkt := buildIPv4UDP(t, "10.0.0.5", insp.serverIPv4.String(), 1, aclControlPort, []byte(`{"Allowed":[]}`))
	if !insp.handleControl(pkt) {
		t.Fatal("expected consume")
	}
	if insp.acl.Allowed(mustAddr(t, "10.0.0.10"), mustAddr(t, "10.0.0.5")) {
		t.Fatal("empty allowlist must isolate the sender")
	}
}

func TestHandleControl_IPv6(t *testing.T) {
	insp := newTestInspector(t)
	payload := []byte(`{"Allowed":["fd00::10"]}`)
	pkt := buildIPv6UDP(t, "fd00::5", insp.serverIPv6.String(), 1, aclControlPort, payload)
	if !insp.handleControl(pkt) {
		t.Fatal("v6 control should be consumed")
	}
	if !insp.acl.Allowed(mustAddr(t, "fd00::10"), mustAddr(t, "fd00::5")) {
		t.Fatal("v6 ACL not applied")
	}
}
