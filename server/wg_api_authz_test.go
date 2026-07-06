package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

// setupWGPeerTest creates a fresh DB + a wg-server record and returns it so
// tests can inject it into the request context the way wireGuardServerKeyCheck
// middleware would in production.
func setupWGPeerTest(t *testing.T) *types.Server {
	t.Helper()
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	s := &types.Server{
		ID:     uuid.New(),
		Tag:    "wg-authz-test",
		APIKey: "test-wg-key",
	}
	if err := BBolt_CreateServer(s); err != nil {
		t.Fatal(err)
	}
	return s
}

// seedUserWithDevice persists a user and a device bound to server, returning the
// device's WireGuard pubkey.
func seedUserWithDevice(t *testing.T, server *types.Server, u *User) string {
	t.Helper()
	if err := BBolt_CreateUser(u); err != nil {
		t.Fatal(err)
	}
	wgKey := makeWGKey()
	d := &types.Device{
		ID:           uuid.New(),
		UserID:       u.ID,
		ServerID:     server.ID,
		WireGuardKey: wgKey,
		WireGuardIP:  "10.0.0.5",
	}
	if err := BBolt_CreateDevice(d); err != nil {
		t.Fatal(err)
	}
	return wgKey
}

func callWGPeerAuthz(t *testing.T, server *types.Server, pubKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/wg/peer?pubkey="+url.QueryEscape(pubKey), nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyServer, server))
	w := httptest.NewRecorder()
	API_WGPeer(w, req)
	return w
}

func TestAPI_WGPeer_ActiveUserAuthorized(t *testing.T) {
	server := setupWGPeerTest(t)
	pubKey := seedUserWithDevice(t, server, &User{
		ID:            uuid.New(),
		Email:         "active@example.com",
		SubExpiration: time.Now().Add(24 * time.Hour),
	})
	w := callWGPeerAuthz(t, server, pubKey)
	if w.Code != http.StatusOK {
		t.Fatalf("active user should be authorized, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPI_WGPeer_DisabledUserRejected(t *testing.T) {
	server := setupWGPeerTest(t)
	pubKey := seedUserWithDevice(t, server, &User{
		ID:            uuid.New(),
		Email:         "disabled@example.com",
		Disabled:      true,
		SubExpiration: time.Now().Add(24 * time.Hour),
	})
	w := callWGPeerAuthz(t, server, pubKey)
	if w.Code != http.StatusForbidden {
		t.Fatalf("disabled user should be rejected with 403, got %d", w.Code)
	}
}

func TestAPI_WGPeer_ExpiredSubscriptionRejected(t *testing.T) {
	server := setupWGPeerTest(t)
	pubKey := seedUserWithDevice(t, server, &User{
		ID:            uuid.New(),
		Email:         "expired@example.com",
		SubExpiration: time.Now().Add(-1 * time.Hour),
	})
	w := callWGPeerAuthz(t, server, pubKey)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expired subscription should be rejected with 403, got %d", w.Code)
	}
}

// A zero SubExpiration means the deployment does not use subscriptions; access
// must not be treated as expired.
func TestAPI_WGPeer_ZeroSubExpirationAllowed(t *testing.T) {
	server := setupWGPeerTest(t)
	pubKey := seedUserWithDevice(t, server, &User{
		ID:    uuid.New(),
		Email: "nosub@example.com",
	})
	w := callWGPeerAuthz(t, server, pubKey)
	if w.Code != http.StatusOK {
		t.Fatalf("zero SubExpiration should not expire access, got %d: %s", w.Code, w.Body.String())
	}
}
