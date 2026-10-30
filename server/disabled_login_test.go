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
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Invalid login credentials") {
		t.Fatalf("login: %d %s, want 401", w.Code, w.Body.String())
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "disabled") {
		t.Fatalf("login names the disabled state: %s", w.Body.String())
	}

	got, err := findUserByEmail(u.Email)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if len(got.Tokens) != 1 || !secretMatches(got.Tokens[0].DT, keep.DT) {
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
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Invalid login credentials") {
		t.Fatalf("admin login: %d %s, want 401", w.Code, w.Body.String())
	}
	if strings.Contains(strings.ToLower(w.Body.String()), "disabled") {
		t.Fatalf("admin login names the disabled state: %s", w.Body.String())
	}
	if c := w.Result().Cookies(); len(c) > 0 {
		t.Fatal("must not set admin_session")
	}
}

func TestDisabledLogin_MatchesBadPassword(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()

	hash, err := bcrypt.GenerateFromPassword([]byte("longenough1"), 4)
	if err != nil {
		t.Fatal(err)
	}
	enabled := &User{
		ID: uuid.New(), Email: "enabled-login@test.local", Password: string(hash),
	}
	disabled := &User{
		ID: uuid.New(), Email: "disabled-login@test.local", Password: string(hash),
		Disabled: true, TwoFactorEnabled: true, TwoFactorCode: []byte("not-a-real-secret"),
		Tokens: []*DeviceToken{{DT: "keep-me", N: "phone"}},
	}
	admin := &User{
		ID: uuid.New(), Email: "disabled-admin-login@test.local", Password: string(hash),
		Disabled: true, IsAdmin: true,
	}
	for _, u := range []*User{enabled, disabled, admin} {
		if err := createUser(u); err != nil {
			t.Fatal(err)
		}
	}

	login := func(handler http.HandlerFunc, email, password string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(loginRequest{Email: email, Password: password, DeviceName: "phone"})
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handler(w, req)
		return w
	}

	bad := login(handleClientLogin, enabled.Email, "wrongpassword1")
	banned := login(handleClientLogin, disabled.Email, "longenough1")
	if bad.Code != http.StatusUnauthorized || banned.Code != bad.Code {
		t.Fatalf("client status bad=%d banned=%d", bad.Code, banned.Code)
	}
	if bad.Body.String() != banned.Body.String() {
		t.Fatalf("client body differs\nbad: %s\nbanned: %s", bad.Body.String(), banned.Body.String())
	}
	if strings.Contains(strings.ToLower(banned.Body.String()), "disabled") {
		t.Fatalf("body names the disabled state: %s", banned.Body.String())
	}

	got, err := findUserByEmail(disabled.Email)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if len(got.Tokens) != 1 || !secretMatches(got.Tokens[0].DT, "keep-me") {
		t.Fatalf("tokens mutated: %+v", got.Tokens)
	}

	adminBad := login(handleAdminLogin, enabled.Email, "wrongpassword1")
	adminBanned := login(handleAdminLogin, admin.Email, "longenough1")
	if adminBad.Code != http.StatusUnauthorized || adminBanned.Code != adminBad.Code {
		t.Fatalf("admin status bad=%d banned=%d", adminBad.Code, adminBanned.Code)
	}
	if adminBad.Body.String() != adminBanned.Body.String() {
		t.Fatalf("admin body differs\nbad: %s\nbanned: %s", adminBad.Body.String(), adminBanned.Body.String())
	}
	if len(adminBanned.Result().Cookies()) != 0 {
		t.Fatal("disabled admin login set a cookie")
	}
}

func TestAdminLogin_NonAdminMatchesBadPassword(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	resetPasswordResetAttemptsForTest()
	okKey := strings.Repeat("c", minSecretLen)
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: okKey})

	hash, err := bcrypt.GenerateFromPassword([]byte("longenough1"), 4)
	if err != nil {
		t.Fatal(err)
	}
	keep := &DeviceToken{DT: "keep-me", N: "phone"}
	member := &User{
		ID: uuid.New(), Email: "member-login@test.local", Password: string(hash),
		Tokens: []*DeviceToken{keep},
	}
	admin := &User{
		ID: uuid.New(), Email: "real-admin-login@test.local", Password: string(hash),
		IsAdmin: true,
	}
	for _, u := range []*User{member, admin} {
		if err := createUser(u); err != nil {
			t.Fatal(err)
		}
	}

	login := func(email, password string) *httptest.ResponseRecorder {
		t.Helper()
		body, _ := json.Marshal(loginRequest{Email: email, Password: password, DeviceName: "phone"})
		req := httptest.NewRequest(http.MethodPost, "/ui/user/login", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handleAdminLogin(w, req)
		return w
	}

	bad := login(member.Email, "wrongpassword1")
	good := login(member.Email, "longenough1")
	if bad.Code != http.StatusUnauthorized || good.Code != bad.Code {
		t.Fatalf("status bad=%d good=%d", bad.Code, good.Code)
	}
	if bad.Body.String() != good.Body.String() {
		t.Fatalf("body differs\nbad: %s\ngood: %s", bad.Body.String(), good.Body.String())
	}
	body := strings.ToLower(good.Body.String())
	if strings.Contains(body, "admin") || strings.Contains(body, "manager") {
		t.Fatalf("body names the role: %s", good.Body.String())
	}
	if len(good.Result().Cookies()) != 0 {
		t.Fatal("non-admin login set a cookie")
	}
	got, err := findUserByEmail(member.Email)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if len(got.Tokens) != 1 || !secretMatches(got.Tokens[0].DT, keep.DT) {
		t.Fatalf("tokens mutated: %+v", got.Tokens)
	}

	ok := login(admin.Email, "longenough1")
	if ok.Code != http.StatusOK {
		t.Fatalf("admin login: %d %s", ok.Code, ok.Body.String())
	}
	if c := ok.Result().Cookies(); len(c) != 1 || c[0].Name != "admin_session" {
		t.Fatalf("admin cookie: %+v", ok.Result().Cookies())
	}
	var adminBody User
	if err := json.Unmarshal(ok.Body.Bytes(), &adminBody); err != nil {
		t.Fatal(err)
	}
	if adminBody.DeviceToken != nil && adminBody.DeviceToken.DT != "" {
		t.Fatalf("admin login returned a device token: %#v", adminBody.DeviceToken)
	}
	for _, tok := range adminBody.Tokens {
		if tok != nil && tok.DT != "" {
			t.Fatalf("admin login returned token secret %q", tok.DT)
		}
	}

	clientReqBody, _ := json.Marshal(loginRequest{Email: admin.Email, Password: "longenough1", DeviceName: "desktop"})
	clientReq := httptest.NewRequest(http.MethodPost, "/client/user/login", bytes.NewReader(clientReqBody))
	clientW := httptest.NewRecorder()
	handleClientLogin(clientW, clientReq)
	if clientW.Code != http.StatusOK {
		t.Fatalf("client login: %d %s", clientW.Code, clientW.Body.String())
	}
	var clientBody User
	if err := json.Unmarshal(clientW.Body.Bytes(), &clientBody); err != nil {
		t.Fatal(err)
	}
	if clientBody.DeviceToken == nil || clientBody.DeviceToken.DT == "" || storedSecretIsHash(clientBody.DeviceToken.DT) {
		t.Fatalf("client login omitted the device token: %#v", clientBody.DeviceToken)
	}
	if _, err := authenticateUserFromEmailOrIDAndToken(admin.Email, uuid.Nil, clientBody.DeviceToken.DT); err != nil {
		t.Fatalf("client token does not authenticate: %v", err)
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
	if len(got.Tokens) != 1 || !secretMatches(got.Tokens[0].DT, keep.DT) {
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
