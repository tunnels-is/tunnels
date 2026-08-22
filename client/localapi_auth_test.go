package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorizeLocalAPI_RequiresSessionCookie(t *testing.T) {
	prevDev := DevMode
	prevTok := sessionToken
	t.Cleanup(func() {
		DevMode = prevDev
		sessionToken = prevTok
	})
	DevMode = false
	sessionToken = "secret-session"

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.Host = "127.0.0.1:7777"
	w := httptest.NewRecorder()
	if authorizeLocalAPI(w, req) {
		t.Fatal("missing cookie must be rejected")
	}
	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req2.Host = "127.0.0.1:7777"
	req2.AddCookie(&http.Cookie{Name: "X-Session-Token", Value: "secret-session"})
	w2 := httptest.NewRecorder()
	if !authorizeLocalAPI(w2, req2) {
		t.Fatal("valid session cookie must be accepted")
	}
}

func TestLocalAPIAuth_WrapsHandler(t *testing.T) {
	prevDev := DevMode
	prevTok := sessionToken
	t.Cleanup(func() {
		DevMode = prevDev
		sessionToken = prevTok
	})
	DevMode = false
	sessionToken = "tok"

	called := false
	h := localAPIAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(204)
	})

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.Host = "127.0.0.1"
	w := httptest.NewRecorder()
	h(w, req)
	if called {
		t.Fatal("unauthenticated pprof handler must not run")
	}
}
