package main

import "testing"

func TestRedactKey(t *testing.T) {
	cases := map[string]string{
		"":                        "…",
		"abc":                     "…",      // shorter than the prefix
		"abcde":                   "…",      // exactly the prefix length → fully masked
		"abcdef":                  "abcde…", // one longer → 5-char prefix
		"ABCDE-12345-67890-FGHIJ": "ABCDE…",
	}
	for in, want := range cases {
		if got := redactKey(in); got != want {
			t.Errorf("redactKey(%q) = %q, want %q", in, got, want)
		}
	}
}
