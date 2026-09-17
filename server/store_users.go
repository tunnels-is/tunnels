package main

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func getUsers(limit, offset int64) ([]*User, error) {
	UL := make([]*User, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if skipped < offset {
				skipped++
				continue
			}
			if int64(len(UL)) >= limit {
				break
			}
			U := new(User)
			if err := bboltUnmarshal(v, U); err == nil {
				UL = append(UL, U)
			}
		}
		return nil
	})
	return UL, err
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
			U := new(User)
			if err := bboltUnmarshal(v, U); err != nil {
				continue
			}
			batch = append(batch, U)
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

func findUserByID(UID uuid.UUID) (*User, error) {
	idStr := UID.String()
	var U *User
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(idStr))
		if v == nil {
			return nil
		}
		U = new(User)
		return bboltUnmarshal(v, U)
	})
	return U, err
}

func createUser(U *User) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		id := U.ID.String()

		emailIdx := tx.Bucket([]byte(bucketUsersEmailIndex))
		U.Email = normalizeEmail(U.Email)
		if U.Email != "" && emailInUse(tx, U.Email, id) {
			return errEmailRegistered
		}
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if U.Email != "" {
			if err := emailIdx.Put([]byte(U.Email), []byte(id)); err != nil {
				return err
			}
		}
		if U.APIKey != "" {
			if err := tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Put([]byte(U.APIKey), []byte(id)); err != nil {
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

func findUserByEmail(Email string) (*User, error) {
	var found *User
	n := normalizeEmail(Email)
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
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.Tokens = update.Tokens
		data, err := bboltMarshal(U)
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
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.SubExpiration = u.SubExpiration
		data, err := bboltMarshal(U)
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
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		oldAPIKey := U.APIKey
		U.APIKey = form.APIKey
		data, err := bboltMarshal(U)
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
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}

		oldEmail := U.Email
		emailChanged := false

		if form.Email != "" {
			newEmail := normalizeEmail(form.Email)
			if newEmail != U.Email {
				if emailInUse(tx, newEmail, id) {
					return errors.New("email already in use by another account")
				}
				U.Email = newEmail
				emailChanged = true
			}
		}

		if !form.SubExpiration.IsZero() {
			U.SubExpiration = form.SubExpiration
		}

		U.Disabled = form.Disabled
		U.Trial = form.Trial

		data, err := bboltMarshal(U)
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
			if U.Email != "" {
				if err := emailIdx.Put([]byte(U.Email), []byte(id)); err != nil {
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
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.RecoveryCodes = codes
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func updateUserTwoFactorCodes(TFP *twoFactorUpdate) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(TFP.UID.String()))
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.TwoFactorCode = TFP.Code
		U.RecoveryCodes = TFP.Recovery
		U.TwoFactorEnabled = true
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put([]byte(U.ID.String()), data)
	})
}

func resetUserPassword(user *User) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(user.ID.String()))
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.Password = user.Password
		U.Tokens = []*DeviceToken{}
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put([]byte(U.ID.String()), data)
	})
}

func deleteUserByID(id uuid.UUID) error {
	idStr := id.String()
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(idStr))
		if v != nil {
			U := new(User)
			if err := bboltUnmarshal(v, U); err == nil {
				if U.Email != "" {
					_ = tx.Bucket([]byte(bucketUsersEmailIndex)).Delete([]byte(U.Email))
				}
				if U.APIKey != "" {
					_ = tx.Bucket([]byte(bucketUsersAPIKeyIndex)).Delete([]byte(U.APIKey))
				}
			}
		}

		deleteDevicesTx(tx, func(devID string, D *types.Device) bool {
			return D.UserID.String() == idStr
		})
		return b.Delete([]byte(idStr))
	})
}

func activateUserKey(SubExpiration time.Time, Key *LicenseKey, userID uuid.UUID) error {
	idStr := userID.String()
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketUsers))
		v := b.Get([]byte(idStr))
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.Disabled = false
		U.Trial = false
		U.SubExpiration = SubExpiration
		U.Key = Key
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		id := U.ID.String()
		return b.Put([]byte(id), data)
	})
}
