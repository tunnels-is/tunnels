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
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func seedUserWithDevice(t *testing.T, server *types.Server, u *User) string {
	t.Helper()
	if err := createUser(u); err != nil {
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
	if err := createDevice(d); err != nil {
		t.Fatal(err)
	}
	return wgKey
}

func callWGPeerAuthz(t *testing.T, server *types.Server, pubKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/wg/peer?pubkey="+url.QueryEscape(pubKey), nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyServer, server))
	w := httptest.NewRecorder()
	handleWGPeer(w, req)
	return w
}

func callWGConfig(t *testing.T, user *User, serverID uuid.UUID, pubKey string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet,
		"/client/wg/config?serverID="+serverID.String()+"&pubKey="+url.QueryEscape(pubKey), nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, user))
	w := httptest.NewRecorder()
	handleWGConfig(w, req)
	return w
}

func TestHandleWGConfig_GroupACL(t *testing.T) {
	setupTestDB(t)
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	group := uuid.New()

	groupedServer := &types.Server{ID: uuid.New(), Tag: "grouped", APIKey: uuid.NewString(), Groups: []uuid.UUID{group}}
	publicServer := &types.Server{ID: uuid.New(), Tag: "public", APIKey: uuid.NewString(), Groups: []uuid.UUID{}}
	if err := createServer(groupedServer); err != nil {
		t.Fatal(err)
	}
	if err := createServer(publicServer); err != nil {
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

func TestHandleWGConfig_NoDeviceReturnsEmptyIP(t *testing.T) {
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
	if err := createServer(server); err != nil {
		t.Fatal(err)
	}

	user := &User{ID: uuid.New()}

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

func TestHandleWGPeer_ActiveUserAuthorized(t *testing.T) {
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

func TestPunchPeersFor_SkipsInactiveOwners(t *testing.T) {
	setupTestDB(t)
	mesh := uuid.NewString()
	open := &types.Server{ID: uuid.New(), Tag: "open", APIKey: uuid.NewString(), MeshGroupID: mesh}
	group := uuid.New()
	locked := &types.Server{ID: uuid.New(), Tag: "locked", APIKey: uuid.NewString(), Groups: []uuid.UUID{group}, MeshGroupID: mesh}
	for _, s := range []*types.Server{open, locked} {
		if err := createServer(s); err != nil {
			t.Fatal(err)
		}
	}
	active := &User{ID: uuid.New(), Email: "active-p2p@test.local", SubExpiration: time.Now().Add(time.Hour)}
	disabled := &User{ID: uuid.New(), Email: "disabled-p2p@test.local", Disabled: true, SubExpiration: time.Now().Add(time.Hour)}
	expired := &User{ID: uuid.New(), Email: "expired-p2p@test.local", SubExpiration: time.Now().Add(-time.Hour)}
	outsider := &User{ID: uuid.New(), Email: "outsider-p2p@test.local", SubExpiration: time.Now().Add(time.Hour)}
	member := &User{ID: uuid.New(), Email: "member-p2p@test.local", SubExpiration: time.Now().Add(time.Hour), Groups: []uuid.UUID{group}}
	selfUser := &User{ID: uuid.New(), Email: "self-p2p@test.local", SubExpiration: time.Now().Add(time.Hour)}
	for _, u := range []*User{active, disabled, expired, outsider, member, selfUser} {
		if err := createUser(u); err != nil {
			t.Fatal(err)
		}
	}
	add := func(owner *User, server *types.Server, ip, tag string) *types.Device {
		t.Helper()
		d := &types.Device{
			ID: uuid.New(), UserID: owner.ID, ServerID: server.ID,
			WireGuardIP: ip, WireGuardKey: tag + "-wg", DiscoKey: tag + "-disco", Tag: tag,
		}
		if err := createDevice(d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	self := add(selfUser, open, "10.8.0.2", "self")
	okDev := add(active, open, "10.8.0.3", "ok")
	add(disabled, open, "10.8.0.4", "disabled")
	add(expired, open, "10.8.0.5", "expired")
	add(outsider, locked, "10.8.0.6", "outsider")
	memberDev := add(member, locked, "10.8.0.7", "member")

	peers, err := punchPeersFor(self, open)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range peers {
		got[p.DeviceID] = true
	}
	if !got[okDev.ID.String()] || !got[memberDev.ID.String()] || len(peers) != 2 {
		t.Fatalf("peers %+v", peers)
	}

	req := httptest.NewRequest(http.MethodGet, "/wg/p2p/local", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyServer, open))
	w := httptest.NewRecorder()
	handleWGP2PLocal(w, req)
	if w.Code != 200 {
		t.Fatalf("local %d %s", w.Code, w.Body.String())
	}
	var local struct {
		Peers []types.P2PPeer
	}
	if err := json.Unmarshal(w.Body.Bytes(), &local); err != nil {
		t.Fatal(err)
	}
	localIDs := map[string]bool{}
	for _, p := range local.Peers {
		localIDs[p.DeviceID] = true
	}
	if !localIDs[self.ID.String()] || !localIDs[okDev.ID.String()] || len(local.Peers) != 2 {
		t.Fatalf("local peers %+v", local.Peers)
	}

	expiredDev := add(expired, open, "10.8.0.9", "expired-self")
	expReq := httptest.NewRequest(http.MethodGet, "/client/p2p/peers?deviceID="+expiredDev.ID.String(), nil)
	expReq = expReq.WithContext(context.WithValue(expReq.Context(), contextKeyUser, expired))
	expW := httptest.NewRecorder()
	handleClientP2PPeers(expW, expReq)
	if expW.Code != http.StatusForbidden {
		t.Fatalf("expired caller: %d %s", expW.Code, expW.Body.String())
	}

	disabledDev := add(disabled, open, "10.8.0.8", "disabled-locate")
	locReq := httptest.NewRequest(http.MethodGet, "/wg/p2p/locate?deviceID="+disabledDev.ID.String(), nil)
	locReq = locReq.WithContext(context.WithValue(locReq.Context(), contextKeyServer, open))
	locW := httptest.NewRecorder()
	handleWGP2PLocate(locW, locReq)
	if locW.Code != http.StatusNotFound {
		t.Fatalf("locate disabled: %d %s", locW.Code, locW.Body.String())
	}
}

func TestHandleWGPeer_DisabledUserRejected(t *testing.T) {
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

func TestHandleWGPeer_ExpiredSubscriptionRejected(t *testing.T) {
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

func TestHandleWGPeer_ZeroSubExpirationAllowed(t *testing.T) {
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
