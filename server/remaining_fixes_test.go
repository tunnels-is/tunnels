package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	"golang.org/x/crypto/bcrypt"
)

func TestLoginLockout_AfterFiveFailures(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	resetPasswordResetAttemptsForTest()

	hash, err := bcrypt.GenerateFromPassword([]byte("longenough1"), 13)
	if err != nil {
		t.Fatal(err)
	}
	u := &User{ID: uuid.New(), Email: "lockme@test.local", Password: string(hash)}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < passwordResetMaxTries; i++ {
		body, _ := json.Marshal(loginRequest{Email: u.Email, Password: "wrongpassword1"})
		req := httptest.NewRequest(http.MethodPost, "/client/user/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handleClientLogin(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d %s", i+1, w.Code, w.Body.String())
		}
	}
	body, _ := json.Marshal(loginRequest{Email: u.Email, Password: "longenough1"})
	req := httptest.NewRequest(http.MethodPost, "/client/user/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleClientLogin(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("locked login: %d %s, want 401", w.Code, w.Body.String())
	}
}

func TestRegister_GenericErrorOnDuplicateAndReserved(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{})

	w := postRegisterRec(t, "dup@example.com", "longenough1")
	if w.Code != http.StatusOK {
		t.Fatalf("first register: %d %s", w.Code, w.Body.String())
	}
	w = postRegisterRec(t, "dup@example.com", "longenough1")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate: %d", w.Code)
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "already") {
		t.Fatalf("duplicate error leaked occupancy: %s", w.Body.String())
	}

	w = postRegisterRec(t, "admin", "longenough1")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("reserved admin: %d %s", w.Code, w.Body.String())
	}
	w = postRegisterRec(t, "  ", "longenough1")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty email: %d", w.Code)
	}
}

func TestRegister_NormalizesEmail(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{})

	if postRegister(t, "Case.User@Example.COM", "longenough1") != http.StatusOK {
		t.Fatal("mixed-case register")
	}
	got, err := findUserByEmail("case.user@example.com")
	if err != nil || got == nil {
		t.Fatalf("normalized lookup failed: %v %#v", err, got)
	}
}

func TestClientAuthMiddleware_GenericUnauthorized(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()

	h := clientAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	req := httptest.NewRequest(http.MethodGet, "/client/x", nil)
	req.Header.Set("X-Email", "nobody@example.com")
	req.Header.Set("X-Device-Token", "dummy")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "not found") {
		t.Fatalf("enumerated unknown email: %s", w.Body.String())
	}
}

func TestAdminCookie_ExpiresAndGenericDecryptError(t *testing.T) {
	setupTestDB(t)
	okKey := strings.Repeat("c", minSecretLen)
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: okKey})

	uid := uuid.New()
	tok, err := encryptAdminCookie(uid.String(), "devicetoken", "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}
	gotUID, gotDT, err := decryptAdminCookie(tok, "192.0.2.1")
	if err != nil || gotUID != uid || gotDT != "devicetoken" {
		t.Fatalf("round trip: uid=%s dt=%s err=%v", gotUID, gotDT, err)
	}
	_, _, err = decryptAdminCookie(tok, "10.0.0.1")
	if err == nil || err.Error() != "invalid session" {
		t.Fatalf("IP mismatch must be generic, got %v", err)
	}

	expired, err := json.Marshal(adminCookiePayload{
		UID: uid.String(), DT: "devicetoken", IP: "192.0.2.1", Exp: time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cookieCipher()
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nonce, nonce, expired, nil)
	_, _, err = decryptAdminCookie(base64.RawURLEncoding.EncodeToString(ct), "192.0.2.1")
	if err == nil {
		t.Fatal("expired cookie must fail")
	}
}

func TestAdminLogout_RevokesCookieToken(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	okKey := strings.Repeat("c", minSecretLen)
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: okKey})

	keep := &DeviceToken{DT: uuid.NewString(), N: "other"}
	sess := &DeviceToken{DT: uuid.NewString(), N: "session"}
	u := &User{
		ID: uuid.New(), Email: "admin-out@test.local", IsAdmin: true,
		Tokens: []*DeviceToken{keep, sess},
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/ui/user/logout", bytes.NewReader([]byte("{}")))
	ctx := context.WithValue(req.Context(), contextKeyUser, u)
	ctx = context.WithValue(ctx, contextKeyDeviceToken, sess.DT)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handleAdminLogout(w, req)
	if w.Code != 200 {
		t.Fatalf("logout %d", w.Code)
	}
	got, err := findUserByID(u.ID)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if len(got.Tokens) != 1 || got.Tokens[0].DT != keep.DT {
		t.Fatalf("tokens after logout: %+v", got.Tokens)
	}
}

func TestInitializeAdminUser_FailsIfNonAdminExists(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	u := &User{ID: uuid.New(), Email: "admin", IsAdmin: false}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	if err := initializeAdminUser(); err == nil {
		t.Fatal("expected error when admin exists without IsAdmin")
	}
}

func TestAssignNextWireGuardIP_SkipsBroadcast(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{
		ID: uuid.New(), Tag: "small", APIKey: uuid.NewString(),
		WireGuardSubnet: "10.9.9.0/30",
	}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}
	ip, err := assignNextWireGuardIP(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "10.9.9.2" {
		t.Fatalf("first usable = %s, want 10.9.9.2", ip)
	}
	d := &types.Device{ID: uuid.New(), ServerID: s.ID, WireGuardIP: ip, Tag: "a"}
	if err := createDevice(d); err != nil {
		t.Fatal(err)
	}
	if _, err := assignNextWireGuardIP(s.ID); err == nil {
		t.Fatal("subnet must be exhausted before assigning broadcast 10.9.9.3")
	}
}

func TestUpdateDevice_RejectsDuplicateAndReservedIP(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{
		ID: uuid.New(), Tag: "dup", APIKey: uuid.NewString(),
		WireGuardSubnet: "10.0.0.0/24",
	}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}
	a := &types.Device{ID: uuid.New(), ServerID: s.ID, WireGuardIP: "10.0.0.10", Tag: "a"}
	b := &types.Device{ID: uuid.New(), ServerID: s.ID, WireGuardIP: "10.0.0.11", Tag: "b"}
	if err := createDevice(a); err != nil {
		t.Fatal(err)
	}
	if err := createDevice(b); err != nil {
		t.Fatal(err)
	}
	b.WireGuardIP = "10.0.0.10"
	if err := updateDevice(b); !errors.Is(err, errDeviceIPInUse) {
		t.Fatalf("duplicate IP: %v", err)
	}
	b.WireGuardIP = "10.0.0.1"
	if err := updateDevice(b); !errors.Is(err, errDeviceIPReserved) {
		t.Fatalf("reserved IP: %v", err)
	}
}

func TestRejectServerWireGuardKey_EmptyRejected(t *testing.T) {
	if err := rejectServerWireGuardKey(""); err == nil {
		t.Fatal("empty WireGuard key must be rejected")
	}
	if err := rejectServerWireGuardKey("   "); err == nil {
		t.Fatal("whitespace WireGuard key must be rejected")
	}
}

func TestRejectServerWireGuardKey(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{
		ID: uuid.New(), Tag: "k", APIKey: uuid.NewString(),
		WireGuardPubKey: "server-pub-key",
	}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}
	if err := rejectServerWireGuardKey("server-pub-key"); err == nil {
		t.Fatal("server pubkey must be rejected")
	}
	if err := rejectServerWireGuardKey("other"); err != nil {
		t.Fatal(err)
	}
}

func TestWGConfig_HidesForeignKeyAndExpiredSub(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	s := &types.Server{ID: uuid.New(), Tag: "c", APIKey: uuid.NewString(), WireGuardSubnet: "10.0.0.0/24"}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}
	owner := &User{ID: uuid.New(), Email: "own@test.local", SubExpiration: time.Now().Add(time.Hour)}
	other := &User{ID: uuid.New(), Email: "oth@test.local", SubExpiration: time.Now().Add(time.Hour)}
	if err := createUser(owner); err != nil {
		t.Fatal(err)
	}
	if err := createUser(other); err != nil {
		t.Fatal(err)
	}
	d := &types.Device{
		ID: uuid.New(), UserID: owner.ID, ServerID: s.ID,
		WireGuardKey: "foreign-key", WireGuardIP: "10.0.0.9", Tag: "d",
	}
	if err := createDevice(d); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/client/wg/config?serverID="+s.ID.String()+"&pubKey=foreign-key", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, other))
	w := httptest.NewRecorder()
	handleWGConfig(w, req)
	if w.Code != 200 {
		t.Fatalf("occupancy: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "10.0.0.9") {
		t.Fatalf("leaked foreign IP: %s", w.Body.String())
	}

	other.SubExpiration = time.Now().Add(-time.Hour)
	req2 := httptest.NewRequest(http.MethodGet, "/client/wg/config?serverID="+s.ID.String()+"&pubKey=x", nil)
	req2 = req2.WithContext(context.WithValue(req2.Context(), contextKeyUser, other))
	w2 := httptest.NewRecorder()
	handleWGConfig(w2, req2)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expired sub: %d %s", w2.Code, w2.Body.String())
	}
}

func TestNextSubExpirationFromLemon_DoesNotStack(t *testing.T) {
	created := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	u := &User{
		SubExpiration: created.AddDate(0, 12, 0),
		Key:           &LicenseKey{Created: created, Months: 12, Key: "k"},
	}
	exp, update := nextSubExpirationFromLemon(u, "active", nil)
	if update {
		t.Fatalf("must not restack an already-granted term, got %v", exp)
	}
	expiredAt := created.AddDate(0, 12, 0)
	got, ok := nextSubExpirationFromLemon(u, "active", &expiredAt)
	if !ok || !got.Equal(expiredAt) {
		t.Fatalf("expires_at not honored: ok=%v got=%v", ok, got)
	}
	if _, ok := nextSubExpirationFromLemon(u, "expired", nil); ok {
		t.Fatal("expired status must not renew")
	}
}

func TestUserLogout_RevokesCurrentDeviceToken(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	keep := &DeviceToken{DT: uuid.NewString(), N: "keep"}
	sess := &DeviceToken{DT: uuid.NewString(), N: "sess"}
	u := &User{ID: uuid.New(), Email: "out@test.local", Tokens: []*DeviceToken{keep, sess}}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(logoutRequest{})
	req := httptest.NewRequest(http.MethodPost, "/client/user/logout", bytes.NewReader(body))
	ctx := context.WithValue(req.Context(), contextKeyUser, u)
	ctx = context.WithValue(ctx, contextKeyDeviceToken, sess.DT)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handleClientLogout(w, req)
	if w.Code != 200 {
		t.Fatalf("logout %d %s", w.Code, w.Body.String())
	}
	got, _ := findUserByID(u.ID)
	if len(got.Tokens) != 1 || got.Tokens[0].DT != keep.DT {
		t.Fatalf("tokens %+v", got.Tokens)
	}
}

func TestAdminUpdate_NormalizesEmail(t *testing.T) {
	setupTestDB(t)
	u := &User{ID: uuid.New(), Email: "jane@old.com"}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	if err := updateUserAdmin(&adminUserUpdateRequest{
		TargetUserID: u.ID,
		Email:        "Jane@Old.COM",
		Disabled:     true,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := findUserByID(u.ID)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if got.Email != "jane@old.com" {
		t.Fatalf("stored Email=%q", got.Email)
	}
	if !got.Disabled {
		t.Fatal("Disabled not set")
	}
}

func TestCreateUser_RejectsDuplicateNormalizedEmail(t *testing.T) {
	setupTestDB(t)
	a := &User{ID: uuid.New(), Email: "Foo@Example.COM"}
	if err := createUser(a); err != nil {
		t.Fatal(err)
	}
	b := &User{ID: uuid.New(), Email: "foo@example.com"}
	if err := createUser(b); err == nil {
		t.Fatal("second create with same email different case must fail")
	}
}

func TestCreateUser_StoresNormalizedEmail(t *testing.T) {
	setupTestDB(t)
	u := &User{ID: uuid.New(), Email: "  Mixed.Case@Example.COM "}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	if u.Email != "mixed.case@example.com" {
		t.Fatalf("in-memory Email=%q", u.Email)
	}
	got, err := findUserByID(u.ID)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if got.Email != "mixed.case@example.com" {
		t.Fatalf("stored Email=%q", got.Email)
	}
	got, err = findUserByEmail("MIXED.CASE@EXAMPLE.COM")
	if err != nil || got == nil {
		t.Fatalf("normalized lookup: %v %#v", err, got)
	}
}

func TestEmailIndex_NormalizedLookupAndSubTime(t *testing.T) {
	setupTestDB(t)
	u := &User{ID: uuid.New(), Email: "Mixed.Case@Example.COM"}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	got, err := findUserByEmail("mixed.case@example.com")
	if err != nil || got == nil {
		t.Fatalf("lowercase lookup: %v %#v", err, got)
	}
	got, err = findUserByEmail("MIXED.CASE@EXAMPLE.COM")
	if err != nil || got == nil {
		t.Fatalf("upper lookup: %v %#v", err, got)
	}

	exp := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	if err := updateUserSubTime(&User{Email: "Mixed.Case@Example.COM", SubExpiration: exp}); err != nil {
		t.Fatalf("sub time by mixed-case email: %v", err)
	}
	found, _ := findUserByID(u.ID)
	if !found.SubExpiration.Truncate(time.Second).Equal(exp) {
		t.Fatalf("sub expiration mismatch: %v vs %v", found.SubExpiration, exp)
	}

	if err := updateUserAdmin(&adminUserUpdateRequest{
		TargetUserID: u.ID,
		Email:        "New.Mixed@Example.COM",
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ = findUserByEmail("new.mixed@example.com"); got == nil {
		t.Fatal("renamed email should resolve normalized")
	}
	if got, _ = findUserByEmail("Mixed.Case@Example.COM"); got != nil {
		t.Fatal("old mixed-case email should not resolve")
	}
}

func TestOpenDB_RejectsWorldReadable(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/tunnels.db"
	if err := os.WriteFile(path, []byte("not a db"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := openDB(path); err == nil {
		t.Fatal("world-readable DB must be rejected")
	}
}
