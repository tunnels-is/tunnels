package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func TestPublicRegistrationEnabledByDefault(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{})

	code := postRegister(t, "open-default@example.com", "longenough1")
	if code != http.StatusOK {
		t.Fatalf("default register: %d", code)
	}
}

func TestDisablePublicRegistration_RejectsAnonymous(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	prev := Config.Load()
	cli := disablePublicRegistrationCLI
	t.Cleanup(func() {
		Config.Store(prev)
		disablePublicRegistrationCLI = cli
	})
	Config.Store(&types.ServerConfig{DisablePublicRegistration: true})
	disablePublicRegistrationCLI = false

	w := postRegisterRec(t, "blocked@example.com", "longenough1")
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d %s, want 403", w.Code, w.Body.String())
	}
	got, err := DB_findUserByEmail("blocked@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("user should not have been created")
	}
}

func TestDisablePublicRegistration_CLIFlag(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	prev := Config.Load()
	cli := disablePublicRegistrationCLI
	t.Cleanup(func() {
		Config.Store(prev)
		disablePublicRegistrationCLI = cli
	})
	Config.Store(&types.ServerConfig{})
	disablePublicRegistrationCLI = true

	if postRegister(t, "cli-blocked@example.com", "longenough1") != http.StatusForbidden {
		t.Fatal("CLI flag must disable public registration")
	}
}

func TestDisablePublicRegistration_AdminUIStillWorks(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	prev := Config.Load()
	cli := disablePublicRegistrationCLI
	t.Cleanup(func() {
		Config.Store(prev)
		disablePublicRegistrationCLI = cli
	})
	okKey := strings.Repeat("k", minSecretLen)
	Config.Store(&types.ServerConfig{
		CookieSigningKey:          okKey,
		TwoFactorKey:              okKey,
		DisablePublicRegistration: true,
	})
	disablePublicRegistrationCLI = false

	tok := &DeviceToken{DT: uuid.NewString(), N: "admin"}
	admin := &User{
		ID:      uuid.New(),
		Email:   "admin",
		IsAdmin: true,
		Tokens:  []*DeviceToken{tok},
	}
	if err := DB_CreateUser(admin); err != nil {
		t.Fatal(err)
	}

	cookie, err := encryptAdminCookie(admin.ID.String(), tok.DT, "192.0.2.1")
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(REGISTER_FORM{Email: "from-admin@example.com", Password: "longenough1"})
	req := httptest.NewRequest(http.MethodPost, "/ui/user/create", bytes.NewReader(body))
	req.RemoteAddr = "192.0.2.1:12345"
	req.AddCookie(&http.Cookie{Name: "admin_session", Value: cookie})
	w := httptest.NewRecorder()
	adminUIMiddleware(http.HandlerFunc(API_AdminUserCreate)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin create: %d %s", w.Code, w.Body.String())
	}
	got, err := DB_findUserByEmail("from-admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("admin UI must still be able to create users")
	}
}

func postRegister(t *testing.T, email, password string) int {
	t.Helper()
	return postRegisterRec(t, email, password).Code
}

func postRegisterRec(t *testing.T, email, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(REGISTER_FORM{Email: email, Password: password})
	req := httptest.NewRequest(http.MethodPost, "/client/user/create", bytes.NewReader(body))
	w := httptest.NewRecorder()
	API_UserCreate(w, req)
	return w
}
