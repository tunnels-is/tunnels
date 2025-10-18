package client

import (
	"net"
	"testing"
)

func TestInc(t *testing.T) {
	tests := []struct {
		name     string
		input    net.IP
		expected net.IP
	}{
		{
			name:     "increment last octet",
			input:    net.IPv4(192, 168, 1, 1),
			expected: net.IPv4(192, 168, 1, 2),
		},
		{
			name:     "rollover last octet",
			input:    net.IPv4(192, 168, 1, 255),
			expected: net.IPv4(192, 168, 2, 0),
		},
		{
			name:     "rollover multiple octets",
			input:    net.IPv4(192, 168, 255, 255),
			expected: net.IPv4(192, 169, 0, 0),
		},
		{
			name:     "increment from zero",
			input:    net.IPv4(0, 0, 0, 0),
			expected: net.IPv4(0, 0, 0, 1),
		},
		{
			name:     "increment middle range",
			input:    net.IPv4(10, 0, 0, 254),
			expected: net.IPv4(10, 0, 0, 255),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy since inc modifies in place
			ip := make(net.IP, len(tc.input))
			copy(ip, tc.input)

			inc(ip)

			if !ip.Equal(tc.expected) {
				t.Errorf("inc(%v) = %v, expected %v", tc.input, ip, tc.expected)
			}

			t.Logf("inc(%v) = %v ✓", tc.input, ip)
		})
	}
}

func TestIncIPv6(t *testing.T) {
	// Test with IPv6 addresses
	tests := []struct {
		name     string
		input    net.IP
		expected net.IP
	}{
		{
			name:     "IPv6 increment last byte",
			input:    net.ParseIP("2001:db8::1"),
			expected: net.ParseIP("2001:db8::2"),
		},
		{
			name:     "IPv6 rollover last byte",
			input:    net.ParseIP("2001:db8::ff"),
			expected: net.ParseIP("2001:db8::100"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Make a copy since inc modifies in place
			ip := make(net.IP, len(tc.input))
			copy(ip, tc.input)

			inc(ip)

			if !ip.Equal(tc.expected) {
				t.Errorf("inc(%v) = %v, expected %v", tc.input, ip, tc.expected)
			}

			t.Logf("inc(%v) = %v ✓", tc.input, ip)
		})
	}
}

func TestIncSequence(t *testing.T) {
	// Test incrementing multiple times
	ip := net.IPv4(192, 168, 1, 250)

	expected := []string{
		"192.168.1.251",
		"192.168.1.252",
		"192.168.1.253",
		"192.168.1.254",
		"192.168.1.255",
		"192.168.2.0",
		"192.168.2.1",
	}

	for i, exp := range expected {
		inc(ip)
		if ip.String() != exp {
			t.Errorf("After %d increments: got %v, expected %s", i+1, ip, exp)
		}
	}

	t.Logf("Sequential increment test passed ✓")
}
