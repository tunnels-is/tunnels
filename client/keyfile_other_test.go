//go:build !unix

package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateUserKeyFile_IgnoresUnixModeBits(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "user")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUserKeyFile(p, info); err != nil {
		t.Fatalf("non-unix should not treat mode %o as insecure: %v", info.Mode().Perm(), err)
	}
	warnIfInsecureSecretFile(p)
	warnIfInsecureSecretFile(filepath.Join(dir, "missing"))
}
