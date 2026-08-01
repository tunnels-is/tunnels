//go:build unix

package client

import (
	"fmt"
	"os"
	"syscall"
)

func validateUserKeyFile(path string, info os.FileInfo) error {
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		err := fmt.Errorf("insecure permissions %o (want 0600)", mode)
		ERROR("file permission check failed:", path, "—", err)
		return err
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		if st.Uid != uint32(os.Getuid()) {
			err := fmt.Errorf("owned by uid %d, expected uid %d", st.Uid, os.Getuid())
			ERROR("file permission check failed:", path, "—", err)
			return err
		}
	}
	return nil
}
