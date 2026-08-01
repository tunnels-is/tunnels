package argon

import (
	"bytes"
	"testing"
)

func TestKey_DeterministicWhenSkipSalt(t *testing.T) {
	a := &Argon{
		Memory:      20 * 1024,
		Iterations:  3,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	}
	k1, err := a.Key("user-id", true)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := a.Key("user-id", true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("Key with skipSalt should be deterministic")
	}
	k3, err := a.Key("other-user", true)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(k1, k3) {
		t.Fatal("different inputs should produce different keys")
	}
}

func TestGenerateUserFolderHash(t *testing.T) {
	k1, err := GenerateUserFolderHash("abc")
	if err != nil {
		t.Fatal(err)
	}
	k2, err := GenerateUserFolderHash("abc")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("GenerateUserFolderHash should be deterministic")
	}
	if len(k1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(k1))
	}
}
