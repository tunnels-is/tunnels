package main

import (
	"sync"
	"time"
)

const (
	authLoginMax            = 10
	authResetMax            = 5
	authRegisterMax         = 50
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

func takeAuthSlot(kind authLimitKind, email, ip string) bool {
	if kind != authLimitRegister && normalizeEmail(email) == "" {
		return false
	}
	key, max := authLimitKey(kind, email, ip)
	now := time.Now()
	authLimitMu.Lock()
	defer authLimitMu.Unlock()
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

func clearAuthSlot(kind authLimitKind, email, ip string) {
	if kind == authLimitRegister {
		return
	}
	key, _ := authLimitKey(kind, email, ip)
	authLimitMu.Lock()
	delete(authLimits, key)
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
