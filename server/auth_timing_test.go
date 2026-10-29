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
)

func TestLogin_UnknownEmailPaysBcrypt(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	resetPasswordResetAttemptsForTest()

	req := httptest.NewRequest(http.MethodPost, "/client/user/login", nil)
	req.RemoteAddr = "192.0.2.60:1234"
	w := httptest.NewRecorder()
	start := time.Now()
	user, email := authenticatePasswordLogin(w, req, &loginRequest{
		Email: "missing@example.com", Password: "longenough1",
	})
	elapsed := time.Since(start)
	if user != nil || email != "missing@example.com" {
		t.Fatalf("user=%v email=%q", user, email)
	}
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Invalid login credentials") {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("unknown email returned in %s, want a cost-13 compare", elapsed)
	}

	w = httptest.NewRecorder()
	user, _ = authenticatePasswordLogin(w, req, &loginRequest{
		Email: "missing@example.com", Password: "dummy-equalize-not-a-login",
	})
	if user != nil || w.Code != http.StatusUnauthorized {
		t.Fatalf("dummy hash match logged in: user=%v %d %s", user, w.Code, w.Body.String())
	}
}

func TestLogin_EmptyEmailAndRateLimitStayFast(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	resetPasswordResetAttemptsForTest()

	req := httptest.NewRequest(http.MethodPost, "/client/user/login", nil)
	req.RemoteAddr = "192.0.2.63:1234"
	w := httptest.NewRecorder()
	start := time.Now()
	user, _ := authenticatePasswordLogin(w, req, &loginRequest{Email: "  ", Password: "longenough1"})
	if user != nil || w.Code != http.StatusUnauthorized {
		t.Fatalf("empty email: user=%v %d %s", user, w.Code, w.Body.String())
	}
	if elapsed := time.Since(start); elapsed >= 100*time.Millisecond {
		t.Fatalf("empty email took %s", elapsed)
	}

	ip := "192.0.2.63"
	key, _ := authLimitKey(authLimitLogin, "missing-limited@example.com", ip)
	authLimitMu.Lock()
	authLimits[key] = &authWindow{count: authLoginMax, start: time.Now()}
	authLimitMu.Unlock()

	w = httptest.NewRecorder()
	start = time.Now()
	user, _ = authenticatePasswordLogin(w, req, &loginRequest{
		Email: "missing-limited@example.com", Password: "longenough1",
	})
	if user != nil || w.Code != http.StatusTooManyRequests {
		t.Fatalf("limited unknown: user=%v %d %s", user, w.Code, w.Body.String())
	}
	if elapsed := time.Since(start); elapsed >= 100*time.Millisecond {
		t.Fatalf("limited unknown email took %s", elapsed)
	}
}

func TestRegister_RejectedNamePaysBcrypt(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	resetPasswordResetAttemptsForTest()
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{})

	if err := createUser(&User{ID: uuid.New(), Email: "taken@example.com", Password: "x"}); err != nil {
		t.Fatal(err)
	}

	call := func(email, password, ip string) (*httptest.ResponseRecorder, time.Duration) {
		t.Helper()
		body, _ := json.Marshal(registerRequest{Email: email, Password: password})
		req := httptest.NewRequest(http.MethodPost, "/client/user/create", bytes.NewReader(body))
		req.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		start := time.Now()
		createUserFromRequest(w, req, true)
		return w, time.Since(start)
	}

	w, elapsed := call("admin", "longenough1", "192.0.2.61")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), registrationDenied) {
		t.Fatalf("reserved: %d %s", w.Code, w.Body.String())
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("reserved name returned in %s, want a cost-13 compare", elapsed)
	}
	if got, _ := findUserByEmail("admin"); got != nil {
		t.Fatal("reserved name was created")
	}

	w, elapsed = call("taken@example.com", "longenough1", "192.0.2.61")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), registrationDenied) {
		t.Fatalf("taken: %d %s", w.Code, w.Body.String())
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("taken name returned in %s, want a cost-13 compare", elapsed)
	}

	w, elapsed = call("admin", "short", "192.0.2.61")
	if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), registrationDenied) {
		t.Fatalf("short password: %d %s", w.Code, w.Body.String())
	}
	if elapsed >= 100*time.Millisecond {
		t.Fatalf("short password took %s", elapsed)
	}

	ip := "192.0.2.62"
	key, _ := authLimitKey(authLimitRegister, "", ip)
	authLimitMu.Lock()
	authLimits[key] = &authWindow{count: authRegisterMax, start: time.Now()}
	authLimitMu.Unlock()

	w, elapsed = call("admin", "longenough1", ip)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("limited reserved: %d %s", w.Code, w.Body.String())
	}
	if elapsed >= 100*time.Millisecond {
		t.Fatalf("limited reserved name took %s", elapsed)
	}
}
