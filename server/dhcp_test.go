package main

import (
	"net"
	"testing"
)

func Test_inc(t *testing.T) {
	tests := []struct {
		name     string
		input    net.IP
		expected net.IP
	}{
		{
			name:     "increment last octet - IPv4",
			input:    net.IPv4(192, 168, 1, 1),
			expected: net.IPv4(192, 168, 1, 2),
		},
		{
			name:     "rollover last octet - IPv4",
			input:    net.IPv4(192, 168, 1, 255),
			expected: net.IPv4(192, 168, 2, 0),
		},
		{
			name:     "rollover multiple octets - IPv4",
			input:    net.IPv4(192, 168, 255, 255),
			expected: net.IPv4(192, 169, 0, 0),
		},
		{
			name:     "increment from zero - IPv4",
			input:    net.IPv4(0, 0, 0, 0),
			expected: net.IPv4(0, 0, 0, 1),
		},
		{
			name:     "increment middle range - IPv4",
			input:    net.IPv4(10, 0, 0, 254),
			expected: net.IPv4(10, 0, 0, 255),
		},
		{
			name:     "private network increment - IPv4",
			input:    net.IPv4(172, 16, 0, 99),
			expected: net.IPv4(172, 16, 0, 100),
		},
		{
			name:     "increment IPv6 last byte",
			input:    net.ParseIP("2001:db8::1"),
			expected: net.ParseIP("2001:db8::2"),
		},
		{
			name:     "rollover IPv6 last byte",
			input:    net.ParseIP("2001:db8::ff"),
			expected: net.ParseIP("2001:db8::100"),
		},
		{
			name:     "IPv6 with multiple bytes",
			input:    net.ParseIP("2001:db8::ffff"),
			expected: net.ParseIP("2001:db8::1:0"),
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

func Test_incSequence(t *testing.T) {
	// Test incrementing through a sequence
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

	t.Log("Sequential increment test passed ✓")
}

func Test_incIPv6Sequence(t *testing.T) {
	// Test IPv6 incrementing through a sequence
	ip := net.ParseIP("2001:db8::fd")

	expected := []string{
		"2001:db8::fe",
		"2001:db8::ff",
		"2001:db8::100",
		"2001:db8::101",
	}

	for i, exp := range expected {
		inc(ip)
		if ip.String() != exp {
			t.Errorf("After %d increments: got %v, expected %s", i+1, ip, exp)
		}
	}

	t.Log("IPv6 sequential increment test passed ✓")
}

func Test_incLargeRollover(t *testing.T) {
	// Test rolling over large sections
	tests := []struct {
		name     string
		input    net.IP
		expected net.IP
	}{
		{
			name:     "rollover first three octets - IPv4",
			input:    net.IPv4(10, 255, 255, 255),
			expected: net.IPv4(11, 0, 0, 0),
		},
		{
			name:     "rollover middle octets - IPv4",
			input:    net.IPv4(192, 255, 255, 255),
			expected: net.IPv4(193, 0, 0, 0),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
