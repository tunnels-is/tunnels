package main

import "strings"

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isReservedAccountEmail(email string) bool {
	return normalizeEmail(email) == "admin"
}
