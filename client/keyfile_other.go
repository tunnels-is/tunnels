//go:build !unix

package client

import "os"

func validateUserKeyFile(path string, info os.FileInfo) error {
	_ = path
	_ = info
	return nil
}

func warnIfInsecureSecretFile(path string) {
	_ = path
}
