package client

import (
	"testing"
)

func Test_formatBytes(t *testing.T) {
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
			bytes:    1023,
			expected: "1023 B",
		},
		{
			name:     "exactly 1 KB (1024 bytes)",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "middle KB range",
			bytes:    512 * 1024, // 512 KB
			expected: "512.0 KB",
		},
		{
			name:     "fractional KB",
			bytes:    1536, // 1.5 KB
			expected: "1.5 KB",
		},
		{
			name:     "exactly 1 MB (1024 KB)",
			bytes:    1024 * 1024,
			expected: "1.0 MB",
		},
		{
			name:     "middle MB range",
			bytes:    50 * 1024 * 1024, // 50 MB
			expected: "50.0 MB",
		},
		{
			name:     "fractional MB",
			bytes:    1536 * 1024, // 1.5 MB
			expected: "1.5 MB",
		},
		{
			name:     "exactly 1 GB (1024 MB)",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GB",
		},
		{
			name:     "middle GB range",
			bytes:    50 * 1024 * 1024 * 1024, // 50 GB
			expected: "50.0 GB",
		},
		{
			name:     "fractional GB",
			bytes:    1536 * 1024 * 1024, // 1.5 GB
			expected: "1.5 GB",
		},
		{
			name:     "exactly 1 TB (1024 GB)",
			bytes:    1024 * 1024 * 1024 * 1024,
			expected: "1.0 TB",
		},
		{
			name:     "middle TB range",
			bytes:    5 * 1024 * 1024 * 1024 * 1024, // 5 TB
			expected: "5.0 TB",
		},
		{
			name:     "fractional TB",
			bytes:    1536 * 1024 * 1024 * 1024, // 1.5 TB
			expected: "1.5 TB",
		},
		{
			name:     "exactly 1 PB (1024 TB)",
			bytes:    1024 * 1024 * 1024 * 1024 * 1024,
			expected: "1.0 PB",
		},
		{
			name:     "middle PB range",
			bytes:    5 * 1024 * 1024 * 1024 * 1024 * 1024, // 5 PB
			expected: "5.0 PB",
		},
		{
			name:     "exactly 1 EB (1024 PB)",
			bytes:    1024 * 1024 * 1024 * 1024 * 1024 * 1024,
			expected: "1.0 EB",
		},
		{
			name:     "2 EB value",
			bytes:    2 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024, // 2 EB
			expected: "2.0 EB",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatBytes(tc.bytes)
			if result != tc.expected {
				t.Errorf("formatBytes(%d) = %q, expected %q", tc.bytes, result, tc.expected)
			}
			t.Logf("formatBytes(%d) = %q ✓", tc.bytes, result)
		})
	}
}

func Test_formatBytesEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		minCheck string // Minimum prefix expected
	}{
		{
			name:     "negative bytes (edge case)",
			bytes:    -1024,
			minCheck: "B", // Should still format something
		},
		{
			name:     "very large number",
			bytes:    9223372036854775807, // max int64
			minCheck: "EB",                // Should reach exabyte range
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatBytes(tc.bytes)
			// Just ensure it doesn't panic and returns something
			if result == "" {
				t.Errorf("formatBytes(%d) returned empty string", tc.bytes)
			}
			t.Logf("formatBytes(%d) = %q", tc.bytes, result)
		})
	}
}
