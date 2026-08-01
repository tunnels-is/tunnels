//go:build !unix

package wgserver

import "os"

func checkKeyFileOwner(path string, info os.FileInfo) error { return nil }
