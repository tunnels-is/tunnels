package main

import (
	"context"
	"encoding/json"
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

func callWGConfig(t *testing.T, user *User, serverID uuid.UUID, pubKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet,
		"/client/wg/config?serverID="+serverID.String()+"&pubKey="+url.QueryEscape(pubKey), nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, user))
	w := httptest.NewRecorder()
	API_WGConfig(w, req)
	return w
}

func TestAPI_WGConfig_GroupACL(t *testing.T) {
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	group := uuid.New()

	groupedServer := &types.Server{ID: uuid.New(), Tag: "grouped", APIKey: uuid.NewString(), Groups: []uuid.UUID{group}}
	publicServer := &types.Server{ID: uuid.New(), Tag: "public", APIKey: uuid.NewString(), Groups: []uuid.UUID{}}
	if err := BBolt_CreateServer(groupedServer); err != nil {
		t.Fatal(err)
	}
	if err := BBolt_CreateServer(publicServer); err != nil {
		t.Fatal(err)
	}

	member := &User{ID: uuid.New(), Groups: []uuid.UUID{group}}
	outsider := &User{ID: uuid.New(), Groups: []uuid.UUID{}}
	key := makeWGKey()

	if w := callWGConfig(t, member, groupedServer.ID, key); w.Code != http.StatusOK {
		t.Fatalf("group member should read grouped server config (200), got %d: %s", w.Code, w.Body.String())
	}
	if w := callWGConfig(t, outsider, groupedServer.ID, key); w.Code != http.StatusUnauthorized {
		t.Fatalf("non-member should be denied grouped server config (401), got %d", w.Code)
	}
	if w := callWGConfig(t, outsider, publicServer.ID, key); w.Code != http.StatusOK {
		t.Fatalf("ungrouped server config should be readable by anyone (200), got %d: %s", w.Code, w.Body.String())
	}
}

func TestAPI_WGConfig_NoDeviceReturnsEmptyIP(t *testing.T) {
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	server := &types.Server{
		ID:              uuid.New(),
		Tag:             "public",
		APIKey:          uuid.NewString(),
		IP:              "1.2.3.4",
		WireGuardPort:   51820,
		WireGuardPubKey: makeWGKey(),
		WireGuardSubnet: "10.7.0.0/24",
	}
	if err := BBolt_CreateServer(server); err != nil {
		t.Fatal(err)
	}
	// User only needs to be in the request context (same as GroupACL tests).
	user := &User{ID: uuid.New()}

	// Unknown pubkey: still 200 so the client can auto-create a device.
	w := callWGConfig(t, user, server.ID, makeWGKey())
	if w.Code != http.StatusOK {
		t.Fatalf("missing device should return 200 with empty WireGuardIP, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, w.Body.String())
	}
	if ip, _ := body["WireGuardIP"].(string); ip != "" {
		t.Fatalf("expected empty WireGuardIP for unknown device, got %q", ip)
	}
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
