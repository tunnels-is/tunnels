package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/puzpuzpuz/xsync/v3"
)

func TestLoadDomainsFromReader(t *testing.T) {
	input := strings.Join([]string{
		"example.com",
		"# comment",
		"nodot",
		"",
		"ads.tracker.net",
		"ok.co.uk",
	}, "\n")

	nm := xsync.NewMapOf[string, bool]()
	count, bad, err := loadDomainsFromReader(strings.NewReader(input), true, nm)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count=%d want 3", count)
	}
	// comment, nodot, empty
	if bad != 3 {
		t.Fatalf("bad=%d want 3", bad)
	}
	for _, d := range []string{"example.com", "ads.tracker.net", "ok.co.uk"} {
		if _, ok := nm.Load(d); !ok {
			t.Fatalf("missing domain %q", d)
		}
	}
	if _, ok := nm.Load("nodot"); ok {
		t.Fatal("should not store invalid domain")
	}
}

func TestLoadDomainsFromReader_disabledDoesNotStore(t *testing.T) {
	nm := xsync.NewMapOf[string, bool]()
	count, _, err := loadDomainsFromReader(strings.NewReader("a.example.com\nb.example.com\n"), false, nm)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count=%d want 2", count)
	}
	n := 0
	nm.Range(func(string, bool) bool { n++; return true })
	if n != 0 {
		t.Fatalf("stored %d entries when disabled, want 0", n)
	}
}

func TestLoadDomainsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ads")
	content := "one.example.com\ntwo.example.com\n#x\nbad\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	nm := xsync.NewMapOf[string, bool]()
	count, bad, err := loadDomainsFromFile(path, true, nm)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || bad != 2 {
		t.Fatalf("count=%d bad=%d", count, bad)
	}
}
