package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tunnels-is/tunnels/types"
)

func TestAdminUI_APIKeyDoesNotSkipSession(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	apiKey := strings.Repeat("k", 16)
	Config.Store(&types.ServerConfig{
		CookieSigningKey: strings.Repeat("c", minSecretLen),
		TwoFactorKey:     strings.Repeat("c", minSecretLen),
		AdminAPIKey:      apiKey,
	})

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	// The API-key check runs first. The admin UI check still requires a session.
	stacked := applyMiddleware(inner, xAdminAPIKeyMiddleware, adminUIMiddleware)
	req := httptest.NewRequest(http.MethodPost, "/ui/servers", nil)
	req.Header.Set("X-API-KEY", apiKey)
	w := httptest.NewRecorder()
	stacked.ServeHTTP(w, req)
	if called || w.Code != http.StatusUnauthorized {
		t.Fatalf("api key skipped the session: called=%v %d %s", called, w.Code, w.Body.String())
	}

	called = false
	onlyKey := applyMiddleware(inner, xAdminAPIKeyMiddleware)
	req = httptest.NewRequest(http.MethodPost, "/ui/device/create", nil)
	req.Header.Set("X-API-KEY", apiKey)
	w = httptest.NewRecorder()
	onlyKey.ServeHTTP(w, req)
	if !called || w.Code != http.StatusOK {
		t.Fatalf("device create key: called=%v %d %s", called, w.Code, w.Body.String())
	}
}
