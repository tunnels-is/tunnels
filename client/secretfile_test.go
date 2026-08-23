package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSecretFile_Validates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user")
	if err := writeSecretFile(path, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUserKeyFile(path, info); err != nil {
		t.Fatalf("writeSecretFile must produce a file that validates: %v", err)
	}
}

func TestWriteSecretFile_TightensExistingLooseFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeSecretFile(path, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUserKeyFile(path, info); err != nil {
		t.Fatalf("rewriting a loose file must leave it owner-only: %v", err)
	}
}

func TestWarnIfInsecureSecretFile_RepairsLoosePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user")
	if err := os.WriteFile(path, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	warnIfInsecureSecretFile(path)
	warnIfInsecureSecretFile(filepath.Join(t.TempDir(), "missing"))

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateUserKeyFile(path, info); err != nil {
		t.Fatalf("expected repair of secret file permissions: %v", err)
	}
}
