//go:build unix

package wgserver

import (
	"fmt"
	"os"
	"syscall"
)

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
