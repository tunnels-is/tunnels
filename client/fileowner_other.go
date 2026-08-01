//go:build !unix

package client

// checkFileOwnership is a no-op on non-unix platforms; Windows uses ACLs
// rather than unix uid ownership.
func checkFileOwnership(path string) error {
	return nil
}
