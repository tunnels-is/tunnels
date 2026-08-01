package main

import "testing"

func TestRedactKey(t *testing.T) {
	cases := map[string]string{
		"":                        "…",
		"abc":                     "…",
		"abcde":                   "…",
		"abcdef":                  "abcde…",
		"ABCDE-12345-67890-FGHIJ": "ABCDE…",
	}
	for in, want := range cases {
		if got := redactKey(in); got != want {
			t.Errorf("redactKey(%q) = %q, want %q", in, got, want)
		}
	}
}
