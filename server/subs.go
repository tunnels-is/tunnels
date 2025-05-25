package main

import (
	"context"
	"strconv"
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
				ADMIN("KEY: unable to validate", u.Key.Key, err)
				return
			} else {
				return
			}
		} else {
			ADMIN("KEY: unable to validate:", u.Key.Key, err)
			return
		}
	}

	if key.LicenseKey.Status == "active" {
		ns := strings.Split(key.LicenseAttributes.Meta.ProductName, " ")
		months, err := strconv.Atoi(ns[0])
		if err != nil {
			ADMIN("unable to parse license key name:", err)
			return
		}
		u.SubExpiration = time.Now().AddDate(0, months, 0)

		_ = DB_updateUserSubTime(u)
	}
}
