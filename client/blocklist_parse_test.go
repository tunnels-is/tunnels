package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	set := NewDomainSet(8)
	count, bad, err := loadDomainsFromReader(strings.NewReader(input), true, set)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("count=%d want 3", count)
	}
	if bad != 3 {
		t.Fatalf("bad=%d want 3", bad)
	}
	for _, d := range []string{"example.com", "ads.tracker.net", "ok.co.uk"} {
		if !set.Has(d) {
			t.Fatalf("missing domain %q", d)
		}
	}
	if set.Has("nodot") {
		t.Fatal("should not store invalid domain")
	}
}

func TestLoadDomainsFromReader_disabledDoesNotStore(t *testing.T) {
	set := NewDomainSet(8)
	count, _, err := loadDomainsFromReader(strings.NewReader("a.example.com\nb.example.com\n"), false, set)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count=%d want 2", count)
	}
	if set.Len() != 0 {
		t.Fatalf("stored %d entries when disabled, want 0", set.Len())
	}
}

func TestLoadDomainsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ads")
	content := "one.example.com\ntwo.example.com\n#x\nbad\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	set := NewDomainSet(8)
	count, bad, err := loadDomainsFromFile(path, true, set)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || bad != 2 {
		t.Fatalf("count=%d bad=%d", count, bad)
	}
}

func TestDomainSet_MergeFrom(t *testing.T) {
	a := NewDomainSet(4)
	a.Add("a.example.com")
	b := NewDomainSet(4)
	b.Add("b.example.com")
	a.MergeFrom(b)
	if !a.Has("a.example.com") || !a.Has("b.example.com") {
		t.Fatal("merge failed")
	}
	if a.Len() != 2 {
		t.Fatalf("len=%d", a.Len())
	}
}
