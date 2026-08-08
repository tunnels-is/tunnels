package crypt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckKeyFilePermissions(t *testing.T) {
	dir := t.TempDir()

	secure := filepath.Join(dir, "secure.pem")
	if err := os.WriteFile(secure, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckKeyFilePermissions(secure); err != nil {
		t.Fatalf("0600 should pass: %v", err)
	}

	loose := filepath.Join(dir, "loose.pem")
	if err := os.WriteFile(loose, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CheckKeyFilePermissions(loose); err == nil {
		t.Fatal("0644 should fail permission check")
	}
}
