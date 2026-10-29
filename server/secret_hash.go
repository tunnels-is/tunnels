package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

// secretHashPrefix marks a SHA-256 hex digest of an API key or device token.
// The prefix keeps a hash distinct from a legacy plaintext UUID or test key.
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

// storeSecret returns the value to persist. An empty string and an existing
// hash are kept. Any other string is hashed.
func storeSecret(raw string) string {
	if raw == "" || storedSecretIsHash(raw) {
		return raw
	}
	return hashSecret(raw)
}

// secretMatches reports whether presented is the secret stored in stored.
// stored is either h1: plus the SHA-256 hex digest, or a legacy plaintext value.
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

// apiKeyRotated reports whether incoming replaces oldStored.
// Echoing the stored value, or sending the plaintext of that value, is not a
// rotation. An empty incoming value clears a key that was set.
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

// hashUserSecrets rewrites plaintext API keys and device tokens on user.
// It returns the previous API key when that field was hashed.
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

// userRecordForStore copies user and hashes secrets on the copy.
// The caller's API key and device tokens stay plaintext so a response can
// still show a key that was just created.
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
