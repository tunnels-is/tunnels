//go:build !unix

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

func warnIfInsecureSecretFile(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if err := validateUserKeyFile(path, info); err != nil {
		SECURITY("insecure secret file permissions (continuing):", path, "—", err)
	}
}
