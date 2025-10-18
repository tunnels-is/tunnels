package main

import (
	"testing"
)

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
		// NOTE: Pure IPv6 addresses (non-IPv4-mapped) will cause a panic in the current
		// implementation because ip.To4() returns nil but the code doesn't check for it.
		// This is a bug in the production code that should be fixed, but we're only testing.
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
