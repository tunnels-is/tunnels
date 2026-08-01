package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileWithBackupCreatesNewFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tunnels.conf")

	if err := writeFileWithBackup(path, []byte("first")); err != nil {
		t.Fatalf("writeFileWithBackup failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}
	if string(got) != "first" {
		t.Errorf("file content = %q, want %q", got, "first")
	}
	if _, err := os.Stat(path + backupFileSuffix); !os.IsNotExist(err) {
		t.Error("no backup should exist when the file was just created")
	}
}

func TestWriteFileWithBackupSkipsIdenticalContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tunnels.conf")
	if err := os.WriteFile(path, []byte("same"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeFileWithBackup(path, []byte("same")); err != nil {
		t.Fatalf("writeFileWithBackup failed: %v", err)
	}

	if _, err := os.Stat(path + backupFileSuffix); !os.IsNotExist(err) {
		t.Error("backup must not be created when content is unchanged")
	}
}

func TestWriteFileWithBackupBacksUpChangedContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tunnels.conf")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeFileWithBackup(path, []byte("new")); err != nil {
		t.Fatalf("writeFileWithBackup failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("file content = %q, want %q", got, "new")
	}

	bak, err := os.ReadFile(path + backupFileSuffix)
	if err != nil {
		t.Fatalf("backup should exist after content change: %v", err)
	}
	if string(bak) != "old" {
		t.Errorf("backup content = %q, want %q", bak, "old")
	}
}

func TestWriteFileWithBackupRefusesForeignOwner(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root to chown the test file to another uid")
	}
	path := filepath.Join(t.TempDir(), "tunnels.conf")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(path, 12345, 12345); err != nil {
		t.Fatal(err)
	}

	if err := writeFileWithBackup(path, []byte("new")); err == nil {
		t.Fatal("writeFileWithBackup must refuse to modify a file owned by another user")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Errorf("file was modified despite foreign ownership: %q", got)
	}
	if _, err := os.Stat(path + backupFileSuffix); !os.IsNotExist(err) {
		t.Error("no backup should be created for a foreign-owned file")
	}
}
