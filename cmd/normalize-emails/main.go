// One-shot migration: lowercase/trim User.Email and rebuild users_by_email.
// This does not run when the controller opens the database.
//
// Copy tunnels.db first, then:
//
//	go run ./cmd/normalize-emails -db tunnels.db -db_old tunnels.db.old
//	go run ./cmd/normalize-emails -db tunnels.db -db_old tunnels.db.old -apply
//
// After -apply, every user in -db is checked against -db_old: Email must be
// the normalized form of the old Email, and every other stored field must
// match exactly.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gobolt "go.etcd.io/bbolt"
)

const (
	usersBucket = "users"
	emailIndex  = "users_by_email"
)

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

type collision struct {
	Email string
	IDs   []string
}

type report struct {
	Users             int
	AlreadyNormalized int
	WouldUpdate       int
	Empty             int
	Collisions        []collision
	VerifyMismatches  []string
}

func main() {
	dbPath := flag.String("db", "", "database to normalize")
	oldPath := flag.String("db_old", "", "unmodified copy of the database, used to verify users after normalization")
	apply := flag.Bool("apply", false, "write changes (default is dry-run)")
	flag.Parse()

	if strings.TrimSpace(*dbPath) == "" || strings.TrimSpace(*oldPath) == "" {
		fmt.Fprintf(os.Stderr, "usage: normalize-emails -db tunnels.db -db_old tunnels.db.old [-apply]\n")
		os.Exit(2)
	}
	if samePath(*dbPath, *oldPath) {
		fmt.Fprintf(os.Stderr, "-db and -db_old must be different files\n")
		os.Exit(2)
	}

	oldDB, err := gobolt.Open(*oldPath, 0o600, &gobolt.Options{Timeout: 2 * time.Second, ReadOnly: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open -db_old %s: %v\n", *oldPath, err)
		os.Exit(1)
	}
	defer oldDB.Close()

	db, err := gobolt.Open(*dbPath, 0o600, &gobolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open -db %s: %v\n", *dbPath, err)
		os.Exit(1)
	}
	defer db.Close()

	rep, err := migrateEmails(db, *apply)
	printReport(rep, *apply)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if !*apply {
		fmt.Println("dry-run only; re-run with -apply to write, then verify against -db_old")
		return
	}

	mismatches, err := verifyUsersAgainstOld(db, oldDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify: %v\n", err)
		os.Exit(1)
	}
	idxMismatches, err := verifyEmailIndex(db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verify index: %v\n", err)
		os.Exit(1)
	}
	mismatches = append(mismatches, idxMismatches...)
	rep.VerifyMismatches = mismatches
	if len(mismatches) > 0 {
		fmt.Fprintf(os.Stderr, "verification FAILED (%d mismatch(es)):\n", len(mismatches))
		for i, m := range mismatches {
			if i >= 50 {
				fmt.Fprintf(os.Stderr, "  … %d more\n", len(mismatches)-50)
				break
			}
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		os.Exit(1)
	}
	fmt.Println("verification OK: every user matches -db_old (Email normalized, all other fields unchanged); email index rebuilt")
}

func samePath(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return a == b
	}
	return aa == bb
}

func printReport(rep *report, apply bool) {
	if rep == nil {
		return
	}
	action := "would update"
	if apply {
		action = "updated"
	}
	fmt.Printf("users=%d already-normalized=%d %s=%d empty-email=%d collisions=%d\n",
		rep.Users, rep.AlreadyNormalized, action, rep.WouldUpdate, rep.Empty, len(rep.Collisions))
	for _, c := range rep.Collisions {
		fmt.Printf("  COLLISION  email=%q  ids=%s\n", c.Email, strings.Join(c.IDs, ","))
	}
}

func migrateEmails(db *gobolt.DB, apply bool) (*report, error) {
	type row struct {
		id    string
		raw   []byte
		email string
		norm  string
	}

	var rows []row
	byNorm := make(map[string][]string)

	err := db.View(func(tx *gobolt.Tx) error {
		users := tx.Bucket([]byte(usersBucket))
		if users == nil {
			return fmt.Errorf("bucket %q missing", usersBucket)
		}
		return users.ForEach(func(k, v []byte) error {
			email, err := emailFromUserJSON(v)
			if err != nil {
				return fmt.Errorf("user %s: %w", k, err)
			}
			norm := normalizeEmail(email)
			r := row{id: string(k), raw: append([]byte(nil), v...), email: email, norm: norm}
			rows = append(rows, r)
			if norm != "" {
				byNorm[norm] = append(byNorm[norm], r.id)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	rep := &report{Users: len(rows)}
	for _, r := range rows {
		if r.norm == "" {
			rep.Empty++
		}
		if r.email == r.norm {
			if r.norm != "" {
				rep.AlreadyNormalized++
			}
			continue
		}
		rep.WouldUpdate++
	}
	for email, ids := range byNorm {
		if len(ids) > 1 {
			rep.Collisions = append(rep.Collisions, collision{Email: email, IDs: ids})
		}
	}
	if len(rep.Collisions) > 0 {
		return rep, errors.New("refusing to migrate: two or more users share the same email after normalization")
	}
	if !apply {
		return rep, nil
	}

	err = db.Update(func(tx *gobolt.Tx) error {
		users := tx.Bucket([]byte(usersBucket))
		if users == nil {
			return fmt.Errorf("bucket %q missing", usersBucket)
		}
		for _, r := range rows {
			if r.email == r.norm {
				continue
			}
			next, err := setUserEmailJSON(r.raw, r.norm)
			if err != nil {
				return fmt.Errorf("user %s: %w", r.id, err)
			}
			if err := users.Put([]byte(r.id), next); err != nil {
				return err
			}
		}
		if err := tx.DeleteBucket([]byte(emailIndex)); err != nil && !errors.Is(err, gobolt.ErrBucketNotFound) {
			return err
		}
		idx, err := tx.CreateBucket([]byte(emailIndex))
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.norm == "" {
				continue
			}
			if err := idx.Put([]byte(r.norm), []byte(r.id)); err != nil {
				return err
			}
		}
		return nil
	})
	return rep, err
}

func loadUserBlobs(db *gobolt.DB) (map[string][]byte, error) {
	out := make(map[string][]byte)
	err := db.View(func(tx *gobolt.Tx) error {
		users := tx.Bucket([]byte(usersBucket))
		if users == nil {
			return fmt.Errorf("bucket %q missing", usersBucket)
		}
		return users.ForEach(func(k, v []byte) error {
			out[string(k)] = append([]byte(nil), v...)
			return nil
		})
	})
	return out, err
}

// verifyUsersAgainstOld checks every user in db against dbOld.
// Email in db must be normalize(old Email). Every other JSON field must match
// the old record byte-for-byte.
func verifyUsersAgainstOld(db, dbOld *gobolt.DB) ([]string, error) {
	got, err := loadUserBlobs(db)
	if err != nil {
		return nil, fmt.Errorf("read -db: %w", err)
	}
	old, err := loadUserBlobs(dbOld)
	if err != nil {
		return nil, fmt.Errorf("read -db_old: %w", err)
	}
	var mismatches []string
	if len(got) != len(old) {
		mismatches = append(mismatches, fmt.Sprintf("user count: -db=%d -db_old=%d", len(got), len(old)))
	}
	for id := range old {
		if _, ok := got[id]; !ok {
			mismatches = append(mismatches, fmt.Sprintf("user %s: present in -db_old, missing from -db", id))
		}
	}
	for id := range got {
		if _, ok := old[id]; !ok {
			mismatches = append(mismatches, fmt.Sprintf("user %s: present in -db, missing from -db_old", id))
		}
	}
	for id, oldRaw := range old {
		newRaw, ok := got[id]
		if !ok {
			continue
		}
		mismatches = append(mismatches, diffUserStoredValues(id, oldRaw, newRaw)...)
	}
	return mismatches, nil
}

func verifyEmailIndex(db *gobolt.DB) ([]string, error) {
	blobs, err := loadUserBlobs(db)
	if err != nil {
		return nil, err
	}
	want := make(map[string]string, len(blobs))
	for id, raw := range blobs {
		email, err := emailFromUserJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("user %s: %w", id, err)
		}
		norm := normalizeEmail(email)
		if norm == "" {
			continue
		}
		if prev, ok := want[norm]; ok {
			return []string{fmt.Sprintf("index: normalized email %q maps to both %s and %s", norm, prev, id)}, nil
		}
		want[norm] = id
	}

	var mismatches []string
	seen := make(map[string]struct{})
	err = db.View(func(tx *gobolt.Tx) error {
		idx := tx.Bucket([]byte(emailIndex))
		if idx == nil {
			mismatches = append(mismatches, "email index bucket missing")
			return nil
		}
		return idx.ForEach(func(k, v []byte) error {
			email := string(k)
			id := string(v)
			seen[email] = struct{}{}
			wantID, ok := want[email]
			if !ok {
				mismatches = append(mismatches, fmt.Sprintf("index: extra key %q -> %s", email, id))
				return nil
			}
			if wantID != id {
				mismatches = append(mismatches, fmt.Sprintf("index: %q -> %s, want user %s", email, id, wantID))
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	for email, id := range want {
		if _, ok := seen[email]; !ok {
			mismatches = append(mismatches, fmt.Sprintf("index: missing key %q for user %s", email, id))
		}
	}
	return mismatches, nil
}

func diffUserStoredValues(id string, oldRaw, newRaw []byte) []string {
	oldM, err := unmarshalFields(oldRaw)
	if err != nil {
		return []string{fmt.Sprintf("user %s: decode -db_old: %v", id, err)}
	}
	newM, err := unmarshalFields(newRaw)
	if err != nil {
		return []string{fmt.Sprintf("user %s: decode -db: %v", id, err)}
	}
	keys := make(map[string]struct{}, len(oldM)+len(newM))
	for k := range oldM {
		keys[k] = struct{}{}
	}
	for k := range newM {
		keys[k] = struct{}{}
	}
	var out []string
	for k := range keys {
		ov, ook := oldM[k]
		nv, nok := newM[k]
		if !ook {
			out = append(out, fmt.Sprintf("user %s field %s: present in -db, missing from -db_old", id, k))
			continue
		}
		if !nok {
			out = append(out, fmt.Sprintf("user %s field %s: present in -db_old, missing from -db", id, k))
			continue
		}
		if k == "Email" {
			oldEmail, err1 := jsonString(ov)
			newEmail, err2 := jsonString(nv)
			if err1 != nil || err2 != nil {
				out = append(out, fmt.Sprintf("user %s field Email: not a JSON string", id))
				continue
			}
			want := normalizeEmail(oldEmail)
			if newEmail != want {
				out = append(out, fmt.Sprintf("user %s field Email: -db=%q want normalized %q (old %q)", id, newEmail, want, oldEmail))
			}
			continue
		}
		if !bytes.Equal(ov, nv) {
			out = append(out, fmt.Sprintf("user %s field %s: stored value changed", id, k))
		}
	}
	return out
}

func unmarshalFields(raw []byte) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func jsonString(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func emailFromUserJSON(raw []byte) (string, error) {
	var rec struct {
		Email string `json:"Email"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return "", err
	}
	return rec.Email, nil
}

func setUserEmailJSON(raw []byte, email string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	b, err := json.Marshal(email)
	if err != nil {
		return nil, err
	}
	m["Email"] = b
	return json.Marshal(m)
}
