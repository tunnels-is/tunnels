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

func putUserWithMeta(t *testing.T, db *gobolt.DB, id, email string, updated, subExp time.Time) {
	t.Helper()
	putUserFields(t, db, id, map[string]any{
		"_id":           id,
		"Email":         email,
		"Password":      "hash-keep",
		"Keep":          "payload",
		"Disabled":      false,
		"Updated":       updated,
		"SubExpiration": subExp,
	})
}

func TestPickCollisionWinner(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	older := now.Add(-48 * time.Hour)
	newer := now.Add(-1 * time.Hour)
	valid := now.Add(24 * time.Hour)
	expired := now.Add(-24 * time.Hour)

	mk := func(id string, updated, sub time.Time) row {
		return row{id: id, updated: updated, subExp: sub}
	}

	t.Run("one valid subscription wins even if older", func(t *testing.T) {
		w, reason := pickCollisionWinner([]row{
			mk("old-paid", older, valid),
			mk("new-expired", newer, expired),
		}, now)
		if w.id != "old-paid" || reason != "valid subscription" {
			t.Fatalf("winner=%s reason=%q", w.id, reason)
		}
	})

	t.Run("both valid picks most recently created", func(t *testing.T) {
		w, reason := pickCollisionWinner([]row{
			mk("old-paid", older, valid),
			mk("new-paid", newer, valid),
		}, now)
		if w.id != "new-paid" || reason != "most recently created" {
			t.Fatalf("winner=%s reason=%q", w.id, reason)
		}
	})

	t.Run("neither valid picks most recently created", func(t *testing.T) {
		w, reason := pickCollisionWinner([]row{
			mk("old-expired", older, expired),
			mk("new-expired", newer, expired),
		}, now)
		if w.id != "new-expired" || reason != "most recently created" {
			t.Fatalf("winner=%s reason=%q", w.id, reason)
		}
	})

	t.Run("equal created breaks tie by larger id", func(t *testing.T) {
		w, reason := pickCollisionWinner([]row{
			mk("aaa", newer, expired),
			mk("zzz", newer, expired),
		}, now)
		if w.id != "zzz" || reason != "most recently created" {
			t.Fatalf("winner=%s reason=%q", w.id, reason)
		}
	})
}

func TestMigrateEmails_CollisionPicksValidSubscription(t *testing.T) {
	db := openTestDB(t)
	now := time.Now()
	oldPaid, newExpired := "user-old-paid", "user-new-expired"
	putUserWithMeta(t, db, oldPaid, "Foo@x.com", now.Add(-48*time.Hour), now.Add(24*time.Hour))
	putUserWithMeta(t, db, newExpired, "foo@x.com", now.Add(-1*time.Hour), now.Add(-24*time.Hour))

	rep, err := migrateEmails(db, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Collisions) != 1 {
		t.Fatalf("collisions=%d", len(rep.Collisions))
	}
	c := rep.Collisions[0]
	if c.Winner != oldPaid || c.Reason != "valid subscription" {
		t.Fatalf("winner=%s reason=%q", c.Winner, c.Reason)
	}

	_ = db.View(func(tx *gobolt.Tx) error {
		idx := tx.Bucket([]byte(emailIndex))
		if got := string(idx.Get([]byte("foo@x.com"))); got != oldPaid {
			t.Fatalf("index -> %q want %s", got, oldPaid)
		}
		if idx.Get([]byte("Foo@x.com")) != nil {
			t.Fatal("old mixed-case index key still present")
		}
		for _, id := range []string{oldPaid, newExpired} {
			email, err := emailFromUserJSON(tx.Bucket([]byte(usersBucket)).Get([]byte(id)))
			if err != nil {
				t.Fatal(err)
			}
			if email != "foo@x.com" {
				t.Fatalf("user %s Email=%q", id, email)
			}
		}
		return nil
	})
}

func TestMigrateEmails_CollisionBothValidPicksNewest(t *testing.T) {
	db := openTestDB(t)
	now := time.Now()
	older, newer := "user-older", "user-newer"
	putUserWithMeta(t, db, older, "Foo@x.com", now.Add(-48*time.Hour), now.Add(24*time.Hour))
	putUserWithMeta(t, db, newer, "foo@x.com", now.Add(-1*time.Hour), now.Add(48*time.Hour))

	rep, err := migrateEmails(db, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Collisions) != 1 || rep.Collisions[0].Winner != newer {
		t.Fatalf("collision=%+v", rep.Collisions)
	}
	if rep.Collisions[0].Reason != "most recently created" {
		t.Fatalf("reason=%q", rep.Collisions[0].Reason)
	}

	_ = db.View(func(tx *gobolt.Tx) error {
		got := string(tx.Bucket([]byte(emailIndex)).Get([]byte("foo@x.com")))
		if got != newer {
			t.Fatalf("index -> %q want %s", got, newer)
		}
		return nil
	})
}

func TestMigrateEmails_CollisionNeitherValidPicksNewest(t *testing.T) {
	db := openTestDB(t)
	now := time.Now()
	older, newer := "user-older", "user-newer"
	putUserWithMeta(t, db, older, "Foo@x.com", now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	putUserWithMeta(t, db, newer, "foo@x.com", now.Add(-1*time.Hour), time.Time{})

	rep, err := migrateEmails(db, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Collisions) != 1 || rep.Collisions[0].Winner != newer {
		t.Fatalf("collision=%+v", rep.Collisions)
	}

	_ = db.View(func(tx *gobolt.Tx) error {
		got := string(tx.Bucket([]byte(emailIndex)).Get([]byte("foo@x.com")))
		if got != newer {
			t.Fatalf("index -> %q want %s", got, newer)
		}
		return nil
	})
}

func TestMigrateEmails_CollisionDryRunDoesNotWrite(t *testing.T) {
	db := openTestDB(t)
	now := time.Now()
	a, b := uuid.NewString(), uuid.NewString()
	putUserWithMeta(t, db, a, "Foo@x.com", now.Add(-48*time.Hour), now.Add(24*time.Hour))
	putUserWithMeta(t, db, b, "foo@x.com", now.Add(-1*time.Hour), now.Add(-24*time.Hour))

	before, err := loadUserBlobs(db)
	if err != nil {
		t.Fatal(err)
	}
	indexBefore := map[string]string{}
	_ = db.View(func(tx *gobolt.Tx) error {
		return tx.Bucket([]byte(emailIndex)).ForEach(func(k, v []byte) error {
			indexBefore[string(k)] = string(v)
			return nil
		})
	})

	rep, err := migrateEmails(db, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Collisions) != 1 {
		t.Fatalf("collisions=%d", len(rep.Collisions))
	}

	after, err := loadUserBlobs(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("user count changed")
	}
	for id, raw := range before {
		if string(raw) != string(after[id]) {
			t.Fatalf("dry-run mutated user %s", id)
		}
	}
	indexAfter := map[string]string{}
	_ = db.View(func(tx *gobolt.Tx) error {
		return tx.Bucket([]byte(emailIndex)).ForEach(func(k, v []byte) error {
			indexAfter[string(k)] = string(v)
			return nil
		})
	})
	if len(indexBefore) != len(indexAfter) {
		t.Fatalf("dry-run changed index size: before=%d after=%d", len(indexBefore), len(indexAfter))
	}
	for k, v := range indexBefore {
		if indexAfter[k] != v {
			t.Fatalf("dry-run mutated index %q", k)
		}
	}
}

func collisionPair(t *testing.T) (oldDB, db *gobolt.DB, winner, loser string) {
	t.Helper()
	oldDB = openNamedDB(t, "old.db")
	db = openNamedDB(t, "new.db")
	now := time.Now()
	winner, loser = uuid.NewString(), uuid.NewString()
	putUserWithMeta(t, oldDB, winner, "Foo@x.com", now.Add(-48*time.Hour), now.Add(24*time.Hour))
	putUserWithMeta(t, oldDB, loser, "foo@x.com", now.Add(-1*time.Hour), now.Add(-24*time.Hour))
	cloneUsers(t, oldDB, db)
	if _, err := migrateEmails(db, true); err != nil {
		t.Fatal(err)
	}
	return oldDB, db, winner, loser
}

func tamperUserField(t *testing.T, db *gobolt.DB, id, field, value string) {
	t.Helper()
	if err := db.Update(func(tx *gobolt.Tx) error {
		raw := tx.Bucket([]byte(usersBucket)).Get([]byte(id))
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}
		m[field], _ = json.Marshal(value)
		next, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return tx.Bucket([]byte(usersBucket)).Put([]byte(id), next)
	}); err != nil {
		t.Fatal(err)
	}
}

func TestVerify_CollisionKeepsAllUsersAndIndexesWinner(t *testing.T) {
	oldDB, db, winner, _ := collisionPair(t)

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

	_ = db.View(func(tx *gobolt.Tx) error {
		got := string(tx.Bucket([]byte(emailIndex)).Get([]byte("foo@x.com")))
		if got != winner {
			t.Fatalf("index -> %q want %s", got, winner)
		}
		return nil
	})
}

func TestVerify_CollisionIgnoresLoserChanges(t *testing.T) {
	oldDB, db, _, loser := collisionPair(t)
	tamperUserField(t, db, loser, "Password", "tampered-loser")

	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(mm) != 0 {
		t.Fatalf("loser changes should be ignored, got %s", strings.Join(mm, "; "))
	}
}

func TestVerify_CollisionIgnoresMissingLoser(t *testing.T) {
	oldDB, db, _, loser := collisionPair(t)
	if err := db.Update(func(tx *gobolt.Tx) error {
		return tx.Bucket([]byte(usersBucket)).Delete([]byte(loser))
	}); err != nil {
		t.Fatal(err)
	}

	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(mm) != 0 {
		t.Fatalf("missing loser should be ignored, got %s", strings.Join(mm, "; "))
	}
}

func TestVerify_CollisionDetectsWinnerChanges(t *testing.T) {
	oldDB, db, winner, _ := collisionPair(t)
	tamperUserField(t, db, winner, "Password", "tampered-winner")

	mm, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range mm {
		if strings.Contains(m, "Password") && strings.Contains(m, winner) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected winner password mismatch, got %v", mm)
	}
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
