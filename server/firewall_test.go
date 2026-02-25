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
