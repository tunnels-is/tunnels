// go build linux
package client

func closeAllOpenTCPconnections() (err error) {
	// Defined to satisfy cross-platform compilation
	// No-op on Linux, as connections are managed by the kernel and closed automatically
	// when the process exits or the socket is closed.
	return nil
}
