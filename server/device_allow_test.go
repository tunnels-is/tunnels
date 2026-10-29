package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func canonWGKey(fill byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, 32))
}

func nonCanonicalWGKey(fill byte) string {
	canon := canonWGKey(fill)
	return canon[:8] + "\n" + canon[8:]
}

func TestCanonicalWireGuardPublicKey(t *testing.T) {
	canon := canonWGKey(0x11)
	got, err := canonicalWireGuardPublicKey(canon)
	if err != nil || got != canon {
		t.Fatalf("canonical: %q %v", got, err)
	}
	if _, err := canonicalWireGuardPublicKey("  " + canon + "\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalWireGuardPublicKey(nonCanonicalWGKey(0x11)); err == nil {
		t.Fatal("newline spelling must be rejected")
	}
	if _, err := canonicalWireGuardPublicKey("not-a-key"); err == nil {
		t.Fatal("short key must be rejected")
	}
	if _, err := canonicalWireGuardPublicKey(""); err == nil {
		t.Fatal("empty key must be rejected")
	}
}

func TestClientDeviceCreate_IgnoresCallerIdentityAndPunchPolicy(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()

	owner := &User{ID: uuid.New(), Email: "owner@test.local"}
	other := &User{ID: uuid.New(), Email: "other@test.local"}
	if err := createUser(owner); err != nil {
		t.Fatal(err)
	}
	if err := createUser(other); err != nil {
		t.Fatal(err)
	}
	srv := &types.Server{
		ID: uuid.New(), Tag: "a", APIKey: uuid.NewString(),
		WireGuardSubnet: "10.4.0.0/24",
	}
	if err := createServer(srv); err != nil {
		t.Fatal(err)
	}

	suppliedID := uuid.New()
	body, _ := json.Marshal(createDeviceRequest{Device: &types.Device{
		ID:           suppliedID,
		CreatedAt:    time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		Tag:          " laptop ",
		UserID:       other.ID,
		ServerID:     srv.ID,
		WireGuardKey: canonWGKey(0x21),
		WireGuardIP:  "10.9.9.9",
		DiscoKey:     canonWGKey(0x22),
		P2PAllowAll:  true,
		P2PAllowIPs:  []string{"10.1.0.5"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/client/device/create", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, owner))
	w := httptest.NewRecorder()
	handleClientDeviceCreate(w, req)
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}

	found, err := getDevicesByUserID(owner.ID)
	if err != nil || len(found) != 1 {
		t.Fatalf("owner devices: %v %d", err, len(found))
	}
	d := found[0]
	if d.ID == suppliedID || d.UserID != owner.ID {
		t.Fatalf("identity taken from body: %+v", d)
	}
	if d.Tag != "laptop" || d.ServerID != srv.ID || d.WireGuardKey != canonWGKey(0x21) {
		t.Fatalf("create fields: %+v", d)
	}
	if d.WireGuardIP == "10.9.9.9" || d.WireGuardIP == "" {
		t.Fatalf("address: %q", d.WireGuardIP)
	}
	if d.DiscoKey != "" || d.P2PAllowAll || len(d.P2PAllowIPs) != 0 {
		t.Fatalf("punch policy stored: %+v", d)
	}
	if !d.CreatedAt.After(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("created at taken from body: %s", d.CreatedAt)
	}
	if others, _ := getDevicesByUserID(other.ID); len(others) != 0 {
		t.Fatal("device assigned to the body user")
	}
}

func TestClientDeviceCreate_RejectsNonCanonicalKey(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()

	owner := &User{ID: uuid.New(), Email: "own2@test.local"}
	if err := createUser(owner); err != nil {
		t.Fatal(err)
	}
	raw := bytes.Repeat([]byte{0xab}, 32)
	serverKey := base64.StdEncoding.EncodeToString(raw)
	srv := &types.Server{
		ID: uuid.New(), Tag: "a", APIKey: uuid.NewString(),
		WireGuardSubnet: "10.5.0.0/24", WireGuardPubKey: serverKey,
	}
	if err := createServer(srv); err != nil {
		t.Fatal(err)
	}

	post := func(key string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(createDeviceRequest{Device: &types.Device{
			Tag: "d", ServerID: srv.ID, WireGuardKey: key,
		}})
		req := httptest.NewRequest(http.MethodPost, "/client/device/create", bytes.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, owner))
		w := httptest.NewRecorder()
		handleClientDeviceCreate(w, req)
		return w
	}

	if w := post(nonCanonicalWGKey(0xcd)); w.Code != 400 || !strings.Contains(w.Body.String(), "invalid WireGuard key") {
		t.Fatalf("newline key: %d %s", w.Code, w.Body.String())
	}
	alias := serverKey[:8] + "\n" + serverKey[8:]
	if w := post(alias); w.Code != 400 {
		t.Fatalf("server key alias: %d %s", w.Code, w.Body.String())
	}
	if w := post(serverKey); w.Code != 400 {
		t.Fatalf("server key: %d %s", w.Code, w.Body.String())
	}
	if devices, _ := getDevicesByUserID(owner.ID); len(devices) != 0 {
		t.Fatalf("rejected create stored a device: %+v", devices[0])
	}
}

func TestAdminDeviceCreate_DropsPunchPolicyAndCallerAddress(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()

	owner := &User{ID: uuid.New(), Email: "adm@test.local"}
	if err := createUser(owner); err != nil {
		t.Fatal(err)
	}
	srv := &types.Server{
		ID: uuid.New(), Tag: "a", APIKey: uuid.NewString(),
		WireGuardSubnet: "10.6.0.0/24",
	}
	if err := createServer(srv); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(createDeviceRequest{Device: &types.Device{
		Tag: "desk", UserID: owner.ID, ServerID: srv.ID,
		WireGuardKey: canonWGKey(0x31),
		WireGuardIP:  "10.6.0.50",
		DiscoKey:     canonWGKey(0x32),
		P2PAllowAll:  true,
		P2PAllowIPs:  []string{"10.6.0.8"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/ui/device/create", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleAdminDeviceCreate(w, req)
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	found, _ := getDevicesByUserID(owner.ID)
	if len(found) != 1 {
		t.Fatalf("devices: %d", len(found))
	}
	d := found[0]
	if d.WireGuardIP == "10.6.0.50" || d.WireGuardIP == "" || d.DiscoKey != "" || d.P2PAllowAll || len(d.P2PAllowIPs) != 0 {
		t.Fatalf("stored caller fields: %+v", d)
	}
}

func TestAdminDeviceUpdate_KeepsIdentityAddressAndPunchPolicy(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()

	owner := &User{ID: uuid.New(), Email: "keep@test.local"}
	intruder := &User{ID: uuid.New(), Email: "take@test.local"}
	if err := createUser(owner); err != nil {
		t.Fatal(err)
	}
	if err := createUser(intruder); err != nil {
		t.Fatal(err)
	}
	home := &types.Server{
		ID: uuid.New(), Tag: "home", APIKey: uuid.NewString(),
		WireGuardSubnet: "10.7.0.0/24",
	}
	away := &types.Server{
		ID: uuid.New(), Tag: "away", APIKey: uuid.NewString(),
		WireGuardSubnet: "10.8.0.0/24",
	}
	if err := createServer(home); err != nil {
		t.Fatal(err)
	}
	if err := createServer(away); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	d := &types.Device{
		ID: uuid.New(), CreatedAt: created, Tag: "old",
		UserID: owner.ID, ServerID: home.ID,
		WireGuardKey: canonWGKey(0x41), WireGuardIP: "10.7.0.2",
		DiscoKey:    canonWGKey(0x42),
		P2PAllowAll: false, P2PAllowIPs: []string{"10.7.0.9"},
	}
	if err := createDevice(d); err != nil {
		t.Fatal(err)
	}

	post := func(body *types.Device) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(updateDeviceRequest{Device: body})
		req := httptest.NewRequest(http.MethodPost, "/ui/device/update", bytes.NewReader(raw))
		w := httptest.NewRecorder()
		handleAdminDeviceUpdate(w, req)
		return w
	}

	w := post(&types.Device{
		ID: d.ID, CreatedAt: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
		Tag: "renamed", UserID: intruder.ID, ServerID: home.ID,
		WireGuardKey: canonWGKey(0x43), WireGuardIP: "10.7.0.40",
		DiscoKey:    canonWGKey(0x44),
		P2PAllowAll: true, P2PAllowIPs: []string{"1.2.3.4"},
	})
	if w.Code != 200 {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	got, _ := findDeviceByID(d.ID)
	if got.Tag != "renamed" || got.WireGuardKey != canonWGKey(0x43) {
		t.Fatalf("mutable fields: %+v", got)
	}
	if got.UserID != owner.ID || !got.CreatedAt.Equal(created) || got.ServerID != home.ID || got.WireGuardIP != "10.7.0.2" {
		t.Fatalf("identity or address changed: %+v", got)
	}
	if got.DiscoKey != d.DiscoKey || got.P2PAllowAll || len(got.P2PAllowIPs) != 1 || got.P2PAllowIPs[0] != "10.7.0.9" {
		t.Fatalf("punch policy changed: %+v", got)
	}
	if moved, _ := getDevicesByUserID(intruder.ID); len(moved) != 0 {
		t.Fatal("device moved to the body user")
	}

	w = post(&types.Device{ID: d.ID, Tag: "nope", WireGuardKey: nonCanonicalWGKey(0x45)})
	if w.Code != 400 {
		t.Fatalf("non-canonical update: %d %s", w.Code, w.Body.String())
	}
	got, _ = findDeviceByID(d.ID)
	if got.Tag != "renamed" || got.WireGuardKey != canonWGKey(0x43) {
		t.Fatalf("rejected update was stored: %+v", got)
	}

	w = post(&types.Device{
		ID: d.ID, UserID: intruder.ID, ServerID: away.ID, WireGuardIP: "10.8.0.50",
	})
	if w.Code != 200 {
		t.Fatalf("move: %d %s", w.Code, w.Body.String())
	}
	got, _ = findDeviceByID(d.ID)
	if got.UserID != owner.ID || got.ServerID != away.ID || got.WireGuardIP == "10.8.0.50" || got.WireGuardIP == "10.7.0.2" || got.WireGuardIP == "" {
		t.Fatalf("server move: %+v", got)
	}
	if got.DiscoKey != d.DiscoKey || len(got.P2PAllowIPs) != 1 {
		t.Fatalf("move cleared punch policy: %+v", got)
	}
}
