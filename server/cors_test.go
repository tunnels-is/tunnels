package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func TestOriginAllowed(t *testing.T) {
	prev := Config.Load()
	defer Config.Store(prev)

	Config.Store(&types.ServerConfig{AllowedOrigins: []string{"https://ui.example.com"}})
	for _, tc := range []struct {
		origin string
		want   bool
	}{
		{"", false},
		{"https://ui.example.com", true},
		{"https://evil.com", false},
	} {
		if got := originAllowed(tc.origin); got != tc.want {
			t.Errorf("originAllowed(%q) = %v, want %v", tc.origin, got, tc.want)
		}
	}

	// A "*" entry allows any concrete origin but never a missing one.
	Config.Store(&types.ServerConfig{AllowedOrigins: []string{"*"}})
	if !originAllowed("https://anything.example") {
		t.Error(`"*" should allow any concrete origin`)
	}
	if originAllowed("") {
		t.Error("empty origin must never be allowed, even with \"*\"")
	}
}

func TestCORSMiddleware(t *testing.T) {
	prev := Config.Load()
	defer Config.Store(prev)
	Config.Store(&types.ServerConfig{AllowedOrigins: []string{"https://ui.example.com"}})

	h := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	call := func(origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}

	if got := call("https://ui.example.com").Header().Get("Access-Control-Allow-Origin"); got != "https://ui.example.com" {
		t.Errorf("allowed origin should be echoed, got %q", got)
	}
	if got := call("https://evil.com").Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin must get no Allow-Origin header, got %q", got)
	}
	if got := call("").Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("no Origin must get no Allow-Origin header, got %q", got)
	}
}
