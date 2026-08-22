package main

import (
	"context"
	"strings"
	"time"
)

func scanSubs() {
	var limit int64 = 100
	var offset int64 = 0
	for {
		users, err := DB_getUsers(limit, offset)
		if err != nil {
			return
		}
		offset += limit

		for i := range users {
			if time.Now().After(users[i].SubExpiration) {
				if users[i].Key != nil && users[i].Key.Key != "unknown" && users[i].Key.Key != "" {
					time.Sleep(1 * time.Second)
					checkIfUserSubIsActive(users[i])
				}
			}
		}

		if int64(len(users)) < limit {
			break
		}
	}
}

func checkIfUserSubIsActive(u *User) {
	lemonClient := lc.Load()
	key, resp, err := lemonClient.Licenses.Validate(context.Background(), u.Key.Key, "")
	if err != nil {
		if resp != nil && resp.Body != nil {
			bs := string(*resp.Body)
			if !strings.Contains(bs, "expired") {
				ADMIN("KEY: unable to validate", redactKey(u.Key.Key), err)
				return
			}
			return
		}
		ADMIN("KEY: unable to validate:", redactKey(u.Key.Key), err)
		return
	}

	newExp, update := nextSubExpirationFromLemon(u, key.LicenseKey.Status, key.LicenseKey.ExpiresAt)
	if !update {
		return
	}
	u.SubExpiration = newExp
	_ = DB_updateUserSubTime(u)
}

func licensePaidThrough(u *User) time.Time {
	if u == nil || u.Key == nil || u.Key.Months <= 0 {
		if u == nil {
			return time.Time{}
		}
		return u.SubExpiration
	}
	if u.Key.Created.IsZero() {
		return u.SubExpiration
	}
	return u.Key.Created.AddDate(0, u.Key.Months, 0)
}

// nextSubExpirationFromLemon never stacks another product term on Lemon
// status "active". Prefer LS expires_at. If that is missing, honor the
// original paid-through date and do not extend past it.
func nextSubExpirationFromLemon(u *User, status string, expiresAt *time.Time) (time.Time, bool) {
	if u == nil {
		return time.Time{}, false
	}
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "expired", "disabled", "inactive":
		return u.SubExpiration, false
	}
	if expiresAt != nil && !expiresAt.IsZero() {
		return expiresAt.UTC(), true
	}
	if status != "active" {
		return u.SubExpiration, false
	}
	through := licensePaidThrough(u)
	if through.After(time.Now()) && !through.Equal(u.SubExpiration) {
		return through, true
	}
	return u.SubExpiration, false
}
