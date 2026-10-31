package client

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallFullListReplacesOnCompleteBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := "one.example\ntwo.example\n"
	if err := installFullList(path, strings.NewReader(body), int64(len(body))); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("installed %q", got)
	}
	if err := installFullList(path, strings.NewReader(body), -1); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFullListKeepsPreviousWhenShort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	body := "short\n"
	if err := installFullList(path, strings.NewReader(body), int64(len(body)+10)); err == nil {
		t.Fatal("short body was installed")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old\n" {
		t.Fatalf("previous list changed to %q", got)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file left behind")
	}
}

func TestInstallFullListKeepsPreviousOnCopyError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := installFullList(path, &failAfter{n: 8}, -1); err == nil {
		t.Fatal("dropped download was installed")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old\n" {
		t.Fatalf("previous list changed to %q", got)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file left behind")
	}
}

func TestInstallFullListDoesNotCreatePartialFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	if err := installFullList(path, strings.NewReader("x"), 5); err == nil {
		t.Fatal("short body created a list")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("partial list was created")
	}
}

type failAfter struct {
	n int
}

func (f *failAfter) Read(p []byte) (int, error) {
	if f.n <= 0 {
		return 0, errors.New("dropped")
	}
	if len(p) > f.n {
		p = p[:f.n]
	}
	for i := range p {
		p[i] = 'a'
	}
	n := len(p)
	f.n -= n
	if f.n == 0 {
		return n, errors.New("dropped")
	}
	return n, nil
}

var _ io.Reader = (*failAfter)(nil)
