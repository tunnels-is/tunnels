//go:build unix

package wgserver

import (
	"fmt"
	"os"
	"syscall"
)

// checkKeyFileOwner rejects a private-key file owned by another user. Mode
// bits alone don't establish trust: a pre-planted file with 0600 permissions
// but a different owner would otherwise be loaded as this server's identity.
func checkKeyFileOwner(info os.FileInfo) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if st.Uid != uint32(os.Getuid()) {
		return fmt.Errorf("owned by uid %d, expected uid %d", st.Uid, os.Getuid())
	}
	return nil
}
