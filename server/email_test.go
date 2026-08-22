package main

import "testing"

func TestNormalizeEmail(t *testing.T) {
	if got := normalizeEmail("  Admin@Example.COM "); got != "admin@example.com" {
		t.Fatalf("got %q", got)
	}
	if normalizeEmail("   ") != "" {
		t.Fatal("blank email should normalize to empty")
	}
}

func TestIsReservedAccountEmail(t *testing.T) {
	if !isReservedAccountEmail("ADMIN") || !isReservedAccountEmail(" admin ") {
		t.Fatal("admin must be reserved")
	}
	if isReservedAccountEmail("admin@example.com") || isReservedAccountEmail("notadmin") {
		t.Fatal("only the exact admin account name is reserved")
	}
}
