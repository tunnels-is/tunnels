//go:build unix

package wgserver

import (
	"fmt"
	"os"
	"syscall"
)

func checkKeyFileOwner(path string, info os.FileInfo) error {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if st.Uid != uint32(os.Getuid()) {
		err := fmt.Errorf("owned by uid %d, expected uid %d", st.Uid, os.Getuid())
		ERR("file permission check failed:", path, "—", err)
		return err
	}
	return nil
}
