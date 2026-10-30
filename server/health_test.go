package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealth_EmptyOK ...
func TestHealth_EmptyOK(t *testing.T) {
	for _, path := range []string{"/health", "/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		healthCheckHandler(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, w.Code)
		}
		if w.Body.Len() != 0 {
			t.Fatalf("%s body: %q", path, w.Body.String())
		}
	}
}
