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
			ip:       0x7F000001, // 127.0.0.1 in big-endian
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
			ip:       0xC0A80101, // 192.168.1.1 in big-endian
			expected: "192.168.1.1",
		},
		{
			name:     "private network 10.0.0.1",
			ip:       0x0A000001, // 10.0.0.1 in big-endian
			expected: "10.0.0.1",
		},
		{
			name:     "private network 172.16.0.1",
			ip:       0xAC100001, // 172.16.0.1 in big-endian
			expected: "172.16.0.1",
		},
		{
			name:     "Google DNS 8.8.8.8",
			ip:       0x08080808, // 8.8.8.8 in big-endian
			expected: "8.8.8.8",
		},
		{
			name:     "Cloudflare DNS 1.1.1.1",
			ip:       0x01010101, // 1.1.1.1 in big-endian
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
			port:     0x00005000, // Port 80 in network byte order (big-endian in last 2 bytes)
			expected: "80",
		},
		{
			name:     "HTTPS port 443",
			port:     0x0000BB01, // Port 443 in network byte order
			expected: "443",
		},
		{
			name:     "SSH port 22",
			port:     0x00001600, // Port 22 in network byte order
			expected: "22",
		},
		{
			name:     "DNS port 53",
			port:     0x00003500, // Port 53 in network byte order
			expected: "53",
		},
		{
			name:     "high port 8080",
			port:     0x0000901F, // Port 8080 in network byte order
			expected: "8080",
		},
		{
			name:     "port 1",
			port:     0x00000100, // Port 1 in network byte order
			expected: "1",
		},
		{
			name:     "max port 65535",
			port:     0x0000FFFF, // Port 65535 in network byte order
			expected: "65535",
		},
		{
			name:     "port 0",
			port:     0x00000000, // Port 0
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
	// Test that we can convert known IP addresses correctly
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
	// Test common well-known ports
	commonPorts := map[string]uint32{
		"21":   0x00001500, // FTP
		"22":   0x00001600, // SSH
		"23":   0x00001700, // Telnet
		"25":   0x00001900, // SMTP
		"80":   0x00005000, // HTTP
		"110":  0x00006E00, // POP3
		"143":  0x00008F00, // IMAP
		"443":  0x0000BB01, // HTTPS
		"3389": 0x00003D0D, // RDP
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
