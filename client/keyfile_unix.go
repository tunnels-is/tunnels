//go:build unix

package client

import (
	"fmt"
	"os"
	"syscall"
)

func validateUserKeyFile(path string, info os.FileInfo) error {
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

// warnIfInsecureSecretFile logs a SECURITY warning when a secret file is
// group/other-readable or not owned by the current user. It does not block load.
func warnIfInsecureSecretFile(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if err := validateUserKeyFile(path, info); err != nil {
		SECURITY("insecure secret file permissions (continuing):", path, "—", err)
	}
}
