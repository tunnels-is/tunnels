package certs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenPrivateKeyFile_Mode0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.pem")

	// Pre-create with loose perms — openPrivateKeyFile should force 0600.
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := openPrivateKeyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("newkey"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		t.Fatalf("expected 0600-ish key file, got %o", mode)
	}
}

func TestMakeCert_WritesKeyWith0600(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "api.crt")
	keyPath := filepath.Join(dir, "api.key")

	_, err := MakeCert(ECDSA, certPath, keyPath, []string{"127.0.0.1"}, nil, "test", time.Time{}, true)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		t.Fatalf("MakeCert key should be 0600, got %o", mode)
	}
}
