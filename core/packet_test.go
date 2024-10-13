package core

import "testing"

func BenchmarkOriginalIPChecksum(b *testing.B) {
	header := []byte{69, 32, 0, 40, 0, 0, 64, 0, 54, 6, 106, 16, 89, 147, 109, 61, 172, 22, 22, 22}
	for i := 0; i < b.N; i++ {
		RecalculateAndReplaceIPv4HeaderChecksum(header)
	}
}
