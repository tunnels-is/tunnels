//go:build unix

package client

import (
	"fmt"
	"os"
	"syscall"
)

// validateUserKeyFile rejects a user-key file that is group/other-accessible or
// owned by another user. The file holds the AES key that encrypts saved
// credentials (device tokens, API keys); a file pre-planted by a local attacker
// (so they know the key) or left world-readable must not be trusted. Mirrors
// the wg-server .pk ownership check.
func validateUserKeyFile(info os.FileInfo) error {
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		return fmt.Errorf("insecure permissions %o (want 0600)", mode)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		if st.Uid != uint32(os.Getuid()) {
			return fmt.Errorf("owned by uid %d, expected uid %d", st.Uid, os.Getuid())
		}
	}
	return nil
}
