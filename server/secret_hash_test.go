package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func TestSecretMatches_HashAndLegacy(t *testing.T) {
	raw := "plain-secret"
	hashed := hashSecret(raw)
	if !storedSecretIsHash(hashed) || storedSecretIsHash(raw) {
		t.Fatalf("hash marker: %q", hashed)
	}
	if !secretMatches(hashed, raw) || secretMatches(hashed, "other") {
		t.Fatal("hash compare")
	}
	if !secretMatches(raw, raw) || secretMatches(raw, "other") || secretMatches("", raw) {
		t.Fatal("legacy compare")
	}
	if storeSecret(hashed) != hashed || storeSecret(raw) != hashed || storeSecret("") != "" {
		t.Fatal("storeSecret")
	}
}

func TestFindServerByAPIKey_UpgradesLegacy(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{
		ID:              uuid.New(),
		Tag:             "legacy",
		APIKey:          "legacy-key",
		WireGuardPubKey: "pinned-key",
	}
	if err := db.Update(func(tx *gobolt.Tx) error {
		data, err := bboltMarshal(s)
		if err != nil {
			return err
		}
		if err := tx.Bucket([]byte(bucketServers)).Put([]byte(s.ID.String()), data); err != nil {
			return err
		}
		return tx.Bucket([]byte(bucketServersAPIKeyIndex)).Put([]byte(s.APIKey), []byte(s.ID.String()))
	}); err != nil {
		t.Fatal(err)
	}

	found, err := findServerByAPIKey("legacy-key")
	if err != nil || found == nil || found.ID != s.ID {
		t.Fatalf("lookup: %v %#v", err, found)
	}
	if found.WireGuardPubKey != "pinned-key" {
		t.Fatalf("upgrade cleared the pin: %q", found.WireGuardPubKey)
	}
	if !storedSecretIsHash(found.APIKey) || !secretMatches(found.APIKey, "legacy-key") {
		t.Fatalf("returned key %q", found.APIKey)
	}

	stored, _ := findServerByID(s.ID)
	if stored == nil || stored.APIKey != found.APIKey || stored.WireGuardPubKey != "pinned-key" {
		t.Fatalf("stored server %+v", stored)
	}
	if err := db.View(func(tx *gobolt.Tx) error {
		idx := tx.Bucket([]byte(bucketServersAPIKeyIndex))
		if idx.Get([]byte("legacy-key")) != nil {
			t.Fatal("plaintext index entry remains")
		}
		if string(idx.Get([]byte(found.APIKey))) != s.ID.String() {
			t.Fatal("hash index missing")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	again, err := findServerByAPIKey("legacy-key")
	if err != nil || again == nil || again.APIKey != found.APIKey {
		t.Fatalf("second lookup: %v %#v", err, again)
	}
}

func TestUpdateServer_SameKeyKeepsPin(t *testing.T) {
	setupTestDB(t)
	s := &types.Server{ID: uuid.New(), APIKey: "same-key", WireGuardPubKey: "pinned-key"}
	if err := createServer(s); err != nil {
		t.Fatal(err)
	}
	if s.APIKey != "same-key" {
		t.Fatalf("createServer changed the caller's key: %q", s.APIKey)
	}
	updated, err := updateServer(s)
	if err != nil {
		t.Fatal(err)
	}
	if updated.WireGuardPubKey != "pinned-key" {
		t.Fatal("echoing the plaintext key must not unpin")
	}
	stored, _ := findServerByID(s.ID)
	if stored == nil || !secretMatches(stored.APIKey, "same-key") || !storedSecretIsHash(stored.APIKey) {
		t.Fatalf("stored key %q", stored.APIKey)
	}

	s.APIKey = stored.APIKey
	updated, err = updateServer(s)
	if err != nil {
		t.Fatal(err)
	}
	if updated.WireGuardPubKey != "pinned-key" || updated.APIKey != stored.APIKey {
		t.Fatalf("echoing the hash rotated the key: %+v", updated)
	}
}

func TestAuthenticate_UpgradesLegacyTokenAndAPIKey(t *testing.T) {
	setupTestDB(t)
	token := &DeviceToken{DT: "legacy-dt", N: "phone"}
	u := &User{
		ID:          uuid.New(),
		Email:       "legacy@example.com",
		APIKey:      "legacy-api",
		DeviceToken: token,
		Tokens:      []*DeviceToken{token},
	}
	if err := db.Update(func(tx *gobolt.Tx) error {
		data, err := bboltMarshal(u)
		if err != nil {
			return err
		}
		if err := tx.Bucket([]byte(bucketUsers)).Put([]byte(u.ID.String()), data); err != nil {
			return err
		}
		if err := tx.Bucket([]byte(bucketUsersEmailIndex)).Put([]byte(u.Email), []byte(u.ID.String())); err != nil {
			return err
		}
		return tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Put([]byte(u.APIKey), []byte(u.ID.String()))
	}); err != nil {
		t.Fatal(err)
	}

	got, err := authenticateUserFromEmailOrIDAndToken(u.Email, uuid.Nil, "legacy-dt")
	if err != nil || got == nil {
		t.Fatalf("token auth: %v", err)
	}
	stored, _ := findUserByID(u.ID)
	if stored == nil || !storedSecretIsHash(stored.Tokens[0].DT) || !secretMatches(stored.Tokens[0].DT, "legacy-dt") {
		t.Fatalf("token not upgraded: %+v", stored.Tokens)
	}
	if storedSecretIsHash(stored.APIKey) {
		t.Fatal("API key upgraded before it was used")
	}

	if _, err := authenticateUserFromEmailOrIDAndToken(u.Email, uuid.Nil, "legacy-api"); err != nil {
		t.Fatal(err)
	}
	stored, _ = findUserByID(u.ID)
	if stored == nil || !storedSecretIsHash(stored.APIKey) || !secretMatches(stored.APIKey, "legacy-api") {
		t.Fatalf("API key not upgraded: %q", stored.APIKey)
	}
	if err := db.View(func(tx *gobolt.Tx) error {
		idx := tx.Bucket([]byte(bucketUsersAPIKeyIndex))
		if idx.Get([]byte("legacy-api")) != nil {
			t.Fatal("plaintext API key index remains")
		}
		if string(idx.Get([]byte(stored.APIKey))) != u.ID.String() {
			t.Fatal("hash API key index missing")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authenticateUserFromEmailOrIDAndToken(u.Email, uuid.Nil, "legacy-api"); err != nil {
		t.Fatal(err)
	}
}

func TestIssueLoginDeviceToken_StoresHashReturnsPlaintext(t *testing.T) {
	setupTestDB(t)
	u := &User{ID: uuid.New(), Email: "hash-login@example.com"}
	if err := createUser(u); err != nil {
		t.Fatal(err)
	}
	if err := issueLoginDeviceToken(u, &loginRequest{DeviceName: "phone"}); err != nil {
		t.Fatal(err)
	}
	if u.DeviceToken == nil || storedSecretIsHash(u.DeviceToken.DT) {
		t.Fatalf("issued token %#v", u.DeviceToken)
	}
	stored, _ := findUserByID(u.ID)
	if stored == nil || len(stored.Tokens) != 1 || stored.Tokens[0].DT == u.DeviceToken.DT || !secretMatches(stored.Tokens[0].DT, u.DeviceToken.DT) {
		t.Fatalf("stored %#v issued %q", stored.Tokens, u.DeviceToken.DT)
	}

	raw := u.DeviceToken.DT
	if _, err := authenticateUserFromEmailOrIDAndToken(u.Email, uuid.Nil, raw); err != nil {
		t.Fatal(err)
	}
	if err := issueLoginDeviceToken(u, &loginRequest{DeviceToken: raw, DeviceName: "phone"}); err != nil {
		t.Fatal(err)
	}
	if u.DeviceToken.DT == raw {
		t.Fatal("login did not rotate the token")
	}
	if _, err := authenticateUserFromEmailOrIDAndToken(u.Email, uuid.Nil, raw); err == nil {
		t.Fatal("rotated token still authenticated")
	}
	if _, err := authenticateUserFromEmailOrIDAndToken(u.Email, uuid.Nil, u.DeviceToken.DT); err != nil {
		t.Fatal(err)
	}
}

func TestOpenDB_HashesLegacySecrets(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/legacy.db"
	if err := openDB(path); err != nil {
		t.Fatal(err)
	}
	u := &User{
		ID:     uuid.New(),
		Email:  "bf-legacy@example.com",
		APIKey: "bf-legacy-key",
		Tokens: []*DeviceToken{{DT: "bf-legacy-dt", N: "old"}},
	}
	s := &types.Server{ID: uuid.New(), Tag: "bf", APIKey: "bf-legacy-wg", WireGuardPubKey: "pin"}
	if err := db.Update(func(tx *gobolt.Tx) error {
		ub, err := bboltMarshal(u)
		if err != nil {
			return err
		}
		if err := tx.Bucket([]byte(bucketUsers)).Put([]byte(u.ID.String()), ub); err != nil {
			return err
		}
		if err := tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Put([]byte(u.APIKey), []byte(u.ID.String())); err != nil {
			return err
		}
		sb, err := bboltMarshal(s)
		if err != nil {
			return err
		}
		if err := tx.Bucket([]byte(bucketServers)).Put([]byte(s.ID.String()), sb); err != nil {
			return err
		}
		return tx.Bucket([]byte(bucketServersAPIKeyIndex)).Put([]byte(s.APIKey), []byte(s.ID.String()))
	}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	if err := openDB(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	got, _ := findUserByID(u.ID)
	if got == nil || !storedSecretIsHash(got.APIKey) || !secretMatches(got.APIKey, "bf-legacy-key") {
		t.Fatalf("user key %+v", got)
	}
	if len(got.Tokens) != 1 || !storedSecretIsHash(got.Tokens[0].DT) || !secretMatches(got.Tokens[0].DT, "bf-legacy-dt") {
		t.Fatalf("user token %+v", got.Tokens)
	}
	if _, err := authenticateUserFromEmailOrIDAndToken(u.Email, uuid.Nil, "bf-legacy-dt"); err != nil {
		t.Fatal(err)
	}
	srv, err := findServerByAPIKey("bf-legacy-wg")
	if err != nil || srv == nil || srv.WireGuardPubKey != "pin" {
		t.Fatalf("server lookup: %v %#v", err, srv)
	}
	if err := db.View(func(tx *gobolt.Tx) error {
		if tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Get([]byte("bf-legacy-key")) != nil {
			t.Fatal("user plaintext index remains")
		}
		if tx.Bucket([]byte(bucketServersAPIKeyIndex)).Get([]byte("bf-legacy-wg")) != nil {
			t.Fatal("server plaintext index remains")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
