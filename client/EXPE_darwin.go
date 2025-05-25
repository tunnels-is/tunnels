//go:build darwin

package client

func closeAllOpenTCPconnections() (err error) {
	// No-op on Darwin (macOS), as connections are managed by the kernel and closed automatically
	// when the process exits or the socket is closed.
	return nil
}
