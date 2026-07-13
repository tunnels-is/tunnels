//go:build !unix

package wgserver

import "os"

func checkKeyFileOwner(info os.FileInfo) error { return nil }
