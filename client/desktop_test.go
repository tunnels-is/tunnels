package client

import "testing"

func TestCloneConfigDoesNotAliasBool(t *testing.T) {
	orig := CONFIG.Load()
	if orig == nil {
		t.Fatal("config not initialized")
	}
	before := orig.InfoLogging
	cfg := CloneConfig()
	cfg.InfoLogging = !before
	if CONFIG.Load().InfoLogging != before {
		t.Fatal("CloneConfig mutated live config")
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{Messages: []string{"a", "b"}}
	if err.Error() != "a; b" {
		t.Fatalf("got %q", err.Error())
	}
}

func TestControllerError(t *testing.T) {
	if got := ControllerError([]byte(`{"Error":"nope"}`), "fb"); got != "nope" {
		t.Fatalf("got %q", got)
	}
	if got := ControllerError(nil, "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}
