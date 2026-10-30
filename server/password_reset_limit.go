package main

import (
	"sync"
	"time"
)

const (
	authLoginMax            = 10
	authResetMax            = 5
	authRegisterMax         = 50
	authEmailMax            = 20
	authLimitWindow         = 15 * time.Minute
	passwordResetCleanEvery = 1 * time.Minute
)

type authLimitKind int

const (
	authLimitLogin authLimitKind = iota
	authLimitReset
	authLimitRegister
)

type authWindow struct {
	count int
	start time.Time
}

var (
	authLimitMu sync.Mutex
	authLimits  = map[string]*authWindow{}
)

func authLimitKey(kind authLimitKind, email, ip string) (string, int) {
	if ip == "" {
		ip = "unknown"
	}
	switch kind {
	case authLimitLogin:
		return "login\x00" + normalizeEmail(email) + "\x00" + ip, authLoginMax
	case authLimitReset:
		return "reset\x00" + normalizeEmail(email) + "\x00" + ip, authResetMax
	default:
		return "register\x00" + ip, authRegisterMax
	}
}

func authEmailKey(kind authLimitKind, email string) (string, int) {
	email = normalizeEmail(email)
	switch kind {
	case authLimitLogin:
		return "login-email\x00" + email, authEmailMax
	case authLimitReset:
		return "reset-email\x00" + email, authEmailMax
	default:
		return "", 0
	}
}

func reserveAuthWindow(key string, max int, now time.Time) bool {
	w := authLimits[key]
	if w == nil || now.Sub(w.start) > authLimitWindow {
		authLimits[key] = &authWindow{count: 1, start: now}
		return true
	}
	if w.count >= max {
		return false
	}
	w.count++
	return true
}

func undoAuthWindow(key string) {
	w := authLimits[key]
	if w == nil {
		return
	}
	if w.count <= 1 {
		delete(authLimits, key)
		return
	}
	w.count--
}

func takeAuthSlot(kind authLimitKind, email, ip string) bool {
	if kind != authLimitRegister && normalizeEmail(email) == "" {
		return false
	}
	now := time.Now()
	authLimitMu.Lock()
	defer authLimitMu.Unlock()
	ipKey, ipMax := authLimitKey(kind, email, ip)
	if !reserveAuthWindow(ipKey, ipMax, now) {
		return false
	}
	if kind == authLimitRegister {
		return true
	}
	emailKey, emailMax := authEmailKey(kind, email)
	if emailKey == "" || reserveAuthWindow(emailKey, emailMax, now) {
		return true
	}
	undoAuthWindow(ipKey)
	return false
}

func clearAuthSlot(kind authLimitKind, email, ip string) {
	if kind == authLimitRegister {
		return
	}
	ipKey, _ := authLimitKey(kind, email, ip)
	emailKey, _ := authEmailKey(kind, email)
	authLimitMu.Lock()
	delete(authLimits, ipKey)
	if emailKey != "" {
		delete(authLimits, emailKey)
	}
	authLimitMu.Unlock()
}

func cleanPasswordResetAttempts() {
	now := time.Now()
	authLimitMu.Lock()
	defer authLimitMu.Unlock()
	for key, w := range authLimits {
		if w == nil || now.Sub(w.start) > authLimitWindow {
			delete(authLimits, key)
		}
	}
}
