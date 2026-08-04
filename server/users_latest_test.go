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

	if users[0].Email != "latest0@example.com" {
		t.Fatalf("first=%s want latest0", users[0].Email)
	}
	for i := 1; i < len(users); i++ {
		if users[i].Updated.After(users[i-1].Updated) {
			t.Fatalf("not sorted DESC at %d", i)
		}
	}

	if trial != 5 {
		t.Fatalf("trial=%d want 5", trial)
	}

	if active != 8 {
		t.Fatalf("active=%d want 8", active)
	}
}
