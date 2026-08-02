package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDomainSetFromReader(t *testing.T) {
	input := strings.Join([]string{
		"example.com",
		"# comment",
		"nodot",
		"",
		"ads.tracker.net",
		"ok.co.uk",
		"0.0.0.0 hosts.style.example",
		"EXAMPLE.COM", // duplicate of example.com after normalize
	}, "\n")

	set, count, bad, err := loadDomainSetFromReader(strings.NewReader(input), 8)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 { // example.com, ads..., ok..., hosts..., EXAMPLE.COM
		t.Fatalf("count=%d want 5", count)
	}
	if bad != 3 {
		t.Fatalf("bad=%d want 3", bad)
	}
	// unique after pack
	if set.Len() != 4 {
		t.Fatalf("unique len=%d want 4", set.Len())
	}
	for _, d := range []string{"example.com", "ads.tracker.net", "ok.co.uk", "hosts.style.example"} {
		if !set.Has(d) {
			t.Fatalf("missing domain %q", d)
		}
	}
	if set.Has("nodot") {
		t.Fatal("should not store invalid domain")
	}
	// case-insensitive lookup
	if !set.Has("Example.COM") {
		t.Fatal("expected case-insensitive match")
	}
}

func TestLoadDomainSetFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ads")
	content := "one.example.com\ntwo.example.com\n#x\nbad\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	set, count, bad, err := loadDomainSetFromFile(path, 8)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || bad != 2 {
		t.Fatalf("count=%d bad=%d", count, bad)
	}
	if set.Len() != 2 {
		t.Fatalf("len=%d", set.Len())
	}
}

func TestDomainSet_HasBinarySearch(t *testing.T) {
	set := DomainSetFromDomains([]string{
		"zeta.example",
		"alpha.example",
		"mu.example",
		"alpha.example", // dup
	})
	if set.Len() != 3 {
		t.Fatalf("len=%d want 3", set.Len())
	}
	for _, d := range []string{"alpha.example", "mu.example", "zeta.example"} {
		if !set.Has(d) {
			t.Fatalf("missing %q", d)
		}
	}
	if set.Has("nope.example") {
		t.Fatal("unexpected hit")
	}
}

func TestDomainCatalog_PerList(t *testing.T) {
	ads := DomainSetFromDomains([]string{"ads.example.com", "tracker.example.com"})
	mal := DomainSetFromDomains([]string{"bad.malware.test"})
	cat := NewCatalog(
		[]string{"Ads", "Malware", "Off"},
		[]*DomainSet{ads, mal, nil},
	)
	if cat.ListCount() != 2 {
		t.Fatalf("lists=%d", cat.ListCount())
	}
	ok, tag := cat.Has("ads.example.com")
	if !ok || tag != "Ads" {
		t.Fatalf("ok=%v tag=%q", ok, tag)
	}
	ok, tag = cat.Has("bad.malware.test")
	if !ok || tag != "Malware" {
		t.Fatalf("ok=%v tag=%q", ok, tag)
	}
	ok, _ = cat.Has("clean.example.com")
	if ok {
		t.Fatal("should not match")
	}

	// disable malware by rebuilding without it
	cat2 := NewCatalog([]string{"Ads"}, []*DomainSet{ads})
	ok, _ = cat2.Has("bad.malware.test")
	if ok {
		t.Fatal("disabled list should not match")
	}
	// reuse snapshot
	snap := cat.Snapshot()
	if snap["Ads"] == nil || snap["Malware"] == nil {
		t.Fatal("snapshot missing lists")
	}
}

func TestDomainCatalog_ParallelHas(t *testing.T) {
	// 4+ lists exercise the parallel path
	tags := make([]string, 5)
	sets := make([]*DomainSet, 5)
	for i := 0; i < 5; i++ {
		tags[i] = "L" + string(rune('A'+i))
		sets[i] = DomainSetFromDomains([]string{
			"shared.example.com",
			"only-" + tags[i] + ".example.com",
		})
	}
	cat := NewCatalog(tags, sets)
	ok, tag := cat.Has("only-LC.example.com")
	if !ok || tag != "LC" {
		t.Fatalf("ok=%v tag=%q want LC", ok, tag)
	}
	ok, _ = cat.Has("missing.example.com")
	if ok {
		t.Fatal("unexpected")
	}
}

func TestMergeDomainSets(t *testing.T) {
	a := DomainSetFromDomains([]string{"a.example.com"})
	b := DomainSetFromDomains([]string{"b.example.com", "a.example.com"})
	m := MergeDomainSets(a, b)
	if m.Len() != 2 {
		t.Fatalf("len=%d", m.Len())
	}
	if !m.Has("a.example.com") || !m.Has("b.example.com") {
		t.Fatal("merge failed")
	}
}

func TestDomainSet_Empty(t *testing.T) {
	var s *DomainSet
	if s.Has("x.com") || s.Len() != 0 {
		t.Fatal("nil set")
	}
	s = DomainSetFromDomains(nil)
	if s.Has("x.com") || s.Len() != 0 {
		t.Fatal("empty set")
	}
	c := EmptyCatalog()
	ok, _ := c.Has("x.com")
	if ok || c.Len() != 0 {
		t.Fatal("empty catalog")
	}
}
