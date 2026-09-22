package main

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func getUsers(limit, offset int64) ([]*User, error) {
	users := make([]*User, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if skipped < offset {
				skipped++
				continue
			}
			if int64(len(users)) >= limit {
				break
			}
			user := new(User)
			if err := bboltUnmarshal(v, user); err == nil {
				users = append(users, user)
			}
		}
		return nil
	})
	return users, err
}

func getUsersLatest(topN, batchSize int) (users []*User, total, trial, active int64, err error) {
	if topN <= 0 {
		topN = 100
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	now := time.Now()
	top := make([]*User, 0, topN)
	batch := make([]*User, 0, batchSize)

	flushBatch := func() {
		for _, u := range batch {
			total++
			if u.Trial {
				trial++
			}
			if !u.Disabled && u.SubExpiration.After(now) {
				active++
			}
			top = insertUserByUpdatedDesc(top, u, topN)
		}

		for i := range batch {
			batch[i] = nil
		}
		batch = batch[:0]
	}

	err = db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			user := new(User)
			if err := bboltUnmarshal(v, user); err != nil {
				continue
			}
			batch = append(batch, user)
			if len(batch) >= batchSize {
				flushBatch()
			}
		}
		if len(batch) > 0 {
			flushBatch()
		}
		return nil
	})
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return top, total, trial, active, nil
}

func insertUserByUpdatedDesc(list []*User, u *User, n int) []*User {
	if u == nil || n <= 0 {
		return list
	}

	if len(list) >= n && !u.Updated.After(list[len(list)-1].Updated) {
		return list
	}
	i := 0
	for i < len(list) && !u.Updated.After(list[i].Updated) {
		i++
	}
	if i >= n {
		return list
	}
	list = append(list, nil)
	copy(list[i+1:], list[i:])
	list[i] = u
	if len(list) > n {
		list = list[:n]
	}
	return list
}

func findUserByID(id uuid.UUID) (*User, error) {
	idStr := id.String()
	var user *User
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(idStr))
		if v == nil {
			return nil
		}
		user = new(User)
		return bboltUnmarshal(v, user)
	})
	return user, err
}

func createUser(user *User) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		id := user.ID.String()

		emailIdx := tx.Bucket([]byte(bucketUsersEmailIndex))
		user.Email = normalizeEmail(user.Email)
		if user.Email != "" && emailInUse(tx, user.Email, id) {
			return errEmailRegistered
		}
		data, err := bboltMarshal(user)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if user.Email != "" {
			if err := emailIdx.Put([]byte(user.Email), []byte(id)); err != nil {
				return err
			}
		}
		if user.APIKey != "" {
			if err := tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Put([]byte(user.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func emailIndexGet(idx *gobolt.Bucket, email string) []byte {
	if idx == nil {
		return nil
	}
	n := normalizeEmail(email)
	if n == "" {
		return nil
	}
	return idx.Get([]byte(n))
}

func emailInUse(tx *gobolt.Tx, email, exceptID string) bool {
	v := emailIndexGet(tx.Bucket([]byte(bucketUsersEmailIndex)), email)
	return v != nil && string(v) != exceptID
}

func findUserByEmail(email string) (*User, error) {
	var found *User
	n := normalizeEmail(email)
	if n == "" {
		return nil, nil
	}
	err := db.View(func(tx *gobolt.Tx) error {
		uid := emailIndexGet(tx.Bucket([]byte(bucketUsersEmailIndex)), n)
		if uid == nil {
			return nil
		}
		v := tx.Bucket([]byte(bucketUsers)).Get(uid)
		if v == nil {
			return nil
		}
		found = new(User)
		return bboltUnmarshal(v, found)
	})
	return found, err
}

func updateUserDeviceTokens(update *userTokensUpdate) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		id := update.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("user not found")
		}
		user := new(User)
		if err := bboltUnmarshal(v, user); err != nil {
			return err
		}
		user.Tokens = update.Tokens
		data, err := bboltMarshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func updateUserSubTime(u *User) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		id := u.ID.String()
		v := b.Get([]byte(id))
		if v == nil && u.Email != "" {
			uid := emailIndexGet(tx.Bucket([]byte(bucketUsersEmailIndex)), u.Email)
			if uid != nil {
				id = string(uid)
				v = b.Get(uid)
			}
		}
		if v == nil {
			return errors.New("user not found")
		}
		user := new(User)
		if err := bboltUnmarshal(v, user); err != nil {
			return err
		}
		user.SubExpiration = u.SubExpiration
		data, err := bboltMarshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func updateUser(form *userUpdateRequest) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		id := form.UID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("user not found")
		}
		user := new(User)
		if err := bboltUnmarshal(v, user); err != nil {
			return err
		}
		oldAPIKey := user.APIKey
		user.APIKey = form.APIKey
		data, err := bboltMarshal(user)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		apikeyIdx := tx.Bucket([]byte(bucketUsersAPIKeyIndex))
		if oldAPIKey != "" {
			if err := apikeyIdx.Delete([]byte(oldAPIKey)); err != nil {
				return err
			}
		}
		if form.APIKey != "" {
			if err := apikeyIdx.Put([]byte(form.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func updateUserAdmin(form *adminUserUpdateRequest) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		id := form.TargetUserID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("user not found")
		}
		user := new(User)
		if err := bboltUnmarshal(v, user); err != nil {
			return err
		}

		oldEmail := user.Email
		emailChanged := false

		if form.Email != "" {
			newEmail := normalizeEmail(form.Email)
			if newEmail != user.Email {
				if emailInUse(tx, newEmail, id) {
					return errors.New("email already in use by another account")
				}
				user.Email = newEmail
				emailChanged = true
			}
		}

		if !form.SubExpiration.IsZero() {
			user.SubExpiration = form.SubExpiration
		}

		user.Disabled = form.Disabled
		user.Trial = form.Trial

		data, err := bboltMarshal(user)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}

		if emailChanged {
			emailIdx := tx.Bucket([]byte(bucketUsersEmailIndex))
			if oldEmail != "" {
				_ = emailIdx.Delete([]byte(oldEmail))
			}
			if user.Email != "" {
				if err := emailIdx.Put([]byte(user.Email), []byte(id)); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func updateUserRecoveryCodes(uid uuid.UUID, codes []byte) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		id := uid.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("user not found")
		}
		user := new(User)
		if err := bboltUnmarshal(v, user); err != nil {
			return err
		}
		user.RecoveryCodes = codes
		data, err := bboltMarshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func updateUserTwoFactorCodes(upd *twoFactorUpdate) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(upd.UID.String()))
		if v == nil {
			return errors.New("user not found")
		}
		user := new(User)
		if err := bboltUnmarshal(v, user); err != nil {
			return err
		}
		user.TwoFactorCode = upd.Code
		user.RecoveryCodes = upd.Recovery
		user.TwoFactorEnabled = true
		data, err := bboltMarshal(user)
		if err != nil {
			return err
		}
		return b.Put([]byte(user.ID.String()), data)
	})
}

func resetUserPassword(user *User) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(user.ID.String()))
		if v == nil {
			return errors.New("user not found")
		}
		stored := new(User)
		if err := bboltUnmarshal(v, stored); err != nil {
			return err
		}
		stored.Password = user.Password
		stored.Tokens = []*DeviceToken{}
		data, err := bboltMarshal(stored)
		if err != nil {
			return err
		}
		return b.Put([]byte(stored.ID.String()), data)
	})
}

func deleteUserByID(id uuid.UUID) error {
	idStr := id.String()
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(idStr))
		if v != nil {
			user := new(User)
			if err := bboltUnmarshal(v, user); err == nil {
				if user.Email != "" {
					_ = tx.Bucket([]byte(bucketUsersEmailIndex)).Delete([]byte(user.Email))
				}
				if user.APIKey != "" {
					_ = tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Delete([]byte(user.APIKey))
				}
			}
		}

		deleteDevicesTx(tx, func(devID string, D *types.Device) bool {
			return D.UserID.String() == idStr
		})
		return b.Delete([]byte(idStr))
	})
}

func activateUserKey(subExpiration time.Time, key *LicenseKey, userID uuid.UUID) error {
	idStr := userID.String()
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(idStr))
		if v == nil {
			return errors.New("user not found")
		}
		user := new(User)
		if err := bboltUnmarshal(v, user); err != nil {
			return err
		}
		user.Disabled = false
		user.Trial = false
		user.SubExpiration = subExpiration
		user.Key = key
		data, err := bboltMarshal(user)
		if err != nil {
			return err
		}
		id := user.ID.String()
		return b.Put([]byte(id), data)
	})
}
