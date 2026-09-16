package client

import (
	"testing"
)

func TestBandwidthBytesToString(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "single byte",
			bytes:    1,
			expected: "1 B",
		},
		{
			name:     "max bytes without conversion",
			bytes:    999,
			expected: "999 B",
		},
		{
			name:     "exactly 1000 bytes - kilobytes",
			bytes:    1000,
			expected: "1 KB",
		},
		{
			name:     "middle KB range",
			bytes:    500_000,
			expected: "500 KB",
		},
		{
			name:     "max kilobytes",
			bytes:    999_999,
			expected: "1000 KB",
		},
		{
			name:     "exactly 1 MB",
			bytes:    1_000_000,
			expected: "1.0 MB",
		},
		{
			name:     "middle MB range",
			bytes:    50_000_000,
			expected: "50.0 MB",
		},
		{
			name:     "fractional MB",
			bytes:    1_500_000,
			expected: "1.5 MB",
		},
		{
			name:     "max megabytes",
			bytes:    999_999_999,
			expected: "1000.0 MB",
		},
		{
			name:     "exactly 1 GB",
			bytes:    1_000_000_000,
			expected: "1.0 GB",
		},
		{
			name:     "middle GB range",
			bytes:    50_000_000_000,
			expected: "50.0 GB",
		},
		{
			name:     "fractional GB",
			bytes:    2_500_000_000,
			expected: "2.5 GB",
		},
		{
			name:     "max gigabytes",
			bytes:    999_999_999_999,
			expected: "1000.0 GB",
		},
		{
			name:     "exactly 1 TB",
			bytes:    1_000_000_000_000,
			expected: "1.0 TB",
		},
		{
			name:     "middle TB range",
			bytes:    5_000_000_000_000,
			expected: "5.0 TB",
		},
		{
			name:     "fractional TB",
			bytes:    1_500_000_000_000,
			expected: "1.5 TB",
		},
		{
			name:     "max terabytes",
			bytes:    999_999_999_999_999,
			expected: "1000.0 TB",
		},
		{
			name:     "beyond TB range",
			bytes:    1_000_000_000_000_000,
			expected: "???",
		},
		{
			name:     "very large value",
			bytes:    9_999_999_999_999_999,
			expected: "???",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := BandwidthBytesToString(tc.bytes)
			if result != tc.expected {
				t.Errorf("BandwidthBytesToString(%d) = %q, expected %q", tc.bytes, result, tc.expected)
			}
		})
	}
}
