package main

import (
	"strings"
	"sync"
	"time"
)

const (
	passwordResetMaxTries   = 5
	passwordResetWindow     = 15 * time.Minute
	passwordResetCleanEvery = 1 * time.Minute
)

// ResetTries tracks failed password-reset attempts for a single email.
type ResetTries struct {
	Count int
	Time  time.Time
}

// passwordResetAttempts maps normalized email → *ResetTries.
var passwordResetAttempts sync.Map

func normalizeResetEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// passwordResetAllowed reports whether a reset attempt may proceed for email.
// Expired entries (older than 15 minutes) are treated as cleared.
func passwordResetAllowed(email string) bool {
	email = normalizeResetEmail(email)
	if email == "" {
		return false
	}
	v, ok := passwordResetAttempts.Load(email)
	if !ok {
		return true
	}
	rt := v.(*ResetTries)
	if time.Since(rt.Time) > passwordResetWindow {
		passwordResetAttempts.Delete(email)
		return true
	}
	return rt.Count < passwordResetMaxTries
}

// recordPasswordResetFailure increments the failure counter for email.
// The window clock starts on the first failure and is not extended by later failures.
func recordPasswordResetFailure(email string) {
	email = normalizeResetEmail(email)
	if email == "" {
		return
	}
	now := time.Now()
	v, ok := passwordResetAttempts.Load(email)
	if !ok {
		passwordResetAttempts.Store(email, &ResetTries{Count: 1, Time: now})
		return
	}
	rt := v.(*ResetTries)
	if time.Since(rt.Time) > passwordResetWindow {
		passwordResetAttempts.Store(email, &ResetTries{Count: 1, Time: now})
		return
	}
	passwordResetAttempts.Store(email, &ResetTries{Count: rt.Count + 1, Time: rt.Time})
}

// clearPasswordResetAttempts removes rate-limit state for email (successful login or reset).
func clearPasswordResetAttempts(email string) {
	email = normalizeResetEmail(email)
	if email == "" {
		return
	}
	passwordResetAttempts.Delete(email)
}

// cleanPasswordResetAttempts drops entries whose window has expired.
// Intended to run once per minute via the background signal worker.
func cleanPasswordResetAttempts() {
	passwordResetAttempts.Range(func(key, value any) bool {
		rt, ok := value.(*ResetTries)
		if !ok || time.Since(rt.Time) > passwordResetWindow {
			passwordResetAttempts.Delete(key)
		}
		return true
	})
}
