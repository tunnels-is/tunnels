package argon

import (
	"strings"
	"testing"
)

func TestCompareRoundTrip(t *testing.T) {
	a := NewDefault()
	hash, err := a.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := a.Compare("correct horse battery staple", hash)
	if err != nil || !ok {
		t.Fatalf("valid password should match: ok=%v err=%v", ok, err)
	}
	ok, _ = a.Compare("wrong", hash)
	if ok {
		t.Fatal("wrong password must not match")
	}
}

// A hostile encodedHash with an enormous memory parameter must be rejected
// before it reaches argon2.IDKey, not turned into a multi-GiB allocation.
func TestCompareRejectsOutOfRangeParams(t *testing.T) {
	a := NewDefault()
	good, err := a.Hash("pw")
	if err != nil {
		t.Fatal(err)
	}
	// Splice an absurd memory cost into an otherwise well-formed hash.
	parts := strings.Split(good, "$")
	parts[3] = "m=4294967295,t=3,p=1"
	evil := strings.Join(parts, "$")
	if _, err := a.Compare("pw", evil); err == nil {
		t.Fatal("out-of-range argon parameters must be rejected")
	}
}
