//go:build !unix

package client

import "os"

// validateUserKeyFile is a no-op on non-unix platforms. Windows uses ACLs rather
// than unix mode/owner bits, and the default install directory there is
// system-protected, so the plant-a-key threat the unix check defends against
// does not map cleanly onto file mode/uid.
func validateUserKeyFile(info os.FileInfo) error {
	return nil
}
