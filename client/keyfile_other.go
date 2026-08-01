//go:build !unix

package client

import "os"

func validateUserKeyFile(path string, info os.FileInfo) error {
	return nil
}
