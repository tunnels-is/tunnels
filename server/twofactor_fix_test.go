package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	"github.com/xlzd/gotp"
	"golang.org/x/crypto/bcrypt"
)

func withTwoFactorKey(t *testing.T) string {
	t.Helper()
	okKey := strings.Repeat("c", minSecretLen)
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{CookieSigningKey: okKey, TwoFactorKey: okKey})
	return okKey
}

func postTwoFactorConfirm(t *testing.T, user *User, password, recovery, secret string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(twoFactorRequest{
		Password: password,
		Recovery: recovery,
		Code:     secret,
		Digits:   gotp.NewDefaultTOTP(secret).Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/client/user/2fa/confirm", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, user))
	w := httptest.NewRecorder()
	handleClientTwoFactorConfirm(w, req)
	return w
}

// TestValidTOTPSecret ...
func TestValidTOTPSecret(t *testing.T) {
	if !validTOTPSecret("ABCDEFGHIJKLMNOP") {
		t.Fatal("client-style secret must be accepted")
	}
	if validTOTPSecret("") || validTOTPSecret("   ") || validTOTPSecret("!!!!") || validTOTPSecret("1111") {
		t.Fatal("empty or non-base32 secret must be rejected")
	}
}

// TestTwoFactorConfirm_RejectsBadSecretBeforeEnrollment ...
func TestTwoFactorConfirm_RejectsBadSecretBeforeEnrollment(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	_ = withTwoFactorKey(t)

	hash, err := bcrypt.GenerateFromPassword([]byte("longenough1"), 4)
	if err != nil {
		t.Fatal(err)
	}
	u := &User{ID: uuid.New(), Email: "bad-secret@test.local", Password: string(hash)}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	for _, secret := range []string{"", "!!!!", "1111"} {
		body, _ := json.Marshal(twoFactorRequest{
			Password: "longenough1", Code: secret, Digits: "000000",
		})
		req := httptest.NewRequest(http.MethodPost, "/client/user/2fa/confirm", bytes.NewReader(body))
		req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, u))
		w := httptest.NewRecorder()
		handleClientTwoFactorConfirm(w, req)
		if w.Code != 400 || !strings.Contains(w.Body.String(), "invalid two-factor secret") {
			t.Fatalf("secret %q: %d %s", secret, w.Code, w.Body.String())
		}
	}

	got, err := findUserByID(u.ID)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	if got.TwoFactorEnabled || len(got.TwoFactorCode) != 0 || len(got.RecoveryCodes) != 0 {
		t.Fatalf("bad secret changed the account: %+v", got)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(got.Password), []byte("longenough1")); err != nil {
		t.Fatal("password hash changed")
	}
}

// TestTwoFactorConfirm_ConsumesRecoveryAndKeepsPassword ...
func TestTwoFactorConfirm_ConsumesRecoveryAndKeepsPassword(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	okKey := withTwoFactorKey(t)

	const password = "longenough1"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	if err != nil {
		t.Fatal(err)
	}
	oldSecret := "ABCDEFGHIJKLMNOP"
	newSecret := "QRSTUVWXYZ234567"
	totpBlob, err := Encrypt(oldSecret, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	const keep = "RECOVERYKEEPAAAA"
	const used = "RECOVERYUSEDAAAA"
	recBlob, err := Encrypt(keep+" "+used, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	u := &User{
		ID:               uuid.New(),
		Email:            "reenroll@test.local",
		Password:         string(hash),
		TwoFactorEnabled: true,
		TwoFactorCode:    totpBlob,
		RecoveryCodes:    recBlob,
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	if w := postTwoFactorConfirm(t, u, "wrongpassword", used, newSecret); w.Code != 401 {
		t.Fatalf("wrong password: %d %s", w.Code, w.Body.String())
	}
	stored, err := findUserByID(u.ID)
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	plain, err := Decrypt(stored.RecoveryCodes, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain, used) || !strings.Contains(plain, keep) {
		t.Fatalf("recovery consumed on a bad password: %q", plain)
	}
	secret, err := Decrypt(stored.TwoFactorCode, []byte(okKey))
	if err != nil || secret != oldSecret {
		t.Fatalf("secret changed on a bad password: %q %v", secret, err)
	}

	badDigits := "000000"
	if gotp.NewDefaultTOTP(newSecret).Now() == badDigits {
		badDigits = "111111"
	}
	body, _ := json.Marshal(twoFactorRequest{
		Password: password, Recovery: used, Code: newSecret, Digits: badDigits,
	})
	req := httptest.NewRequest(http.MethodPost, "/client/user/2fa/confirm", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), contextKeyUser, u))
	w := httptest.NewRecorder()
	handleClientTwoFactorConfirm(w, req)
	if w.Code != 400 {
		t.Fatalf("wrong digits: %d %s", w.Code, w.Body.String())
	}
	stored, _ = findUserByID(u.ID)
	plain, err = Decrypt(stored.RecoveryCodes, []byte(okKey))
	if err != nil || !strings.Contains(plain, used) {
		t.Fatalf("recovery consumed on bad digits: %q %v", plain, err)
	}

	w = postTwoFactorConfirm(t, u, password, used, newSecret)
	if w.Code != 200 {
		t.Fatalf("re-enroll: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data string `json:"Data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	issued := strings.Fields(resp.Data)
	if len(issued) != 2 || issued[0] == used || issued[1] == used {
		t.Fatalf("issued codes: %q", resp.Data)
	}
	stored, _ = findUserByID(u.ID)
	plain, err = Decrypt(stored.RecoveryCodes, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, used) || strings.Contains(plain, keep) || plain != resp.Data {
		t.Fatalf("stored recovery: %q issued %q", plain, resp.Data)
	}
	secret, err = Decrypt(stored.TwoFactorCode, []byte(okKey))
	if err != nil || secret != newSecret {
		t.Fatalf("stored secret: %q %v", secret, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(password)); err != nil {
		t.Fatal("password hash changed")
	}
	if stored.Email != u.Email {
		t.Fatalf("email changed: %s", stored.Email)
	}

	if w := postTwoFactorConfirm(t, u, password, used, "MMMMMMMMMMMMMMMM"); w.Code != 401 {
		t.Fatalf("reused recovery: %d %s", w.Code, w.Body.String())
	}
	stored, _ = findUserByID(u.ID)
	secret, err = Decrypt(stored.TwoFactorCode, []byte(okKey))
	if err != nil || secret != newSecret {
		t.Fatalf("reused recovery rewrote the secret: %q %v", secret, err)
	}

	digits := gotp.NewDefaultTOTP(newSecret).Now()
	if digits == stored.LastTOTP {
		if err := validateUserTwoFactor(stored, &loginRequest{Digits: digits}); err == nil {
			t.Fatal("code used at enrollment was accepted again")
		}
	} else if err := validateUserTwoFactor(stored, &loginRequest{Digits: digits}); err != nil {
		t.Fatal(err)
	}
	if err := validateUserTwoFactor(stored, &loginRequest{Recovery: issued[0]}); err != nil {
		t.Fatal(err)
	}
	if err := validateUserTwoFactor(stored, &loginRequest{Recovery: issued[0]}); err == nil {
		t.Fatal("login accepted a recovery code twice")
	}
}

// TestTwoFactorConfirm_FirstEnrollmentStillOmitsRecovery ...
func TestTwoFactorConfirm_FirstEnrollmentStillOmitsRecovery(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	okKey := withTwoFactorKey(t)

	const password = "longenough1"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	if err != nil {
		t.Fatal(err)
	}
	u := &User{ID: uuid.New(), Email: "first-2fa@test.local", Password: string(hash)}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	secret := "ABCDEFGHIJKLMNOP"
	w := postTwoFactorConfirm(t, u, password, "", secret)
	if w.Code != 200 {
		t.Fatalf("enroll: %d %s", w.Code, w.Body.String())
	}
	stored, err := findUserByID(u.ID)
	if err != nil || stored == nil || !stored.TwoFactorEnabled {
		t.Fatalf("not enrolled: %+v %v", stored, err)
	}
	got, err := Decrypt(stored.TwoFactorCode, []byte(okKey))
	if err != nil || got != secret {
		t.Fatalf("secret: %q %v", got, err)
	}
	if _, err := Decrypt(stored.RecoveryCodes, []byte(okKey)); err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(password)); err != nil {
		t.Fatal("password hash changed")
	}

	if w := postTwoFactorConfirm(t, u, password, "", "QRSTUVWXYZ234567"); w.Code != 401 {
		t.Fatalf("second enroll without recovery: %d %s", w.Code, w.Body.String())
	}
	stored, _ = findUserByID(u.ID)
	got, err = Decrypt(stored.TwoFactorCode, []byte(okKey))
	if err != nil || got != secret {
		t.Fatalf("second enroll rewrote the secret: %q %v", got, err)
	}
}

// TestTwoFactorConfirm_OneRecoveryCodeWinsTheRace ...
func TestTwoFactorConfirm_OneRecoveryCodeWinsTheRace(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	okKey := withTwoFactorKey(t)

	const password = "longenough1"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 4)
	if err != nil {
		t.Fatal(err)
	}
	recBlob, err := Encrypt("RACECODEAAAAAAA SPARECODEAAAAAA", []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	oldBlob, err := Encrypt("ABCDEFGHIJKLMNOP", []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	u := &User{
		ID: uuid.New(), Email: "race-2fa@test.local", Password: string(hash),
		TwoFactorEnabled: true, TwoFactorCode: oldBlob, RecoveryCodes: recBlob,
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	codes := make([]int, 2)
	secrets := []string{"QRSTUVWXYZ234567", "MMMMMMMMMMMMMMMM"}
	for i := range secrets {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := postTwoFactorConfirm(t, u, password, "RACECODEAAAAAAA", secrets[i])
			codes[i] = w.Code
		}(i)
	}
	wg.Wait()

	wins := 0
	for _, code := range codes {
		if code == 200 {
			wins++
		} else if code != 401 {
			t.Fatalf("codes: %v", codes)
		}
	}
	if wins != 1 {
		t.Fatalf("wins: %d codes %v", wins, codes)
	}
	stored, err := findUserByID(u.ID)
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	plain, err := Decrypt(stored.RecoveryCodes, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "RACECODEAAAAAAA") {
		t.Fatalf("race code still stored: %q", plain)
	}
	got, err := Decrypt(stored.TwoFactorCode, []byte(okKey))
	if err != nil || (got != secrets[0] && got != secrets[1]) {
		t.Fatalf("secret: %q %v", got, err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(password)); err != nil {
		t.Fatal("password hash changed")
	}
}

// TestTOTPCode_RejectedWhenReused ...
func TestTOTPCode_RejectedWhenReused(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	okKey := withTwoFactorKey(t)

	secret := "ABCDEFGHIJKLMNOP"
	blob, err := Encrypt(secret, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	u := &User{
		ID: uuid.New(), Email: "totp-reuse@test.local",
		TwoFactorEnabled: true, TwoFactorCode: blob,
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	digits := gotp.NewDefaultTOTP(secret).Now()
	if err := validateUserTwoFactor(u, &loginRequest{Digits: digits}); err != nil {
		t.Fatal(err)
	}
	if err := validateUserTwoFactor(u, &loginRequest{Digits: digits}); err == nil {
		t.Fatal("reused authenticator code was accepted")
	}
	stored, err := findUserByID(u.ID)
	if err != nil || stored == nil || stored.LastTOTP != digits {
		t.Fatalf("stored code: %+v %v", stored, err)
	}
	stored.RemoveSensitiveInformation()
	if stored.LastTOTP != "" {
		t.Fatal("last authenticator code was left on the user")
	}
}

// TestResetPassword_RejectsReusedTOTP ...
func TestResetPassword_RejectsReusedTOTP(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	resetPasswordResetAttemptsForTest()
	okKey := withTwoFactorKey(t)

	secret := "QRSTUVWXYZ234567"
	totpBlob, err := Encrypt(secret, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	const spare = "SPARECODERESET11"
	const used = "USEDCODERESET111"
	recBlob, err := Encrypt(spare+" "+used, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("oldpassword1"), 4)
	if err != nil {
		t.Fatal(err)
	}
	u := &User{
		ID: uuid.New(), Email: "reset-reuse@test.local", Password: string(hash),
		TwoFactorEnabled: true, TwoFactorCode: totpBlob, RecoveryCodes: recBlob,
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	otp := gotp.NewDefaultTOTP(secret).Now()
	post := func(recovery, password string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(passwordResetRequest{
			Email: u.Email, Password: password, ResetCode: otp, Recovery: recovery,
		})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/client/user/reset/password", bytes.NewReader(body))
		w := httptest.NewRecorder()
		handleClientResetPassword(w, req)
		return w
	}

	if w := post("NOTACODE", "newpassword1"); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong recovery: %d %s", w.Code, w.Body.String())
	}
	stored, err := findUserByID(u.ID)
	if err != nil || stored == nil || stored.LastTOTP != "" {
		t.Fatalf("wrong recovery recorded a code: %+v %v", stored, err)
	}

	if w := post(used, "newpassword1"); w.Code != http.StatusOK {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}
	if w := post(spare, "otherpassword1"); w.Code != http.StatusUnauthorized {
		t.Fatalf("reused code: %d %s", w.Code, w.Body.String())
	}
	stored, err = findUserByID(u.ID)
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte("newpassword1")); err != nil {
		t.Fatal("password changed on the reused code")
	}
	if stored.LastTOTP != otp {
		t.Fatalf("stored code %q", stored.LastTOTP)
	}
	plain, err := Decrypt(stored.RecoveryCodes, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, used) || !strings.Contains(plain, spare) {
		t.Fatalf("recovery codes: %q", plain)
	}
}

// TestAcceptResetAuth_WritesPasswordWithCode ...
func TestAcceptResetAuth_WritesPasswordWithCode(t *testing.T) {
	setupTestDB(t)
	okKey := withTwoFactorKey(t)
	const keep = "KEEPCODERESET111"
	const used = "USEDCODERESET222"
	recBlob, err := Encrypt(keep+" "+used, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	u := &User{
		ID: uuid.New(), Email: "atomic-reset@test.local", Password: "old-hash",
		APIKey: "atomic-api-key", RecoveryCodes: recBlob,
		Tokens: []*DeviceToken{{DT: "tok", N: "n"}},
	}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}

	unchanged := func() {
		t.Helper()
		stored, err := findUserByID(u.ID)
		if err != nil || stored == nil {
			t.Fatal(err)
		}
		if stored.Password != "old-hash" || stored.LastTOTP != "" || stored.APIKey == "" || len(stored.Tokens) != 1 {
			t.Fatalf("reset mutated the user: %+v", stored)
		}
		if !apiKeyIndexHas(t, "atomic-api-key") {
			t.Fatal("api key index dropped before the password write")
		}
		plain, err := Decrypt(stored.RecoveryCodes, []byte(okKey))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(plain, keep) || !strings.Contains(plain, used) {
			t.Fatalf("recovery codes: %q", plain)
		}
	}

	if err := acceptResetAuth(u.ID, "123456", "NOPE", "new-hash"); !errors.Is(err, errRecoveryNotFound) {
		t.Fatalf("wrong recovery: %v", err)
	}
	unchanged()
	if err := acceptResetAuth(u.ID, "", used, "new-hash"); !errors.Is(err, errTOTPReused) {
		t.Fatalf("empty code: %v", err)
	}
	unchanged()
	if err := acceptResetAuth(u.ID, "123456", used, ""); err == nil {
		t.Fatal("empty hash was accepted")
	}
	unchanged()

	if err := acceptResetAuth(u.ID, "123456", used, "new-hash"); err != nil {
		t.Fatal(err)
	}
	stored, err := findUserByID(u.ID)
	if err != nil || stored == nil {
		t.Fatal(err)
	}
	if stored.Password != "new-hash" || stored.LastTOTP != "123456" || stored.APIKey != "" || len(stored.Tokens) != 0 {
		t.Fatalf("committed reset: %+v", stored)
	}
	if apiKeyIndexHas(t, "atomic-api-key") {
		t.Fatal("api key index still contains the reset key")
	}
	plain, err := Decrypt(stored.RecoveryCodes, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, used) || !strings.Contains(plain, keep) {
		t.Fatalf("recovery codes: %q", plain)
	}
	if err := acceptResetAuth(u.ID, "123456", keep, "other-hash"); !errors.Is(err, errTOTPReused) {
		t.Fatalf("reused code: %v", err)
	}
	stored, err = findUserByID(u.ID)
	if err != nil || stored == nil || stored.Password != "new-hash" {
		t.Fatalf("reused code changed the password: %+v %v", stored, err)
	}
}

// TestValidateUserTwoFactor_PanicFailsClosed ...
func TestValidateUserTwoFactor_PanicFailsClosed(t *testing.T) {
	setupTestDB(t)
	okKey := withTwoFactorKey(t)
	bad, err := Encrypt("!!!not-base32!!!", []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	u := &User{ID: uuid.New(), TwoFactorEnabled: true, TwoFactorCode: bad}
	if err := validateUserTwoFactor(u, &loginRequest{Digits: "000000"}); err == nil {
		t.Fatal("panic returned success")
	}

	goodSecret := "ABCDEFGHIJKLMNOP"
	good, err := Encrypt(goodSecret, []byte(okKey))
	if err != nil {
		t.Fatal(err)
	}
	u.TwoFactorCode = good
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	goodDigits := gotp.NewDefaultTOTP(goodSecret).Now()
	if err := validateUserTwoFactor(u, &loginRequest{Digits: goodDigits}); err != nil {
		t.Fatalf("existing secret rejected: %v", err)
	}
	badDigits := "000000"
	if badDigits == goodDigits {
		badDigits = "111111"
	}
	if err := validateUserTwoFactor(u, &loginRequest{Digits: badDigits}); err == nil {
		t.Fatal("wrong authenticator code accepted")
	}
}
