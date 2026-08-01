package wgserver

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
