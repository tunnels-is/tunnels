//go:build !unix

package client

func checkFileOwnership(path string) error {
	return nil
}
