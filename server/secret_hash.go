package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

const secretHashPrefix = "h1:"

func hashSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return secretHashPrefix + hex.EncodeToString(sum[:])
}

func storedSecretIsHash(stored string) bool {
	hexPart, ok := strings.CutPrefix(stored, secretHashPrefix)
	if !ok || len(hexPart) != sha256.Size*2 {
		return false
	}
	for _, c := range hexPart {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}

func storeSecret(raw string) string {
	if raw == "" || storedSecretIsHash(raw) {
		return raw
	}
	return hashSecret(raw)
}

func secretMatches(stored, presented string) bool {
	if stored == "" || presented == "" {
		return false
	}
	if storedSecretIsHash(stored) {
		return subtle.ConstantTimeCompare([]byte(stored), []byte(hashSecret(presented))) == 1
	}
	if len(stored) != len(presented) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(presented)) == 1
}

func apiKeyRotated(oldStored, incoming string) bool {
	if incoming == "" {
		return oldStored != ""
	}
	if incoming == oldStored {
		return false
	}
	return !secretMatches(oldStored, incoming)
}

func copyDeviceToken(t *DeviceToken) *DeviceToken {
	if t == nil {
		return nil
	}
	c := *t
	return &c
}

func hashUserSecrets(user *User) (oldAPIKey string, changed bool) {
	if user == nil {
		return "", false
	}
	if user.APIKey != "" && !storedSecretIsHash(user.APIKey) {
		oldAPIKey = user.APIKey
		user.APIKey = hashSecret(user.APIKey)
		changed = true
	}
	for _, t := range user.Tokens {
		if t == nil || t.DT == "" || storedSecretIsHash(t.DT) {
			continue
		}
		t.DT = hashSecret(t.DT)
		changed = true
	}
	if user.DeviceToken != nil && user.DeviceToken.DT != "" && !storedSecretIsHash(user.DeviceToken.DT) {
		user.DeviceToken.DT = hashSecret(user.DeviceToken.DT)
		changed = true
	}
	return oldAPIKey, changed
}

func userRecordForStore(user *User) *User {
	if user == nil {
		return nil
	}
	cp := *user
	if user.Tokens != nil {
		cp.Tokens = make([]*DeviceToken, len(user.Tokens))
		for i, t := range user.Tokens {
			cp.Tokens[i] = copyDeviceToken(t)
		}
	}
	cp.DeviceToken = copyDeviceToken(user.DeviceToken)
	hashUserSecrets(&cp)
	return &cp
}
