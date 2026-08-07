package main

import (
	"sync"
	"testing"
	"time"
)

func resetPasswordResetAttemptsForTest() {
	passwordResetAttempts.Range(func(key, _ any) bool {
		passwordResetAttempts.Delete(key)
		return true
	})
}

func TestPasswordResetAllowed_FirstAttempt(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	if !passwordResetAllowed("user@example.com") {
		t.Fatal("first attempt should be allowed")
	}
}

func TestPasswordReset_FiveFailuresThenBlocked(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "limit@example.com"

	for i := 0; i < passwordResetMaxTries; i++ {
		if !passwordResetAllowed(email) {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
		recordPasswordResetFailure(email)
	}
	if passwordResetAllowed(email) {
		t.Fatal("6th attempt should be blocked after 5 failures")
	}
}

func TestPasswordReset_WindowExpiry(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "expire@example.com"

	passwordResetAttempts.Store(normalizeResetEmail(email), &ResetTries{
		Count: passwordResetMaxTries,
		Time:  time.Now().Add(-passwordResetWindow - time.Second),
	})

	if !passwordResetAllowed(email) {
		t.Fatal("expired window should allow attempts again")
	}
}

func TestPasswordReset_ClearOnSuccess(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "clear@example.com"

	for i := 0; i < passwordResetMaxTries; i++ {
		recordPasswordResetFailure(email)
	}
	if passwordResetAllowed(email) {
		t.Fatal("expected blocked before clear")
	}

	clearPasswordResetAttempts(email)
	if !passwordResetAllowed(email) {
		t.Fatal("expected allowed after clear")
	}
}

func TestPasswordReset_EmailNormalization(t *testing.T) {
	resetPasswordResetAttemptsForTest()

	for i := 0; i < passwordResetMaxTries; i++ {
		recordPasswordResetFailure("  User@Example.COM ")
	}
	if passwordResetAllowed("user@example.com") {
		t.Fatal("normalized email should share the same bucket")
	}
}

func TestCleanPasswordResetAttempts(t *testing.T) {
	resetPasswordResetAttemptsForTest()

	passwordResetAttempts.Store("old@example.com", &ResetTries{
		Count: 3,
		Time:  time.Now().Add(-passwordResetWindow - time.Minute),
	})
	passwordResetAttempts.Store("new@example.com", &ResetTries{
		Count: 2,
		Time:  time.Now(),
	})

	cleanPasswordResetAttempts()

	if _, ok := passwordResetAttempts.Load("old@example.com"); ok {
		t.Fatal("expired entry should be cleaned")
	}
	if _, ok := passwordResetAttempts.Load("new@example.com"); !ok {
		t.Fatal("fresh entry should remain")
	}
}

func TestPasswordReset_ConcurrentRecords(t *testing.T) {
	resetPasswordResetAttemptsForTest()
	email := "race@example.com"

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recordPasswordResetFailure(email)
		}()
	}
	wg.Wait()

	v, ok := passwordResetAttempts.Load(normalizeResetEmail(email))
	if !ok {
		t.Fatal("expected entry after concurrent failures")
	}
	rt := v.(*ResetTries)
	if rt.Count < 1 {
		t.Fatalf("expected count >= 1, got %d", rt.Count)
	}
}
