package main

import (
	"testing"

	"github.com/tunnels-is/tunnels/signal"
	"github.com/tunnels-is/tunnels/types"
)

func TestMain(m *testing.M) {
	initLogging(true, false, false, "error")
	m.Run()
}

// helpers shared across firewall tests
func ip4b(a, b, c, d byte) [4]byte { return [4]byte{a, b, c, d} }
func pt(a, b byte) [2]byte          { return [2]byte{a, b} }

func Test_getIP4FromHostOrDHCP_ValidIPs(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected [4]byte
		expectOk bool
	}{
		{
			name:     "valid IPv4 - 192.168.1.1",
			host:     "192.168.1.1",
			expected: [4]byte{192, 168, 1, 1},
			expectOk: true,
		},
		{
			name:     "valid IPv4 - 10.0.0.1",
			host:     "10.0.0.1",
			expected: [4]byte{10, 0, 0, 1},
			expectOk: true,
		},
		{
			name:     "valid IPv4 - 172.16.0.1",
			host:     "172.16.0.1",
			expected: [4]byte{172, 16, 0, 1},
			expectOk: true,
		},
		{
			name:     "valid IPv4 - 8.8.8.8",
			host:     "8.8.8.8",
			expected: [4]byte{8, 8, 8, 8},
			expectOk: true,
		},
		{
			name:     "valid IPv4 - 1.1.1.1",
			host:     "1.1.1.1",
			expected: [4]byte{1, 1, 1, 1},
			expectOk: true,
		},
		{
			name:     "valid IPv4 - 127.0.0.1",
			host:     "127.0.0.1",
			expected: [4]byte{127, 0, 0, 1},
			expectOk: true,
		},
		{
			name:     "valid IPv4 - 255.255.255.255",
			host:     "255.255.255.255",
			expected: [4]byte{255, 255, 255, 255},
			expectOk: true,
		},
		{
			name:     "valid IPv4 - 0.0.0.0",
			host:     "0.0.0.0",
			expected: [4]byte{0, 0, 0, 0},
			expectOk: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip4, ok := getIP4FromHostOrDHCP(tc.host)

			if ok != tc.expectOk {
				t.Errorf("getIP4FromHostOrDHCP(%q) ok = %v, expected %v", tc.host, ok, tc.expectOk)
			}

			if ok && ip4 != tc.expected {
				t.Errorf("getIP4FromHostOrDHCP(%q) = %v, expected %v", tc.host, ip4, tc.expected)
			}

			t.Logf("getIP4FromHostOrDHCP(%q) = %v, ok=%v ✓", tc.host, ip4, ok)
		})
	}
}

func Test_getIP4FromHostOrDHCP_InvalidIPs(t *testing.T) {
	tests := []struct {
		name string
		host string
	}{
		{
			name: "invalid IP - missing octet",
			host: "192.168.1",
		},
		{
			name: "invalid IP - too many octets",
			host: "192.168.1.1.1",
		},
		{
			name: "invalid IP - out of range",
			host: "256.256.256.256",
		},
		{
			name: "invalid IP - letters",
			host: "abc.def.ghi.jkl",
		},
		{
			name: "invalid IP - negative numbers",
			host: "-1.-1.-1.-1",
		},
		{
			name: "invalid IP - empty string",
			host: "",
		},
		{
			name: "invalid IP - just dots",
			host: "...",
		},
		{
			name: "invalid IP - special characters",
			host: "192!168@1#1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip4, ok := getIP4FromHostOrDHCP(tc.host)

			if ok {
				t.Errorf("getIP4FromHostOrDHCP(%q) should fail but succeeded with %v", tc.host, ip4)
			}

			t.Logf("getIP4FromHostOrDHCP(%q) correctly failed ✓", tc.host)
		})
	}
}

func Test_getIP4FromHostOrDHCP_IPv6ToIPv4Conversion(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected [4]byte
		expectOk bool
	}{
		{
			name:     "IPv6 mapped IPv4 - ::ffff:192.168.1.1",
			host:     "::ffff:192.168.1.1",
			expected: [4]byte{192, 168, 1, 1},
			expectOk: true,
		},
		{
			name:     "IPv6 mapped IPv4 - ::ffff:8.8.8.8",
			host:     "::ffff:8.8.8.8",
			expected: [4]byte{8, 8, 8, 8},
			expectOk: true,
		},
		{
			name:     "pure IPv6 - should fail (not IPv4)",
			host:     "2001:db8::1",
			expectOk: false, // Pure IPv6 not convertible to IPv4
		},
		{
			name:     "IPv6 loopback - ::1",
			host:     "::1",
			expectOk: false, // Pure IPv6 loopback
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip4, ok := getIP4FromHostOrDHCP(tc.host)

			if ok != tc.expectOk {
				t.Errorf("getIP4FromHostOrDHCP(%q) ok = %v, expected %v", tc.host, ok, tc.expectOk)
			}

			if ok && ip4 != tc.expected {
				t.Errorf("getIP4FromHostOrDHCP(%q) = %v, expected %v", tc.host, ip4, tc.expected)
			}

			t.Logf("getIP4FromHostOrDHCP(%q) = %v, ok=%v ✓", tc.host, ip4, ok)
		})
	}
}

func Test_getIP4FromHostOrDHCP_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected [4]byte
		expectOk bool
	}{
		{
			name:     "IP with leading zeros - 192.168.001.001 (not supported by net.ParseIP)",
			host:     "192.168.001.001",
			expectOk: false, // net.ParseIP doesn't support leading zeros
		},
		{
			name:     "whitespace prefix",
			host:     " 192.168.1.1",
			expectOk: false,
		},
		{
			name:     "whitespace suffix",
			host:     "192.168.1.1 ",
			expectOk: false,
		},
		{
			name:     "tab character",
			host:     "192.168.1.1\t",
			expectOk: false,
		},
		{
			name:     "newline character",
			host:     "192.168.1.1\n",
			expectOk: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip4, ok := getIP4FromHostOrDHCP(tc.host)

			if ok != tc.expectOk {
				t.Errorf("getIP4FromHostOrDHCP(%q) ok = %v, expected %v", tc.host, ok, tc.expectOk)
			}

			if ok && ip4 != tc.expected {
				t.Errorf("getIP4FromHostOrDHCP(%q) = %v, expected %v", tc.host, ip4, tc.expected)
			}

			t.Logf("getIP4FromHostOrDHCP(%q) ok=%v ✓", tc.host, ok)
		})
	}
}

func Test_getIP4FromHostOrDHCP_AllZeros(t *testing.T) {
	ip4, ok := getIP4FromHostOrDHCP("0.0.0.0")
	if !ok {
		t.Error("0.0.0.0 should be valid")
	}

	expected := [4]byte{0, 0, 0, 0}
	if ip4 != expected {
		t.Errorf("getIP4FromHostOrDHCP(\"0.0.0.0\") = %v, expected %v", ip4, expected)
	}

	t.Log("getIP4FromHostOrDHCP(\"0.0.0.0\") correctly parsed ✓")
}

func Test_getIP4FromHostOrDHCP_BroadcastAddress(t *testing.T) {
	ip4, ok := getIP4FromHostOrDHCP("255.255.255.255")
	if !ok {
		t.Error("255.255.255.255 should be valid")
	}

	expected := [4]byte{255, 255, 255, 255}
	if ip4 != expected {
		t.Errorf("getIP4FromHostOrDHCP(\"255.255.255.255\") = %v, expected %v", ip4, expected)
	}

	t.Log("getIP4FromHostOrDHCP(\"255.255.255.255\") correctly parsed ✓")
}

// ---------------------------------------------------------------------------
// AddHost
// ---------------------------------------------------------------------------

func TestAddHost(t *testing.T) {
	t.Run("adds new entry", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		if len(cm.AllowedHosts) != 1 {
			t.Fatalf("expected 1 host, got %d", len(cm.AllowedHosts))
		}
		h := cm.AllowedHosts[0]
		if h.IP != ip4b(10, 0, 0, 5) || h.PORT != pt(0, 80) || h.Type != "auto" {
			t.Errorf("unexpected entry: %+v", h)
		}
	})

	t.Run("does not duplicate same IP and port", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		if len(cm.AllowedHosts) != 1 {
			t.Errorf("expected 1 entry, got %d", len(cm.AllowedHosts))
		}
	})

	t.Run("does not add auto when manual already exists for same IP", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 0), "manual")
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		if len(cm.AllowedHosts) != 1 {
			t.Errorf("expected 1 entry, got %d", len(cm.AllowedHosts))
		}
	})

	t.Run("different IPs are added independently", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.AddHost(ip4b(10, 0, 0, 6), pt(0, 80), "auto")
		if len(cm.AllowedHosts) != 2 {
			t.Errorf("expected 2 entries, got %d", len(cm.AllowedHosts))
		}
	})
}

// ---------------------------------------------------------------------------
// DelHost
// ---------------------------------------------------------------------------

func TestDelHost(t *testing.T) {
	t.Run("removes existing entry", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.DelHost(ip4b(10, 0, 0, 5), "auto")
		if len(cm.AllowedHosts) != 0 {
			t.Errorf("expected 0 entries, got %d", len(cm.AllowedHosts))
		}
	})

	t.Run("does not remove entry with wrong type", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.DelHost(ip4b(10, 0, 0, 5), "manual")
		if len(cm.AllowedHosts) != 1 {
			t.Errorf("expected 1 entry to remain, got %d", len(cm.AllowedHosts))
		}
	})

	t.Run("does not remove entry with wrong IP", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.DelHost(ip4b(10, 0, 0, 6), "auto")
		if len(cm.AllowedHosts) != 1 {
			t.Errorf("expected 1 entry to remain, got %d", len(cm.AllowedHosts))
		}
	})

	t.Run("preserves other entries when removing one", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.AddHost(ip4b(10, 0, 0, 6), pt(0, 80), "auto")
		cm.DelHost(ip4b(10, 0, 0, 5), "auto")
		if len(cm.AllowedHosts) != 1 {
			t.Fatalf("expected 1 entry to remain, got %d", len(cm.AllowedHosts))
		}
		if cm.AllowedHosts[0].IP != ip4b(10, 0, 0, 6) {
			t.Errorf("wrong entry remained: %+v", cm.AllowedHosts[0])
		}
	})

	t.Run("no-op on empty list", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.DelHost(ip4b(10, 0, 0, 5), "auto") // must not panic
	})
}

// ---------------------------------------------------------------------------
// IsHostAllowed
// ---------------------------------------------------------------------------

func TestIsHostAllowed(t *testing.T) {
	t.Run("returns nil for empty list", func(t *testing.T) {
		cm := &UserCoreMapping{}
		if cm.IsHostAllowed(ip4b(10, 0, 0, 5), pt(0, 80)) != nil {
			t.Error("expected nil for empty list")
		}
	})

	t.Run("matches auto entry by IP and port", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		if cm.IsHostAllowed(ip4b(10, 0, 0, 5), pt(0, 80)) == nil {
			t.Error("expected a match for correct IP and port")
		}
	})

	t.Run("rejects auto entry with wrong port", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		if cm.IsHostAllowed(ip4b(10, 0, 0, 5), pt(1, 187)) != nil {
			t.Error("expected nil for wrong port on auto entry")
		}
	})

	t.Run("manual entry matches any port", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 0), "manual")
		if cm.IsHostAllowed(ip4b(10, 0, 0, 5), pt(0, 80)) == nil {
			t.Error("expected match on manual entry for port 80")
		}
		if cm.IsHostAllowed(ip4b(10, 0, 0, 5), pt(1, 187)) == nil {
			t.Error("expected match on manual entry for port 443")
		}
	})

	t.Run("returns nil for wrong IP", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		if cm.IsHostAllowed(ip4b(10, 0, 0, 6), pt(0, 80)) != nil {
			t.Error("expected nil for wrong IP")
		}
	})
}

// ---------------------------------------------------------------------------
// SetFin
// ---------------------------------------------------------------------------

func TestSetFin(t *testing.T) {
	t.Run("sets FFIN when called from user side", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.SetFin(ip4b(10, 0, 0, 5), pt(0, 80), true)
		h := cm.AllowedHosts[0]
		if !h.FFIN {
			t.Error("expected FFIN=true")
		}
		if h.TFIN {
			t.Error("expected TFIN to remain false")
		}
	})

	t.Run("sets TFIN when called from server side", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.SetFin(ip4b(10, 0, 0, 5), pt(0, 80), false)
		h := cm.AllowedHosts[0]
		if h.FFIN {
			t.Error("expected FFIN to remain false")
		}
		if !h.TFIN {
			t.Error("expected TFIN=true")
		}
	})

	t.Run("no-op for wrong IP", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.SetFin(ip4b(10, 0, 0, 6), pt(0, 80), true)
		h := cm.AllowedHosts[0]
		if h.FFIN || h.TFIN {
			t.Error("expected no FIN flags set for wrong IP")
		}
	})

	t.Run("no-op for wrong port", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.SetFin(ip4b(10, 0, 0, 5), pt(1, 187), true)
		h := cm.AllowedHosts[0]
		if h.FFIN || h.TFIN {
			t.Error("expected no FIN flags set for wrong port")
		}
	})
}

// ---------------------------------------------------------------------------
// ClearHost
// ---------------------------------------------------------------------------

func TestClearHost(t *testing.T) {
	t.Run("removes single entry", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.ClearHost(ip4b(10, 0, 0, 5))
		if len(cm.AllowedHosts) != 0 {
			t.Errorf("expected 0 entries, got %d", len(cm.AllowedHosts))
		}
	})

	t.Run("removes all ports for the same IP", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AllowedHosts = []*AllowedHost{
			{IP: ip4b(10, 0, 0, 5), PORT: pt(0, 80), Type: "auto"},
			{IP: ip4b(10, 0, 0, 5), PORT: pt(1, 187), Type: "auto"},
		}
		cm.ClearHost(ip4b(10, 0, 0, 5))
		if len(cm.AllowedHosts) != 0 {
			t.Errorf("expected 0 entries, got %d", len(cm.AllowedHosts))
		}
	})

	t.Run("removes both auto and manual entries for the same IP", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AllowedHosts = []*AllowedHost{
			{IP: ip4b(10, 0, 0, 5), PORT: pt(0, 0), Type: "manual"},
			{IP: ip4b(10, 0, 0, 5), PORT: pt(0, 80), Type: "auto"},
		}
		cm.ClearHost(ip4b(10, 0, 0, 5))
		if len(cm.AllowedHosts) != 0 {
			t.Errorf("expected 0 entries, got %d", len(cm.AllowedHosts))
		}
	})

	t.Run("preserves entries for other IPs", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.AddHost(ip4b(10, 0, 0, 6), pt(0, 80), "auto")
		cm.ClearHost(ip4b(10, 0, 0, 5))
		if len(cm.AllowedHosts) != 1 {
			t.Fatalf("expected 1 entry to remain, got %d", len(cm.AllowedHosts))
		}
		if cm.AllowedHosts[0].IP != ip4b(10, 0, 0, 6) {
			t.Errorf("wrong entry remained: %+v", cm.AllowedHosts[0])
		}
	})

	t.Run("no-op for non-existent IP", func(t *testing.T) {
		cm := &UserCoreMapping{}
		cm.AddHost(ip4b(10, 0, 0, 5), pt(0, 80), "auto")
		cm.ClearHost(ip4b(10, 0, 0, 6))
		if len(cm.AllowedHosts) != 1 {
			t.Errorf("expected list unchanged, got %d entries", len(cm.AllowedHosts))
		}
	})
}

// ---------------------------------------------------------------------------
// TCPLifecycle — functional test for the full connection state machine
//
// Each step calls AllowedHosts methods with exactly the arguments that
// fromUserChannel / toUserChannel in socket.go would derive from real
// packet bytes, without spinning up goroutines or network I/O.
//
// Topology:
//   cmA — "server" listening at 10.0.0.1:80
//   cmB — "client" at 10.0.0.2, ephemeral source port 54321
//
// Port encoding (network byte order — two bytes as extracted by socket.go):
//   fromUserChannel  D4Port = PACKET[l+2 : l+4]   (destination port)
//   toUserChannel    S4Port = PACKET[headLen : headLen+2] (source port)
//
//   port 80    → {0x00, 0x50}   = pt(0, 80)
//   port 54321 → {0xD4, 0x31}   = pt(0xD4, 0x31)
//
// When B sends to A:80  — D4Port = {0,80},       S4Port on A side = {0xD4,0x31}
// When A sends to B     — D4Port = {0xD4,0x31},  S4Port on B side = {0,80}
// ---------------------------------------------------------------------------

func TestFirewall_TCPLifecycle(t *testing.T) {
	aIP := ip4b(10, 0, 0, 1)
	bIP := ip4b(10, 0, 0, 2)
	port80    := pt(0x00, 0x50) // 80   — A's listen port
	portEph   := pt(0xD4, 0x31) // 54321 — B's ephemeral source port

	t.Run("graceful FIN teardown", func(t *testing.T) {
		cmA := &UserCoreMapping{}
		cmB := &UserCoreMapping{}

		// Pre-condition: A has B in its AllowedHosts (from a prior outbound packet
		// or admin manual rule) so that A's toUserChannel gate passes for B's source.
		cmA.AddHost(bIP, portEph, "auto")

		// ── SYN: B → A:80 ────────────────────────────────────────────────────
		// socket.go fromUserChannel (cmB) line ~413: SYN > 0 → AddHost(D4=aIP, D4Port=port80, "auto")
		cmB.AddHost(aIP, port80, "auto")
		if cmB.IsHostAllowed(aIP, port80) == nil {
			t.Fatal("SYN: cmB must allow return traffic from aIP:80")
		}

		// socket.go toUserChannel (cmA) line ~542: IsHostAllowed(S4=bIP, S4Port=portEph)
		// SYN carries no FIN/RST → gate check only, no state change on receiver.
		if cmA.IsHostAllowed(bIP, portEph) == nil {
			t.Fatal("SYN: cmA firewall gate must pass for bIP:54321")
		}

		// ── SYN-ACK: A → B ───────────────────────────────────────────────────
		// socket.go fromUserChannel (cmA) line ~413: SYN > 0 → AddHost(D4=bIP, D4Port=portEph, "auto")
		// Entry already exists from pre-condition; AddHost is idempotent.
		cmA.AddHost(bIP, portEph, "auto")
		if len(cmA.AllowedHosts) != 1 {
			t.Errorf("SYN-ACK: AddHost must be idempotent, expected 1 entry got %d", len(cmA.AllowedHosts))
		}

		// socket.go toUserChannel (cmB) line ~542: IsHostAllowed(S4=aIP, S4Port=port80)
		// SYN+ACK carries no FIN/RST → gate check only.
		if cmB.IsHostAllowed(aIP, port80) == nil {
			t.Fatal("SYN-ACK: cmB firewall gate must pass for aIP:80")
		}

		// ── DATA exchange — gate checks only, no state change ─────────────────
		if cmB.IsHostAllowed(aIP, port80) == nil {
			t.Fatal("DATA B→A: cmB must allow data from aIP:80")
		}
		if cmA.IsHostAllowed(bIP, portEph) == nil {
			t.Fatal("DATA A→B: cmA must allow data from bIP:54321")
		}

		// ── FIN: B → A (B initiates half-close) ──────────────────────────────
		// socket.go fromUserChannel (cmB) line ~415: FIN > 0 → SetFin(D4=aIP, D4Port=port80, true)
		cmB.SetFin(aIP, port80, true)
		h := cmB.IsHostAllowed(aIP, port80)
		if h == nil {
			t.Fatal("FIN send B: entry for aIP must still exist after sending FIN")
		}
		if !h.FFIN {
			t.Error("FIN send B: FFIN must be true after B sends FIN")
		}
		if h.TFIN {
			t.Error("FIN send B: TFIN must still be false before A replies")
		}

		// socket.go toUserChannel (cmA) line ~542–555:
		//   activeHost = IsHostAllowed(S4=bIP, S4Port=portEph)
		//   FIN set; activeHost.FFIN == false → SetFin(bIP, portEph, false) → TFIN=true
		if cmA.IsHostAllowed(bIP, portEph) == nil {
			t.Fatal("FIN recv A: cmA must have entry for bIP to process the FIN")
		}
		cmA.SetFin(bIP, portEph, false)
		h = cmA.IsHostAllowed(bIP, portEph)
		if h == nil || !h.TFIN {
			t.Fatal("FIN recv A: cmA entry for bIP must have TFIN=true")
		}
		if h.FFIN {
			t.Error("FIN recv A: FFIN must still be false — A has not sent its own FIN yet")
		}

		// ── FIN-ACK: A → B (A closes its side) ───────────────────────────────
		// socket.go fromUserChannel (cmA) line ~415: FIN > 0 → SetFin(D4=bIP, D4Port=portEph, true)
		cmA.SetFin(bIP, portEph, true)
		h = cmA.IsHostAllowed(bIP, portEph)
		if h == nil {
			t.Fatal("FIN-ACK send A: entry for bIP must still exist")
		}
		if !h.FFIN || !h.TFIN {
			t.Errorf("FIN-ACK send A: expected FFIN=true TFIN=true, got FFIN=%v TFIN=%v", h.FFIN, h.TFIN)
		}

		// socket.go toUserChannel (cmB) line ~551–553:
		//   activeHost = IsHostAllowed(S4=aIP, S4Port=port80)
		//   FIN set; activeHost.FFIN == true → DelHost(aIP, "auto")
		if cmB.IsHostAllowed(aIP, port80) == nil {
			t.Fatal("FIN-ACK recv B: cmB must have entry for aIP to process FIN-ACK")
		}
		cmB.DelHost(aIP, "auto") // both sides have sent FIN → remove entry
		if cmB.IsHostAllowed(aIP, port80) != nil {
			t.Error("FIN-ACK recv B: cmB entry for aIP must be removed (both FINs exchanged)")
		}

		// ── Final state ───────────────────────────────────────────────────────
		if len(cmB.AllowedHosts) != 0 {
			t.Errorf("after graceful close: cmB AllowedHosts must be empty, got %d entries", len(cmB.AllowedHosts))
		}
		// A's entry for B stays until a subsequent RST or disconnect — B's final
		// ACK carries no FIN bit so socket.go has no hook to remove it here.
		if cmA.IsHostAllowed(bIP, portEph) == nil {
			t.Error("cmA entry for bIP should still exist (no FIN in B's final ACK)")
		}
	})

	t.Run("RST terminates connection immediately on both sides", func(t *testing.T) {
		cmA := &UserCoreMapping{}
		cmB := &UserCoreMapping{}

		// Establish connection
		cmB.AddHost(aIP, port80, "auto")
		cmA.AddHost(bIP, portEph, "auto")

		if cmB.IsHostAllowed(aIP, port80) == nil || cmA.IsHostAllowed(bIP, portEph) == nil {
			t.Fatal("connection not established before RST test")
		}

		// ── RST: B → A ───────────────────────────────────────────────────────
		// socket.go fromUserChannel (cmB) line ~411: RST > 0 → DelHost(D4=aIP, "auto")
		cmB.DelHost(aIP, "auto")
		if cmB.IsHostAllowed(aIP, port80) != nil {
			t.Error("RST send B: cmB entry for aIP must be removed immediately")
		}

		// socket.go toUserChannel (cmA) line ~549–550: RST > 0 → DelHost(S4=bIP, "auto")
		cmA.DelHost(bIP, "auto")
		if cmA.IsHostAllowed(bIP, portEph) != nil {
			t.Error("RST recv A: cmA entry for bIP must be removed immediately")
		}

		if len(cmB.AllowedHosts) != 0 {
			t.Errorf("RST: cmB AllowedHosts must be empty, got %d", len(cmB.AllowedHosts))
		}
		if len(cmA.AllowedHosts) != 0 {
			t.Errorf("RST: cmA AllowedHosts must be empty, got %d", len(cmA.AllowedHosts))
		}
	})

	t.Run("nil VPLIPToCore removes sender firewall entry (target disconnected)", func(t *testing.T) {
		// socket.go fromUserChannel lines ~397–401:
		//   targetCM = VPLIPToCore[D4[0]][D4[1]][D4[2]][D4[3]]
		//   if targetCM == nil { CM.DelHost(D4, "auto"); continue }
		// This is intentional NAT-like cleanup: if the target is not reachable,
		// the sender's firewall entry is immediately removed.
		cmB := &UserCoreMapping{}
		cmB.AddHost(aIP, port80, "auto")

		// Simulate VPLIPToCore[aIP] == nil → DelHost fires
		cmB.DelHost(aIP, "auto")

		if cmB.IsHostAllowed(aIP, port80) != nil {
			t.Error("nil VPLIPToCore: cmB entry for aIP must be removed")
		}
		if len(cmB.AllowedHosts) != 0 {
			t.Errorf("nil VPLIPToCore: cmB AllowedHosts must be empty, got %d", len(cmB.AllowedHosts))
		}
	})

	t.Run("simultaneous FIN — each side independently processes the other's FIN", func(t *testing.T) {
		// Both peers send FIN at the same time (no half-close).
		// Each side's fromUserChannel calls SetFin(peer, port, true) → FFIN=true.
		// Each side's toUserChannel sees FIN from peer: activeHost.FFIN is true → DelHost.
		cmA := &UserCoreMapping{}
		cmB := &UserCoreMapping{}
		cmA.AddHost(bIP, portEph, "auto")
		cmB.AddHost(aIP, port80, "auto")

		// Both send FIN simultaneously
		// fromUserChannel (cmA): FIN → SetFin(bIP, portEph, true)
		cmA.SetFin(bIP, portEph, true)
		// fromUserChannel (cmB): FIN → SetFin(aIP, port80, true)
		cmB.SetFin(aIP, port80, true)

		// toUserChannel (cmA) receives FIN from B: activeHost.FFIN=true → DelHost
		if ah := cmA.IsHostAllowed(bIP, portEph); ah != nil && ah.FFIN {
			cmA.DelHost(bIP, "auto")
		}
		// toUserChannel (cmB) receives FIN from A: activeHost.FFIN=true → DelHost
		if ah := cmB.IsHostAllowed(aIP, port80); ah != nil && ah.FFIN {
			cmB.DelHost(aIP, "auto")
		}

		if len(cmA.AllowedHosts) != 0 {
			t.Errorf("simultaneous FIN: cmA AllowedHosts must be empty, got %d", len(cmA.AllowedHosts))
		}
		if len(cmB.AllowedHosts) != 0 {
			t.Errorf("simultaneous FIN: cmB AllowedHosts must be empty, got %d", len(cmB.AllowedHosts))
		}
	})
}

// ---------------------------------------------------------------------------
// NukeClient — firewall cleanup on disconnect
//
// Requires the proposed NukeClient changes:
//   - VPLIPToCore[ip] set to nil on disconnect
//   - ClearHost called on all remaining connected clients
// ---------------------------------------------------------------------------

func TestNukeClient_ClearsFirewallOnDisconnect(t *testing.T) {
	// Initialise the VPLIPToCore inner slices for 10.0.0.x
	VPLIPToCore[10] = make([][][]*UserCoreMapping, 1)
	VPLIPToCore[10][0] = make([][]*UserCoreMapping, 256)
	VPLIPToCore[10][0][0] = make([]*UserCoreMapping, 256)

	disconnectingIP := ip4b(10, 0, 0, 5)

	// Client A — the one that will disconnect
	cmA := &UserCoreMapping{
		ToUser:     make(chan []byte, 1),
		FromUser:   make(chan Packet, 1),
		ToSignal:   &signal.Signal{},
		FromSignal: &signal.Signal{},
		DHCP:       &types.DHCPRecord{IP: disconnectingIP},
	}
	clientCoreMappings[0] = cmA
	VPLIPToCore[10][0][0][5] = cmA

	// Client B — has A's IP in its allowed-host list (both auto and manual)
	cmB := &UserCoreMapping{
		ToUser:     make(chan []byte, 1),
		FromUser:   make(chan Packet, 1),
		ToSignal:   &signal.Signal{},
		FromSignal: &signal.Signal{},
	}
	cmB.AddHost(disconnectingIP, pt(0, 80), "auto")
	cmB.AllowedHosts = append(cmB.AllowedHosts, &AllowedHost{
		IP:   disconnectingIP,
		PORT: pt(0, 0),
		Type: "manual",
	})
	clientCoreMappings[1] = cmB

	NukeClient(0)

	// VPLIPToCore must be nil so future LAN packets to A's IP are dropped cleanly
	if VPLIPToCore[10][0][0][5] != nil {
		t.Error("VPLIPToCore entry was not cleared on disconnect")
	}

	// B must no longer have A's IP in any allowed-host entry
	if cmB.IsHostAllowed(disconnectingIP, pt(0, 80)) != nil {
		t.Error("client B still allows auto traffic from disconnected client A")
	}
	if cmB.IsHostAllowed(disconnectingIP, pt(0, 0)) != nil {
		t.Error("client B still allows manual traffic from disconnected client A")
	}

	// Cleanup
	clientCoreMappings[1] = nil
}
