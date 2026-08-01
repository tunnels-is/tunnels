package main

import (
	"fmt"
	"testing"
	"time"
)

func TestInsertUserByUpdatedDesc(t *testing.T) {
	mk := func(email string, hoursAgo int) *User {
		return &User{
			Email:   email,
			Updated: time.Now().Add(-time.Duration(hoursAgo) * time.Hour),
		}
	}
	var top []*User
	// Insert out of order; keep top 3 newest.
	for _, u := range []*User{mk("old", 10), mk("new", 1), mk("mid", 5), mk("newer", 0), mk("ancient", 100)} {
		top = insertUserByUpdatedDesc(top, u, 3)
	}
	if len(top) != 3 {
		t.Fatalf("len=%d want 3", len(top))
	}
	if top[0].Email != "newer" || top[1].Email != "new" || top[2].Email != "mid" {
		t.Fatalf("order=%v,%v,%v", top[0].Email, top[1].Email, top[2].Email)
	}
}

func TestBBolt_getUsersLatest(t *testing.T) {
	setupTestDB(t)
	now := time.Now()
	for i := 0; i < 15; i++ {
		u := testUser(fmt.Sprintf("latest%d@example.com", i), "")
		u.Updated = now.Add(-time.Duration(i) * time.Hour)
		u.Trial = i%3 == 0
		if i%2 == 0 {
			u.SubExpiration = now.Add(24 * time.Hour)
		} else {
			u.SubExpiration = now.Add(-24 * time.Hour)
		}
		if err := BBolt_CreateUser(u); err != nil {
			t.Fatal(err)
		}
	}

	users, total, trial, active, err := BBolt_getUsersLatest(5, 4)
	if err != nil {
		t.Fatal(err)
	}
	if total != 15 {
		t.Fatalf("total=%d want 15", total)
	}
	if len(users) != 5 {
		t.Fatalf("len users=%d want 5", len(users))
	}
	// Newest first: latest0, latest1, ...
	if users[0].Email != "latest0@example.com" {
		t.Fatalf("first=%s want latest0", users[0].Email)
	}
	for i := 1; i < len(users); i++ {
		if users[i].Updated.After(users[i-1].Updated) {
			t.Fatalf("not sorted DESC at %d", i)
		}
	}
	// Trials: i%3==0 → 0,3,6,9,12 = 5
	if trial != 5 {
		t.Fatalf("trial=%d want 5", trial)
	}
	// Active: even i, not disabled, sub future → 0,2,4,6,8,10,12,14 = 8
	if active != 8 {
		t.Fatalf("active=%d want 8", active)
	}
}
