//go:build !unix

package client

import "os"

func validateUserKeyFile(info os.FileInfo) error {
	return nil
}
