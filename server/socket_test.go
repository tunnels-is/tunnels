package main

import (
	"net"
	"testing"
)

func TestIS_LOCAL(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		// Loopback addresses
		{
			name:     "IPv4 loopback 127.0.0.1",
			ip:       "127.0.0.1",
			expected: true,
		},
		{
			name:     "IPv4 loopback 127.0.0.2",
			ip:       "127.0.0.2",
			expected: true,
		},
		{
			name:     "IPv6 loopback ::1",
			ip:       "::1",
			expected: true,
		},

		// Private IPv4 addresses
		{
			name:     "Private 10.0.0.1",
			ip:       "10.0.0.1",
			expected: true,
		},
		{
			name:     "Private 10.255.255.255",
			ip:       "10.255.255.255",
			expected: true,
		},
		{
			name:     "Private 172.16.0.1",
			ip:       "172.16.0.1",
			expected: true,
		},
		{
			name:     "Private 172.31.255.255",
			ip:       "172.31.255.255",
			expected: true,
		},
		{
			name:     "Private 192.168.0.1",
			ip:       "192.168.0.1",
			expected: true,
		},
		{
			name:     "Private 192.168.255.255",
			ip:       "192.168.255.255",
			expected: true,
		},

		// Link-local addresses
		{
			name:     "Link-local 169.254.0.1",
			ip:       "169.254.0.1",
			expected: true,
		},
		{
			name:     "Link-local 169.254.255.254",
			ip:       "169.254.255.254",
			expected: true,
		},
		{
			name:     "IPv6 link-local fe80::1",
			ip:       "fe80::1",
			expected: true,
		},

		// Public IPv4 addresses (should be false)
		{
			name:     "Public Google DNS 8.8.8.8",
			ip:       "8.8.8.8",
			expected: false,
		},
		{
			name:     "Public Cloudflare DNS 1.1.1.1",
			ip:       "1.1.1.1",
			expected: false,
		},
		{
			name:     "Public 93.184.216.34 (example.com)",
			ip:       "93.184.216.34",
			expected: false,
		},
		{
			name:     "Public 151.101.1.69",
			ip:       "151.101.1.69",
			expected: false,
		},

		// Public IPv6 addresses (should be false)
		{
			name:     "Public IPv6 2001:4860:4860::8888 (Google DNS)",
			ip:       "2001:4860:4860::8888",
			expected: false,
		},
		{
			name:     "Public IPv6 2606:4700:4700::1111 (Cloudflare DNS)",
			ip:       "2606:4700:4700::1111",
			expected: false,
		},

		// IPv6 unique local addresses (private)
		{
			name:     "IPv6 ULA fc00::1",
			ip:       "fc00::1",
			expected: true,
		},
		{
			name:     "IPv6 ULA fd00::1",
			ip:       "fd00::1",
			expected: true,
		},

		// Multicast addresses
		{
			name:     "IPv4 multicast 224.0.0.1",
			ip:       "224.0.0.1",
			expected: true, // Link-local multicast is considered local
		},
		{
			name:     "IPv6 multicast ff02::1",
			ip:       "ff02::1",
			expected: true, // Interface-local/link-local multicast
		},

		// Edge cases
		{
			name:     "Broadcast 255.255.255.255",
			ip:       "255.255.255.255",
			expected: false,
		},
		{
			name:     "Zero address 0.0.0.0",
			ip:       "0.0.0.0",
			expected: false,
		},
		{
			name:     "IPv6 zero address ::",
			ip:       "::",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tc.ip)
			}

			result := IS_LOCAL(ip)
			if result != tc.expected {
				t.Errorf("IS_LOCAL(%s) = %v, expected %v", tc.ip, result, tc.expected)
			}

			t.Logf("IS_LOCAL(%s) = %v ✓", tc.ip, result)
		})
	}
}

func TestIS_LOCAL_PrivateRanges(t *testing.T) {
	// Test comprehensive coverage of private ranges
	privateRanges := []struct {
		name  string
		start string
		end   string
	}{
		{
			name:  "10.0.0.0/8",
			start: "10.0.0.0",
			end:   "10.255.255.255",
		},
		{
			name:  "172.16.0.0/12",
			start: "172.16.0.0",
			end:   "172.31.255.255",
		},
		{
			name:  "192.168.0.0/16",
			start: "192.168.0.0",
			end:   "192.168.255.255",
		},
	}

	for _, pr := range privateRanges {
		t.Run(pr.name, func(t *testing.T) {
			// Test start of range
			startIP := net.ParseIP(pr.start)
			if !IS_LOCAL(startIP) {
				t.Errorf("IS_LOCAL(%s) should be true (start of %s)", pr.start, pr.name)
			}

			// Test end of range
			endIP := net.ParseIP(pr.end)
			if !IS_LOCAL(endIP) {
				t.Errorf("IS_LOCAL(%s) should be true (end of %s)", pr.end, pr.name)
			}

			t.Logf("Private range %s correctly identified as local ✓", pr.name)
		})
	}
}

func TestIS_LOCAL_PublicRanges(t *testing.T) {
	// Test that various public IPs are correctly identified as non-local
	publicIPs := []string{
		"8.8.8.8",         // Google DNS
		"1.1.1.1",         // Cloudflare DNS
		"208.67.222.222",  // OpenDNS
		"9.9.9.9",         // Quad9
		"76.76.19.19",     // Alternate DNS
		"185.228.168.168", // CleanBrowsing
	}

	for _, ip := range publicIPs {
		t.Run(ip, func(t *testing.T) {
			parsed := net.ParseIP(ip)
			if IS_LOCAL(parsed) {
				t.Errorf("IS_LOCAL(%s) should be false (public IP)", ip)
			}
			t.Logf("Public IP %s correctly identified as non-local ✓", ip)
		})
	}
}

func TestIS_LOCAL_Loopback127Range(t *testing.T) {
	// Test multiple IPs in the 127.0.0.0/8 range
	for i := 0; i < 256; i += 50 {
		ip := net.IPv4(127, 0, 0, byte(i))
		if !IS_LOCAL(ip) {
			t.Errorf("IS_LOCAL(127.0.0.%d) should be true (loopback)", i)
		}
	}
	t.Log("Loopback range 127.0.0.0/8 correctly identified ✓")
}
