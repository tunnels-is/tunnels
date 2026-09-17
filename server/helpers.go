package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/google/uuid"
)

func BasicRecover() {
	if r := recover(); r != nil {
		ERR(r, string(debug.Stack()))
	}
}

func redactKey(k string) string {
	const show = 5
	if len(k) <= show {
		return "…"
	}
	return k[:show] + "…"
}

var totpAlphabet = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")

func generateCode() string {
	defer BasicRecover()
	b := make([]rune, 16)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(totpAlphabet))))
		if err != nil {
			panic(err)
		}
		b[i] = totpAlphabet[n.Int64()]
	}

	return strings.ToUpper(string(b))
}

func decodeBody(r *http.Request, target any) (err error) {
	r.Body = http.MaxBytesReader(nil, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	err = dec.Decode(target)
	if err != nil {
		return fmt.Errorf("Invalid request body: %s", err)
	}
	return nil
}

func hasSharedOrNoGroup(actorGroups []uuid.UUID, serverGroups []uuid.UUID) (yes bool) {
	if len(serverGroups) == 0 {
		return true
	}
	for _, g := range actorGroups {
		for _, dg := range serverGroups {
			if subtle.ConstantTimeCompare(g[:], dg[:]) == 1 {
				return true
			}
		}
	}

	return false
}
