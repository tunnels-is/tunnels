package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	"github.com/xlzd/gotp"
	"golang.org/x/crypto/bcrypt"
)

func TestDisabledUser_CannotLoginOrMintTokens(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()

	hash, err := bcrypt.GenerateFromPassword([]byte("longenough1"), 13)
	if err != nil {
		t.Fatal(err)
	}
	keep := &DeviceToken{DT: uuid.NewString(), N: "legit"}
	u := &User{
		ID:       uuid.New(),
		Email:    "banned@test.local",
		Password: string(hash),
		Disabled: true,
		Tokens:   []*DeviceToken{keep},
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(loginRequest{Email: u.Email, Password: "longenough1", DeviceName: "attacker"})
	req := httptest.NewRequest(http.MethodPost, "/client/user/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleClientLogin(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("login: %d %s, want 403", w.Code, w.Body.String())
	}

	got, err := findUserByEmail(u.Email)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if len(got.Tokens) != 1 || got.Tokens[0].DT != keep.DT {
		t.Fatalf("tokens mutated: %+v", got.Tokens)
	}
}

func TestDisabledAdmin_CannotUILogin(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()

	hash, err := bcrypt.GenerateFromPassword([]byte("longenough1"), 13)
	if err != nil {
		t.Fatal(err)
	}
	u := &User{
		ID:       uuid.New(),
		Email:    "admin-banned@test.local",
		Password: string(hash),
		Disabled: true,
		IsAdmin:  true,
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(loginRequest{Email: u.Email, Password: "longenough1"})
	req := httptest.NewRequest(http.MethodPost, "/ui/user/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleAdminLogin(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("admin login: %d %s, want 403", w.Code, w.Body.String())
	}
	if c := w.Result().Cookies(); len(c) > 0 {
		t.Fatal("must not set admin_session")
	}
}

func TestDisabledUser_CannotResetPassword(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()

	hash, err := bcrypt.GenerateFromPassword([]byte("oldpassword1"), 13)
	if err != nil {
		t.Fatal(err)
	}
	keep := &DeviceToken{DT: uuid.NewString(), N: "legit"}
	u := &User{
		ID:       uuid.New(),
		Email:    "banned-reset@test.local",
		Password: string(hash),
		Disabled: true,
		Tokens:   []*DeviceToken{keep},
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(passwordResetRequest{
		Email: u.Email, Password: "newpassword1", ResetCode: "000000",
	})
	req := httptest.NewRequest(http.MethodPost, "/client/user/reset/password", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handleClientResetPassword(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("reset: %d %s, want 401", w.Code, w.Body.String())
	}

	got, err := findUserByEmail(u.Email)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.Password), []byte("oldpassword1")); err != nil {
		t.Fatal("password must not change")
	}
	if len(got.Tokens) != 1 || got.Tokens[0].DT != keep.DT {
		t.Fatalf("tokens wiped: %+v", got.Tokens)
	}
}

func TestResetPassword_RequiresRecoveryCode(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	resetPasswordResetAttemptsForTest()
	okKey := strings.Repeat("c", minSecretLen)
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: okKey})

	secret := gotp.RandomSecret(16)
	totpBlob, err := Encrypt(secret, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	keepRecovery := "RECOVERYKEEPAAAA"
	usedRecovery := "RECOVERYUSEDAAAA"
	recBlob, err := Encrypt(keepRecovery+" "+usedRecovery, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("oldpassword1"), 13)
	if err != nil {
		t.Fatal(err)
	}
	u := &User{
		ID:               uuid.New(),
		Email:            "reset-2fa@test.local",
		Password:         string(hash),
		TwoFactorEnabled: true,
		TwoFactorCode:    totpBlob,
		RecoveryCodes:    recBlob,
		SubExpiration:    time.Now().Add(time.Hour),
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	otp := gotp.NewDefaultTOTP(secret).Now()
	post := func(recovery string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(passwordResetRequest{
			Email: u.Email, Password: "newpassword1", ResetCode: otp, Recovery: recovery,
		})
		req := httptest.NewRequest(http.MethodPost, "/client/user/reset/password", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handleClientResetPassword(w, req)
		return w
	}

	if w := post(""); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing recovery: %d %s", w.Code, w.Body.String())
	}
	if w := post("NOTACODE"); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong recovery: %d %s", w.Code, w.Body.String())
	}
	got, err := findUserByEmail(u.Email)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.Password), []byte("oldpassword1")); err != nil {
		t.Fatal("password changed before a valid recovery code")
	}

	otp = gotp.NewDefaultTOTP(secret).Now()
	if w := post(usedRecovery); w.Code != http.StatusOK {
		t.Fatalf("valid reset: %d %s", w.Code, w.Body.String())
	}
	got, err = findUserByEmail(u.Email)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.Password), []byte("newpassword1")); err != nil {
		t.Fatal("password was not updated")
	}
	plain, err := Decrypt(got.RecoveryCodes, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, usedRecovery) || !strings.Contains(plain, keepRecovery) {
		t.Fatalf("recovery codes after reset: %q", plain)
	}
	otp = gotp.NewDefaultTOTP(secret).Now()
	if w := post(usedRecovery); w.Code != http.StatusUnauthorized {
		t.Fatalf("reused recovery: %d %s", w.Code, w.Body.String())
	}
}
