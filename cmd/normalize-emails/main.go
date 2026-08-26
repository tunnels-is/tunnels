// One-shot migration: lowercase/trim User.Email and rebuild users_by_email.
// This does not run when the controller opens the database.
//
// Copy tunnels.db first, then:
//
//	go run ./cmd/normalize-emails -db tunnels.db -db_old tunnels.db.old
//	go run ./cmd/normalize-emails -db tunnels.db -db_old tunnels.db.old -apply
//
// After -apply, each users_by_email winner in -db is checked against
// -db_old: Email must be the normalized form of the old Email, and every
// other stored field must match exactly. Collision losers are not compared.
//
// If two or more users share an email after normalization, every record is
// kept and the email is still normalized. users_by_email points at one
// winner: a user whose SubExpiration is in the future, if any; otherwise
// (or if more than one has a valid subscription) the most recently created.
// User has no Created field; Updated is written at registration and not
// changed later, so it is the creation time. Equal Updated times break
// ties by larger user id.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	Email  string
	IDs    []string
	Winner string
	Reason string
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
	fmt.Println("verification OK: every email-index winner matches -db_old (Email normalized, all other fields unchanged); email index rebuilt")
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
		fmt.Printf("  COLLISION  email=%q  winner=%s  reason=%q  ids=%s\n",
			c.Email, c.Winner, c.Reason, strings.Join(c.IDs, ","))
	}
}

type row struct {
	id      string
	raw     []byte
	email   string
	norm    string
	updated time.Time
	subExp  time.Time
}

func migrateEmails(db *gobolt.DB, apply bool) (*report, error) {
	rows, err := loadUserRows(db)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	owner, collisions := resolveEmailOwners(rows, now)

	rep := &report{Users: len(rows), Collisions: collisions}
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
		for email, id := range owner {
			if err := idx.Put([]byte(email), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
	return rep, err
}

func loadUserRows(db *gobolt.DB) ([]row, error) {
	var rows []row
	err := db.View(func(tx *gobolt.Tx) error {
		users := tx.Bucket([]byte(usersBucket))
		if users == nil {
			return fmt.Errorf("bucket %q missing", usersBucket)
		}
		return users.ForEach(func(k, v []byte) error {
			email, updated, subExp, err := userFieldsFromJSON(v)
			if err != nil {
				return fmt.Errorf("user %s: %w", k, err)
			}
			rows = append(rows, row{
				id:      string(k),
				raw:     append([]byte(nil), v...),
				email:   email,
				norm:    normalizeEmail(email),
				updated: updated,
				subExp:  subExp,
			})
			return nil
		})
	})
	return rows, err
}

func resolveEmailOwners(rows []row, now time.Time) (map[string]string, []collision) {
	byNorm := make(map[string][]row)
	for _, r := range rows {
		if r.norm == "" {
			continue
		}
		byNorm[r.norm] = append(byNorm[r.norm], r)
	}
	owner := make(map[string]string, len(byNorm))
	var collisions []collision
	for email, cands := range byNorm {
		if len(cands) == 1 {
			owner[email] = cands[0].id
			continue
		}
		w, reason := pickCollisionWinner(cands, now)
		ids := make([]string, len(cands))
		for i, c := range cands {
			ids[i] = c.id
		}
		collisions = append(collisions, collision{
			Email:  email,
			IDs:    ids,
			Winner: w.id,
			Reason: reason,
		})
		owner[email] = w.id
	}
	sort.Slice(collisions, func(i, j int) bool {
		return collisions[i].Email < collisions[j].Email
	})
	return owner, collisions
}

func pickCollisionWinner(cands []row, now time.Time) (row, string) {
	if len(cands) == 0 {
		return row{}, ""
	}
	var withSub []row
	for _, c := range cands {
		if c.subExp.After(now) {
			withSub = append(withSub, c)
		}
	}
	if len(withSub) == 1 {
		return withSub[0], "valid subscription"
	}
	pool := cands
	if len(withSub) > 1 {
		pool = withSub
	}
	best := pool[0]
	for _, c := range pool[1:] {
		if c.updated.After(best.updated) || (c.updated.Equal(best.updated) && c.id > best.id) {
			best = c
		}
	}
	return best, "most recently created"
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

// verifyUsersAgainstOld checks email-index winners in db against dbOld.
// Collision losers are ignored: they may be missing or have other fields
// changed. For each normalized email, Email in db must be normalize(old
// Email) and every other JSON field on the winner must match byte-for-byte.
// Users with an empty email are compared by id.
func verifyUsersAgainstOld(db, dbOld *gobolt.DB) ([]string, error) {
	gotRows, err := loadUserRows(db)
	if err != nil {
		return nil, fmt.Errorf("read -db: %w", err)
	}
	oldRows, err := loadUserRows(dbOld)
	if err != nil {
		return nil, fmt.Errorf("read -db_old: %w", err)
	}
	now := time.Now()
	gotOwners, _ := resolveEmailOwners(gotRows, now)
	oldOwners, _ := resolveEmailOwners(oldRows, now)
	got := blobsFromRows(gotRows)
	old := blobsFromRows(oldRows)

	emails := make(map[string]struct{}, len(gotOwners)+len(oldOwners))
	for email := range oldOwners {
		emails[email] = struct{}{}
	}
	for email := range gotOwners {
		emails[email] = struct{}{}
	}
	keys := make([]string, 0, len(emails))
	for email := range emails {
		keys = append(keys, email)
	}
	sort.Strings(keys)

	var mismatches []string
	for _, email := range keys {
		oldID, oldOK := oldOwners[email]
		gotID, gotOK := gotOwners[email]
		switch {
		case !oldOK:
			mismatches = append(mismatches, fmt.Sprintf("email %q: present in -db (user %s), missing from -db_old", email, gotID))
		case !gotOK:
			mismatches = append(mismatches, fmt.Sprintf("email %q: present in -db_old (user %s), missing from -db", email, oldID))
		case oldID != gotID:
			mismatches = append(mismatches, fmt.Sprintf("email %q: winner -db=%s -db_old=%s", email, gotID, oldID))
		default:
			mismatches = append(mismatches, diffUserStoredValues(oldID, old[oldID], got[gotID])...)
		}
	}

	oldEmpty := emptyUserIDs(oldRows)
	gotEmpty := emptyUserIDs(gotRows)
	emptyIDs := make(map[string]struct{}, len(oldEmpty)+len(gotEmpty))
	for id := range oldEmpty {
		emptyIDs[id] = struct{}{}
	}
	for id := range gotEmpty {
		emptyIDs[id] = struct{}{}
	}
	emptyKeys := make([]string, 0, len(emptyIDs))
	for id := range emptyIDs {
		emptyKeys = append(emptyKeys, id)
	}
	sort.Strings(emptyKeys)
	for _, id := range emptyKeys {
		_, inOld := oldEmpty[id]
		_, inGot := gotEmpty[id]
		switch {
		case !inOld:
			mismatches = append(mismatches, fmt.Sprintf("user %s: present in -db, missing from -db_old", id))
		case !inGot:
			mismatches = append(mismatches, fmt.Sprintf("user %s: present in -db_old, missing from -db", id))
		default:
			mismatches = append(mismatches, diffUserStoredValues(id, old[id], got[id])...)
		}
	}
	return mismatches, nil
}

func blobsFromRows(rows []row) map[string][]byte {
	out := make(map[string][]byte, len(rows))
	for _, r := range rows {
		out[r.id] = r.raw
	}
	return out
}

func emptyUserIDs(rows []row) map[string]struct{} {
	out := make(map[string]struct{})
	for _, r := range rows {
		if r.norm == "" {
			out[r.id] = struct{}{}
		}
	}
	return out
}

func verifyEmailIndex(db *gobolt.DB) ([]string, error) {
	rows, err := loadUserRows(db)
	if err != nil {
		return nil, err
	}
	want, _ := resolveEmailOwners(rows, time.Now())

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
	email, _, _, err := userFieldsFromJSON(raw)
	return email, err
}

func userFieldsFromJSON(raw []byte) (email string, updated, subExp time.Time, err error) {
	var rec struct {
		Email         string          `json:"Email"`
		Updated       json.RawMessage `json:"Updated"`
		SubExpiration json.RawMessage `json:"SubExpiration"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	return rec.Email, parseTimeJSON(rec.Updated), parseTimeJSON(rec.SubExpiration), nil
}

func parseTimeJSON(raw json.RawMessage) time.Time {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}
	}
	var t time.Time
	if err := json.Unmarshal(raw, &t); err != nil {
		return time.Time{}
	}
	return t
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
