//go:build unix

package client

import (
	"fmt"
	"os"
	"syscall"
)

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
			err := fmt.Errorf("%s is owned by uid %d, current uid is %d", path, st.Uid, os.Getuid())
			ERROR("file permission check failed:", path, "—", err)
			return err
		}
	}
	return nil
}
