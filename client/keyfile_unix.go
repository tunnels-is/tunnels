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

func secureSecretFile(path string) error {
	return os.Chmod(path, 0o600)
}
