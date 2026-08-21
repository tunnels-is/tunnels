package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
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
	if err := DB_CreateUser(u); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(LOGIN_FORM{Email: u.Email, Password: "longenough1", DeviceName: "attacker"})
	req := httptest.NewRequest(http.MethodPost, "/client/user/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	API_UserLogin(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("login: %d %s, want 403", w.Code, w.Body.String())
	}

	got, err := DB_findUserByEmail(u.Email)
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
	if err := DB_CreateUser(u); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(LOGIN_FORM{Email: u.Email, Password: "longenough1"})
	req := httptest.NewRequest(http.MethodPost, "/ui/user/login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	API_AdminUILogin(w, req)
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
	if err := DB_CreateUser(u); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(PASSWORD_RESET_FORM{
		Email: u.Email, Password: "newpassword1", ResetCode: "000000",
	})
	req := httptest.NewRequest(http.MethodPost, "/client/user/reset/password", bytes.NewReader(body))
	w := httptest.NewRecorder()
	API_UserResetPassword(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("reset: %d %s, want 401", w.Code, w.Body.String())
	}

	got, err := DB_findUserByEmail(u.Email)
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
