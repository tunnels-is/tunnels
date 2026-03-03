//go:build windows

package client

import (
	"testing"
)

func Test_ipString(t *testing.T) {
	tests := []struct {
		name     string
		ip       uint32
		expected string
	}{
		{
			name:     "localhost 127.0.0.1",
			ip:       0x7F000001,
			expected: "127.0.0.1",
		},
		{
			name:     "zero address 0.0.0.0",
			ip:       0x00000000,
			expected: "0.0.0.0",
		},
		{
			name:     "broadcast 255.255.255.255",
			ip:       0xFFFFFFFF,
			expected: "255.255.255.255",
		},
		{
			name:     "private network 192.168.1.1",
			ip:       0xC0A80101,
			expected: "192.168.1.1",
		},
		{
			name:     "private network 10.0.0.1",
			ip:       0x0A000001,
			expected: "10.0.0.1",
		},
		{
			name:     "private network 172.16.0.1",
			ip:       0xAC100001,
			expected: "172.16.0.1",
		},
		{
			name:     "Google DNS 8.8.8.8",
			ip:       0x08080808,
			expected: "8.8.8.8",
		},
		{
			name:     "Cloudflare DNS 1.1.1.1",
			ip:       0x01010101,
			expected: "1.1.1.1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ipString(tc.ip)
			if result != tc.expected {
				t.Errorf("ipString(0x%08X) = %q, expected %q", tc.ip, result, tc.expected)
			}
			t.Logf("ipString(0x%08X) = %q ✓", tc.ip, result)
		})
	}
}

func Test_portString(t *testing.T) {
	tests := []struct {
		name     string
		port     uint32
		expected string
	}{
		{
			name:     "HTTP port 80",
			port:     0x00005000,
			expected: "80",
		},
		{
			name:     "HTTPS port 443",
			port:     0x0000BB01,
			expected: "443",
		},
		{
			name:     "SSH port 22",
			port:     0x00001600,
			expected: "22",
		},
		{
			name:     "DNS port 53",
			port:     0x00003500,
			expected: "53",
		},
		{
			name:     "high port 8080",
			port:     0x0000901F,
			expected: "8080",
		},
		{
			name:     "port 1",
			port:     0x00000100,
			expected: "1",
		},
		{
			name:     "max port 65535",
			port:     0x0000FFFF,
			expected: "65535",
		},
		{
			name:     "port 0",
			port:     0x00000000,
			expected: "0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := portString(tc.port)
			if result != tc.expected {
				t.Errorf("portString(0x%08X) = %q, expected %q", tc.port, result, tc.expected)
			}
			t.Logf("portString(0x%08X) = %q ✓", tc.port, result)
		})
	}
}

func Test_ipString_RoundTrip(t *testing.T) {

	knownIPs := []struct {
		ipString string
		ipUint32 uint32
	}{
		{"192.168.1.100", 0xC0A80164},
		{"10.20.30.40", 0x0A141E28},
		{"172.31.255.254", 0xAC1FFFFE},
	}

	for _, tc := range knownIPs {
		t.Run(tc.ipString, func(t *testing.T) {
			result := ipString(tc.ipUint32)
			if result != tc.ipString {
				t.Errorf("ipString(0x%08X) = %q, expected %q", tc.ipUint32, result, tc.ipString)
			}
		})
	}
}

func Test_portString_CommonPorts(t *testing.T) {

	commonPorts := map[string]uint32{
		"21":   0x00001500,
		"22":   0x00001600,
		"23":   0x00001700,
		"25":   0x00001900,
		"80":   0x00005000,
		"110":  0x00006E00,
		"143":  0x00008F00,
		"443":  0x0000BB01,
		"3389": 0x00003D0D,
	}

	for expectedPort, portValue := range commonPorts {
		t.Run("port_"+expectedPort, func(t *testing.T) {
			result := portString(portValue)
			if result != expectedPort {
				t.Errorf("portString(0x%08X) = %q, expected %q", portValue, result, expectedPort)
			}
		})
	}
}
