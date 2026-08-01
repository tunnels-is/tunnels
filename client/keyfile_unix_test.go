//go:build unix

package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateUserKeyFile(t *testing.T) {
	dir := t.TempDir()

	secure := filepath.Join(dir, "secure.key")
	if err := os.WriteFile(secure, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(secure)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUserKeyFile(secure, info); err != nil {
		t.Fatalf("0600 file owned by us should validate: %v", err)
	}

	insecure := filepath.Join(dir, "insecure.key")
	if err := os.WriteFile(insecure, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(insecure)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUserKeyFile(insecure, info); err == nil {
		t.Fatal("world-readable key file must be rejected")
	}
}
