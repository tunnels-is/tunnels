package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

func resetPasswordResetAttemptsForTest() {
	authLimitMu.Lock()
	authLimits = map[string]*authWindow{}
	authLimitMu.Unlock()
}

// TestAuthSlot_FirstAttempt ...
func TestAuthSlot_FirstAttempt(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	if !takeAuthSlot(authLimitLogin, "user@example.com", "192.0.2.1") {
		t.Fatal("first attempt should be allowed")
	}
}

// TestAuthSlot_LoginAndResetAreSeparate ...
func TestAuthSlot_LoginAndResetAreSeparate(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "split@example.com"
	ip := "192.0.2.2"
	for i := 0; i < authResetMax; i++ {
		if !takeAuthSlot(authLimitReset, email, ip) {
			t.Fatalf("reset attempt %d should be allowed", i+1)
		}
	}
	if takeAuthSlot(authLimitReset, email, ip) {
		t.Fatal("reset should be blocked")
	}
	if !takeAuthSlot(authLimitLogin, email, ip) {
		t.Fatal("login should not share the reset counter")
	}
	if !takeAuthSlot(authLimitReset, email, "192.0.2.3") {
		t.Fatal("another address should have its own reset counter")
	}
}

// TestAuthSlot_LoginBlocksAtTen ...
func TestAuthSlot_LoginBlocksAtTen(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "login@example.com"
	ip := "192.0.2.4"
	for i := 0; i < authLoginMax; i++ {
		if !takeAuthSlot(authLimitLogin, email, ip) {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if takeAuthSlot(authLimitLogin, email, ip) {
		t.Fatal("11th login attempt should be blocked")
	}
}

// TestAuthSlot_RegisterBlocksAtFiftyAndIgnoresClear ...
func TestAuthSlot_RegisterBlocksAtFiftyAndIgnoresClear(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	ip := "192.0.2.5"
	for i := 0; i < authRegisterMax; i++ {
		if !takeAuthSlot(authLimitRegister, "", ip) {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	clearAuthSlot(authLimitRegister, "", ip)
	if takeAuthSlot(authLimitRegister, "", ip) {
		t.Fatal("register counter must stay until the window expires")
	}
}

// TestAuthSlot_WindowExpiry ...
func TestAuthSlot_WindowExpiry(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	key, _ := authLimitKey(authLimitReset, "expire@example.com", "192.0.2.6")
	authLimitMu.Lock()
	authLimits[key] = &authWindow{count: authResetMax, start: time.Now().Add(-authLimitWindow - time.Second)}
	authLimitMu.Unlock()

	if !takeAuthSlot(authLimitReset, "expire@example.com", "192.0.2.6") {
		t.Fatal("expired window should allow attempts again")
	}
}

// TestAuthSlot_ClearOnSuccess ...
func TestAuthSlot_ClearOnSuccess(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "clear@example.com"
	ip := "192.0.2.7"
	for i := 0; i < authLoginMax; i++ {
		if !takeAuthSlot(authLimitLogin, email, ip) {
			t.Fatalf("attempt %d", i+1)
		}
	}
	if takeAuthSlot(authLimitLogin, email, ip) {
		t.Fatal("expected blocked before clear")
	}
	clearAuthSlot(authLimitLogin, email, ip)
	if !takeAuthSlot(authLimitLogin, email, ip) {
		t.Fatal("expected allowed after clear")
	}
}

// TestAuthSlot_EmailNormalization ...
func TestAuthSlot_EmailNormalization(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	ip := "192.0.2.8"
	for i := 0; i < authResetMax; i++ {
		if !takeAuthSlot(authLimitReset, "  User@Example.COM ", ip) {
			t.Fatalf("attempt %d", i+1)
		}
	}
	if takeAuthSlot(authLimitReset, "user@example.com", ip) {
		t.Fatal("normalized email should share the same bucket")
	}
}

// TestCleanPasswordResetAttempts ...
func TestCleanPasswordResetAttempts(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	oldKey, _ := authLimitKey(authLimitLogin, "old@example.com", "192.0.2.9")
	newKey, _ := authLimitKey(authLimitLogin, "new@example.com", "192.0.2.9")
	authLimitMu.Lock()
	authLimits[oldKey] = &authWindow{count: 3, start: time.Now().Add(-authLimitWindow - time.Minute)}
	authLimits[newKey] = &authWindow{count: 2, start: time.Now()}
	authLimitMu.Unlock()

	cleanPasswordResetAttempts()

	authLimitMu.Lock()
	defer authLimitMu.Unlock()
	if _, ok := authLimits[oldKey]; ok {
		t.Fatal("expired entry should be cleaned")
	}
	if _, ok := authLimits[newKey]; !ok {
		t.Fatal("fresh entry should remain")
	}
}

// TestAuthSlot_EmailBudgetBlocksAcrossAddresses ...
func TestAuthSlot_EmailBudgetBlocksAcrossAddresses(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "spread@example.com"
	for i := 0; i < authEmailMax; i++ {
		ip := fmt.Sprintf("198.51.100.%d", i+1)
		if !takeAuthSlot(authLimitLogin, email, ip) {
			t.Fatalf("attempt %d from %s should be allowed", i+1, ip)
		}
	}
	if takeAuthSlot(authLimitLogin, email, "198.51.100.200") {
		t.Fatal("email budget should block a fresh address")
	}
	if takeAuthSlot(authLimitLogin, email, "198.51.100.1") {
		t.Fatal("email budget should block an address that still has room")
	}
	if !takeAuthSlot(authLimitLogin, "other@example.com", "198.51.100.200") {
		t.Fatal("another account should keep its own email budget")
	}
	if !takeAuthSlot(authLimitReset, email, "198.51.100.200") {
		t.Fatal("reset should not share the login email budget")
	}
}

// TestAuthSlot_ResetEmailBudget ...
func TestAuthSlot_ResetEmailBudget(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "spread-reset@example.com"
	for i := 0; i < authEmailMax; i++ {
		ip := fmt.Sprintf("198.51.101.%d", i+1)
		if !takeAuthSlot(authLimitReset, email, ip) {
			t.Fatalf("reset %d from %s should be allowed", i+1, ip)
		}
	}
	if takeAuthSlot(authLimitReset, email, "198.51.101.200") {
		t.Fatal("reset email budget should block a fresh address")
	}
}

// TestAuthSlot_ClearResetsEmailBudget ...
func TestAuthSlot_ClearResetsEmailBudget(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "cleared-spread@example.com"
	var lastIP string
	for i := 0; i < authEmailMax; i++ {
		lastIP = fmt.Sprintf("198.51.102.%d", i+1)
		if !takeAuthSlot(authLimitLogin, email, lastIP) {
			t.Fatalf("attempt %d", i+1)
		}
	}
	if takeAuthSlot(authLimitLogin, email, "198.51.102.200") {
		t.Fatal("expected the email budget to be spent")
	}
	clearAuthSlot(authLimitLogin, email, lastIP)
	if !takeAuthSlot(authLimitLogin, email, "198.51.102.200") {
		t.Fatal("a successful login should clear the email budget")
	}
}

// TestAuthSlot_EmailWindowExpiry ...
func TestAuthSlot_EmailWindowExpiry(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "email-expire@example.com"
	ip := "198.51.102.9"
	key, max := authEmailKey(authLimitLogin, email)
	authLimitMu.Lock()
	authLimits[key] = &authWindow{count: max, start: time.Now().Add(-authLimitWindow - time.Second)}
	authLimitMu.Unlock()
	if !takeAuthSlot(authLimitLogin, email, ip) {
		t.Fatal("expired email window should allow attempts again")
	}
}

// TestAuthSlot_EmailBudgetConcurrent ...
func TestAuthSlot_EmailBudgetConcurrent(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "spread-race@example.com"
	const attempts = 40
	var wg sync.WaitGroup
	var allowed int
	var mu sync.Mutex
	for i := 0; i < attempts; i++ {
		ip := fmt.Sprintf("198.51.103.%d", (i%20)+1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if takeAuthSlot(authLimitLogin, email, ip) {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != authEmailMax {
		t.Fatalf("allowed %d, want %d", allowed, authEmailMax)
	}
}

// TestAuthSlot_ConcurrentRecords ...
func TestAuthSlot_ConcurrentRecords(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "race@example.com"
	ip := "192.0.2.10"

	var wg sync.WaitGroup
	var allowed int
	var mu sync.Mutex
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if takeAuthSlot(authLimitLogin, email, ip) {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if allowed != authLoginMax {
		t.Fatalf("allowed %d, want %d", allowed, authLoginMax)
	}
}

// TestRegister_LimitedBeforeBcrypt ...
func TestRegister_LimitedBeforeBcrypt(t *testing.T) {
	setupTestDB(t)
	securityReviewLogger()
	resetPasswordResetAttemptsForTest()
	prev := Config.Load()
	t.Cleanup(func() { Config.Store(prev) })
	Config.Store(&types.ServerConfig{})

	ip := "192.0.2.11"
	key, _ := authLimitKey(authLimitRegister, "", ip)
	authLimitMu.Lock()
	authLimits[key] = &authWindow{count: authRegisterMax, start: time.Now()}
	authLimitMu.Unlock()

	body, _ := json.Marshal(registerRequest{Email: "new-limited@example.com", Password: "longenough1", Password2: "longenough1"})
	req := httptest.NewRequest(http.MethodPost, "/client/user/create", bytes.NewReader(body))
	req.RemoteAddr = ip + ":1234"
	w := httptest.NewRecorder()
	handleClientUserCreate(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	if got, _ := findUserByEmail("new-limited@example.com"); got != nil {
		t.Fatal("limited registration must not create the user")
	}
}
