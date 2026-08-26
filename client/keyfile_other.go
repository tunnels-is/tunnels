//go:build !unix

package client

import "os"

func validateUserKeyFile(path string, info os.FileInfo) error {
	// Windows (and other non-unix) filesystems do not store Unix permission
	// bits. os.FileMode.Perm() is typically 0666 even for files we wrote with
	// 0600, so a unix-style group/other check is a false positive on every
	// secret file.
	_ = path
	_ = info
	return nil
}

func warnIfInsecureSecretFile(path string) {
	_ = path
}
