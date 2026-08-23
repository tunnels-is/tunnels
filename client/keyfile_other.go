//go:build !unix && !windows

package client

import (
	"fmt"
	"os"
)

func validateUserKeyFile(path string, info os.FileInfo) error {
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		return fmt.Errorf("insecure permissions %o (want 0600)", mode)
	}
	return nil
}

func secureSecretFile(path string) error {
	return os.Chmod(path, 0o600)
}
