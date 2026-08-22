package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	gobolt "go.etcd.io/bbolt"
)

func openTestDB(t *testing.T) *gobolt.DB {
	t.Helper()
	return openNamedDB(t, "tunnels.db")
}

func openNamedDB(t *testing.T, name string) *gobolt.DB {
	t.Helper()
	db, err := gobolt.Open(filepath.Join(t.TempDir(), name), 0o600, &gobolt.Options{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Update(func(tx *gobolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(usersBucket)); err != nil {
			return err
		}
		_, err := tx.CreateBucketIfNotExists([]byte(emailIndex))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return db
}

func putUser(t *testing.T, db *gobolt.DB, id, email string) {
	t.Helper()
	putUserFields(t, db, id, map[string]any{
		"_id":      id,
		"Email":    email,
		"Password": "hash-keep",
		"Keep":     "payload",
		"Disabled": false,
	})
}

func putUserFields(t *testing.T, db *gobolt.DB, id string, rec map[string]any) {
	t.Helper()
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	email, _ := rec["Email"].(string)
	if err := db.Update(func(tx *gobolt.Tx) error {
		if err := tx.Bucket([]byte(usersBucket)).Put([]byte(id), raw); err != nil {
			return err
		}
		if email != "" {
			return tx.Bucket([]byte(emailIndex)).Put([]byte(email), []byte(id))
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func cloneUsers(t *testing.T, src, dst *gobolt.DB) {
	t.Helper()
	blobs, err := loadUserBlobs(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(usersBucket))
		idx := tx.Bucket([]byte(emailIndex))
		for id, raw := range blobs {
			if err := b.Put([]byte(id), raw); err != nil {
				return err
			}
			email, err := emailFromUserJSON(raw)
			if err != nil {
				return err
			}
			if email != "" {
				if err := idx.Put([]byte(email), []byte(id)); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateEmails_DryRunDoesNotWrite(t *testing.T) {
	db := openTestDB(t)
	id := uuid.NewString()
	putUser(t, db, id, "Jane@Company.COM")

	rep, err := migrateEmails(db, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.WouldUpdate != 1 {
		t.Fatalf("WouldUpdate=%d", rep.WouldUpdate)
	}

	_ = db.View(func(tx *gobolt.Tx) error {
		email, err := emailFromUserJSON(tx.Bucket([]byte(usersBucket)).Get([]byte(id)))
		if err != nil {
			t.Fatal(err)
		}
		if email != "Jane@Company.COM" {
			t.Fatalf("dry-run mutated Email: %q", email)
		}
		if tx.Bucket([]byte(emailIndex)).Get([]byte("Jane@Company.COM")) == nil {
			t.Fatal("dry-run dropped old index key")
		}
		if tx.Bucket([]byte(emailIndex)).Get([]byte("jane@company.com")) != nil {
			t.Fatal("dry-run wrote normalized index")
		}
		return nil
	})
}

func TestMigrateEmails_ApplyNormalizesAndRebuildsIndex(t *testing.T) {
	db := openTestDB(t)
	id := uuid.NewString()
	putUser(t, db, id, "Jane@Company.COM")

	rep, err := migrateEmails(db, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.WouldUpdate != 1 {
		t.Fatalf("WouldUpdate=%d", rep.WouldUpdate)
	}

	_ = db.View(func(tx *gobolt.Tx) error {
		raw := tx.Bucket([]byte(usersBucket)).Get([]byte(id))
		email, err := emailFromUserJSON(raw)
		if err != nil {
			t.Fatal(err)
		}
		if email != "jane@company.com" {
			t.Fatalf("stored Email=%q", email)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if m["Keep"] != "payload" || m["Password"] != "hash-keep" {
			t.Fatalf("lost extra field: %#v", m)
		}
		idx := tx.Bucket([]byte(emailIndex))
		if got := string(idx.Get([]byte("jane@company.com"))); got != id {
			t.Fatalf("index jane@company.com -> %q", got)
		}
		if idx.Get([]byte("Jane@Company.COM")) != nil {
			t.Fatal("old mixed-case index key still present")
		}
		return nil
	})
}

func TestMigrateEmails_CollisionAborts(t *testing.T) {
	db := openTestDB(t)
	a, b := uuid.NewString(), uuid.NewString()
	putUser(t, db, a, "Foo@x.com")
	putUser(t, db, b, "foo@x.com")

	rep, err := migrateEmails(db, true)
	if err == nil {
		t.Fatal("expected collision error")
	}
	if len(rep.Collisions) != 1 {
		t.Fatalf("collisions=%d", len(rep.Collisions))
	}

	_ = db.View(func(tx *gobolt.Tx) error {
		email, _ := emailFromUserJSON(tx.Bucket([]byte(usersBucket)).Get([]byte(a)))
		if email != "Foo@x.com" {
			t.Fatalf("collision apply mutated user: %q", email)
		}
		return nil
	})
}

func TestVerifyUsersAgainstOld_OK(t *testing.T) {
	oldDB := openNamedDB(t, "old.db")
	db := openNamedDB(t, "new.db")
	id := uuid.NewString()
	putUser(t, oldDB, id, "Jane@Company.COM")
	cloneUsers(t, oldDB, db)

	if _, err := migrateEmails(db, true); err != nil {
		t.Fatal(err)
	}
	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(mm) != 0 {
		t.Fatalf("unexpected mismatches: %s", strings.Join(mm, "; "))
	}
}

func TestVerifyUsersAgainstOld_DetectsChangedField(t *testing.T) {
	oldDB := openNamedDB(t, "old.db")
	db := openNamedDB(t, "new.db")
	id := uuid.NewString()
	putUser(t, oldDB, id, "Jane@Company.COM")
	cloneUsers(t, oldDB, db)
	if _, err := migrateEmails(db, true); err != nil {
		t.Fatal(err)
	}

	if err := db.Update(func(tx *gobolt.Tx) error {
		raw := tx.Bucket([]byte(usersBucket)).Get([]byte(id))
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		m["Password"], _ = json.Marshal("tampered")
		next, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return tx.Bucket([]byte(usersBucket)).Put([]byte(id), next)
	}); err != nil {
		t.Fatal(err)
	}

	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(mm) == 0 {
		t.Fatal("expected password mismatch")
	}
	found := false
	for _, m := range mm {
		if strings.Contains(m, "Password") {
			found = true
		}
	}
	if !found {
		t.Fatalf("mismatches=%v", mm)
	}
}

func TestVerifyUsersAgainstOld_DetectsMissingUser(t *testing.T) {
	oldDB := openNamedDB(t, "old.db")
	db := openNamedDB(t, "new.db")
	id := uuid.NewString()
	putUser(t, oldDB, id, "a@b.com")
	putUser(t, db, uuid.NewString(), "other@b.com")

	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(mm) == 0 {
		t.Fatal("expected missing/extra user mismatches")
	}
}

func productionUser(id, email string) map[string]any {
	return map[string]any{
		"_id":              id,
		"Email":            email,
		"Updated":          "2024-06-01T12:00:00Z",
		"Disabled":         false,
		"APIKey":           "ak-keep-me",
		"Password":         "$2a$13$abcdefghijklmnopqrstuv",
		"ConfirmCode":      "",
		"RecoveryCodes":    []byte{0x01, 0x02, 0xff},
		"TwoFactorCode":    []byte{0xde, 0xad},
		"TwoFactorEnabled": true,
		"Tokens":           []map[string]any{{"DT": "tok-1", "N": "laptop", "C": "2024-01-02T03:04:05Z"}},
		"IsAdmin":          false,
		"Groups":           []string{"11111111-1111-1111-1111-111111111111"},
		"Trial":            false,
		"SubExpiration":    "2025-01-01T00:00:00Z",
		"Key":              map[string]any{"Created": "2024-01-01T00:00:00Z", "Months": 12, "Key": "AAAA-BBBB-CCCC"},
		"DeviceToken":      map[string]any{"DT": "tok-1", "N": "laptop", "C": "2024-01-02T03:04:05Z"},
	}
}

func TestVerify_ProductionShapedUserAllFields(t *testing.T) {
	oldDB := openNamedDB(t, "old.db")
	db := openNamedDB(t, "new.db")
	id := uuid.NewString()
	putUserFields(t, oldDB, id, productionUser(id, "Admin.User@Example.COM"))
	cloneUsers(t, oldDB, db)

	if _, err := migrateEmails(db, true); err != nil {
		t.Fatal(err)
	}
	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	idxMM, err := verifyEmailIndex(db)
	if err != nil {
		t.Fatal(err)
	}
	mm = append(mm, idxMM...)
	if len(mm) != 0 {
		t.Fatalf("mismatches: %s", strings.Join(mm, "; "))
	}

	raw, err := loadUserBlobs(db)
	if err != nil {
		t.Fatal(err)
	}
	email, _ := emailFromUserJSON(raw[id])
	if email != "admin.user@example.com" {
		t.Fatalf("Email=%q", email)
	}
}

func TestVerify_IdempotentSecondApply(t *testing.T) {
	oldDB := openNamedDB(t, "old.db")
	db := openNamedDB(t, "new.db")
	id := uuid.NewString()
	putUserFields(t, oldDB, id, productionUser(id, "Jane@Company.COM"))
	cloneUsers(t, oldDB, db)

	if _, err := migrateEmails(db, true); err != nil {
		t.Fatal(err)
	}
	rep, err := migrateEmails(db, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.WouldUpdate != 0 || rep.AlreadyNormalized != 1 {
		t.Fatalf("second apply: %+v", *rep)
	}
	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(mm) != 0 {
		t.Fatalf("second apply broke verify: %s", strings.Join(mm, "; "))
	}
}

func TestMigrate_WhitespaceEmailIsAnUpdate(t *testing.T) {
	oldDB := openNamedDB(t, "old.db")
	db := openNamedDB(t, "new.db")
	id := uuid.NewString()
	putUser(t, oldDB, id, "  Jane@Company.COM\t")
	cloneUsers(t, oldDB, db)

	rep, err := migrateEmails(db, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.WouldUpdate != 1 {
		t.Fatalf("whitespace email should count as update, got WouldUpdate=%d Empty=%d", rep.WouldUpdate, rep.Empty)
	}
	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(mm) != 0 {
		t.Fatalf("mismatches: %s", strings.Join(mm, "; "))
	}
	blobs, _ := loadUserBlobs(db)
	email, _ := emailFromUserJSON(blobs[id])
	if email != "jane@company.com" {
		t.Fatalf("Email=%q", email)
	}
}

func TestMigrate_MixedAlreadyNormalizedAndMixedCase(t *testing.T) {
	oldDB := openNamedDB(t, "old.db")
	db := openNamedDB(t, "new.db")
	a, b := uuid.NewString(), uuid.NewString()
	putUser(t, oldDB, a, "already@example.com")
	putUser(t, oldDB, b, "Needs.Case@Example.COM")
	cloneUsers(t, oldDB, db)

	rep, err := migrateEmails(db, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.AlreadyNormalized != 1 || rep.WouldUpdate != 1 {
		t.Fatalf("report=%+v", *rep)
	}
	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	idxMM, err := verifyEmailIndex(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(append(mm, idxMM...)) != 0 {
		t.Fatalf("mismatches: %v %v", mm, idxMM)
	}
}

func TestVerify_DetectsUnnormalizedEmail(t *testing.T) {
	oldDB := openNamedDB(t, "old.db")
	db := openNamedDB(t, "new.db")
	id := uuid.NewString()
	putUser(t, oldDB, id, "Jane@Company.COM")
	cloneUsers(t, oldDB, db)

	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range mm {
		if strings.Contains(m, "Email") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Email mismatch before migrate, got %v", mm)
	}
}

func TestVerify_DetectsExtraField(t *testing.T) {
	oldDB := openNamedDB(t, "old.db")
	db := openNamedDB(t, "new.db")
	id := uuid.NewString()
	putUser(t, oldDB, id, "a@b.com")
	cloneUsers(t, oldDB, db)
	if _, err := migrateEmails(db, true); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *gobolt.Tx) error {
		raw := tx.Bucket([]byte(usersBucket)).Get([]byte(id))
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		m["Injected"], _ = json.Marshal("nope")
		next, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return tx.Bucket([]byte(usersBucket)).Put([]byte(id), next)
	}); err != nil {
		t.Fatal(err)
	}
	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range mm {
		if strings.Contains(m, "Injected") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected extra field mismatch, got %v", mm)
	}
}

func TestVerifyEmailIndex_DetectsStaleKey(t *testing.T) {
	db := openNamedDB(t, "new.db")
	id := uuid.NewString()
	putUser(t, db, id, "Jane@Company.COM")
	if _, err := migrateEmails(db, true); err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx *gobolt.Tx) error {
		return tx.Bucket([]byte(emailIndex)).Put([]byte("Jane@Company.COM"), []byte(id))
	}); err != nil {
		t.Fatal(err)
	}
	mm, err := verifyEmailIndex(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(mm) == 0 {
		t.Fatal("expected stale mixed-case index key")
	}
}

func TestSamePath(t *testing.T) {
	if !samePath("tunnels.db", "tunnels.db") {
		t.Fatal("same relative path")
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "x.db")
	if !samePath(a, filepath.Join(dir, ".", "x.db")) {
		t.Fatal("abs vs . should match")
	}
	if samePath(a, filepath.Join(dir, "y.db")) {
		t.Fatal("different files")
	}
}
