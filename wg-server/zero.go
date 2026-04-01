package wgserver

// zeroBytes overwrites every byte in b with zero.
// Used to scrub key material from memory as soon as it is no longer needed.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
