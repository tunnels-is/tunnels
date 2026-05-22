package wgserver

import (
	"slices"
	"testing"
)

func TestMasqueradeArgs_MASQUERADEWhenNoPublicIP(t *testing.T) {
	got := masqueradeArgs("-A", "10.0.0.0/24", "eth0", "")
	want := []string{
		"-t", "nat", "-A", "POSTROUTING",
		"-s", "10.0.0.0/24", "-o", "eth0",
		"-j", "MASQUERADE",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMasqueradeArgs_SNATWhenPublicIPSet(t *testing.T) {
	got := masqueradeArgs("-A", "10.0.0.0/24", "eth0", "63.143.33.106")
	want := []string{
		"-t", "nat", "-A", "POSTROUTING",
		"-s", "10.0.0.0/24", "-o", "eth0",
		"-j", "SNAT", "--to-source", "63.143.33.106",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMasqueradeArgs_DrainShape(t *testing.T) {
	// Drain (-D) must match install (-A) exactly except for the action.
	install := masqueradeArgs("-A", "10.0.0.0/24", "eth0", "63.143.33.106")
	drain := masqueradeArgs("-D", "10.0.0.0/24", "eth0", "63.143.33.106")

	if len(install) != len(drain) {
		t.Fatalf("install/drain length mismatch")
	}
	for i := range install {
		if install[i] == "-A" {
			if drain[i] != "-D" {
				t.Fatalf("expected -D at index %d, got %q", i, drain[i])
			}
			continue
		}
		if install[i] != drain[i] {
			t.Fatalf("mismatch at %d: install=%q drain=%q", i, install[i], drain[i])
		}
	}
}
