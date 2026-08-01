//go:build unix

package client

import (
	"fmt"
	"os"
	"syscall"
)

// checkFileOwnership returns an error when path exists but is owned by a
// different uid than the current process. Config and tunnel files are
// rewritten in place (rename to .bak + recreate); without this check a client
// started as one user (e.g. root) and later as another would wipe the other
// user's files.
func checkFileOwnership(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		if st.Uid != uint32(os.Getuid()) {
			return fmt.Errorf("%s is owned by uid %d, current uid is %d", path, st.Uid, os.Getuid())
		}
	}
	return nil
}
