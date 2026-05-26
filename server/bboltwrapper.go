package main

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

var BBoltDB *gobolt.DB

const (
	USERS_BUCKET         = "users"
	USERS_EMAIL_INDEX    = "users_by_email"
	USERS_APIKEY_INDEX   = "users_by_apikey"
	DEVICES_BUCKET       = "devices"
	DEVICES_USERID_INDEX = "devices_by_user_id"
	DEVICES_WGKEY_INDEX  = "devices_by_wg_key"
	ORGS_BUCKET          = "orgs"
	GROUPS_BUCKET        = "groups"
	SERVERS_BUCKET       = "servers"
	SERVERS_APIKEY_INDEX = "servers_by_apikey"
)

func ConnectToBBoltDB(path string) (err error) {
	BBoltDB, err = gobolt.Open(path, 0o600, &gobolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		buckets := []string{
			USERS_BUCKET, USERS_EMAIL_INDEX, USERS_APIKEY_INDEX,
			DEVICES_BUCKET, DEVICES_USERID_INDEX, DEVICES_WGKEY_INDEX,
			ORGS_BUCKET, GROUPS_BUCKET, SERVERS_BUCKET, SERVERS_APIKEY_INDEX,
		}
		for _, b := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(b))
			if err != nil {
				return err
			}
		}

		// Backfill secondary indexes from existing users.
		users := tx.Bucket([]byte(USERS_BUCKET))
		emailIdx := tx.Bucket([]byte(USERS_EMAIL_INDEX))
		apikeyIdx := tx.Bucket([]byte(USERS_APIKEY_INDEX))
		uc := users.Cursor()
		for k, v := uc.First(); k != nil; k, v = uc.Next() {
			U := new(User)
			if err := bboltUnmarshal(v, U); err != nil {
				continue
			}
			if U.Email != "" {
				if err := emailIdx.Put([]byte(U.Email), k); err != nil {
					return err
				}
			}
			if U.APIKey != "" {
				if err := apikeyIdx.Put([]byte(U.APIKey), k); err != nil {
					return err
				}
			}
		}

		// Backfill devices_by_user_id index.
		devices := tx.Bucket([]byte(DEVICES_BUCKET))
		devUserIdx := tx.Bucket([]byte(DEVICES_USERID_INDEX))
		dc := devices.Cursor()
		for k, v := dc.First(); k != nil; k, v = dc.Next() {
			D := new(types.Device)
			if err := bboltUnmarshal(v, D); err != nil {
				continue
			}
			uid := D.UserID.String()
			if uid != "00000000-0000-0000-0000-000000000000" {
				compositeKey := []byte(uid + "/" + string(k))
				if err := devUserIdx.Put(compositeKey, nil); err != nil {
					return err
				}
			}
		}

		// Backfill servers_by_apikey index.
		servers := tx.Bucket([]byte(SERVERS_BUCKET))
		srvApikeyIdx := tx.Bucket([]byte(SERVERS_APIKEY_INDEX))
		sc := servers.Cursor()
		for k, v := sc.First(); k != nil; k, v = sc.Next() {
			S := new(types.Server)
			if err := bboltUnmarshal(v, S); err != nil {
				continue
			}
			if S.APIKey != "" {
				if err := srvApikeyIdx.Put([]byte(S.APIKey), k); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func bboltMarshal(v any) ([]byte, error)      { return json.Marshal(v) }
func bboltUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

func BBolt_DeleteDeviceByID(id string) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(DEVICES_BUCKET))
		v := b.Get([]byte(id))
		if v != nil {
			D := new(types.Device)
			if err := bboltUnmarshal(v, D); err == nil {
				uid := D.UserID.String()
				if uid != "00000000-0000-0000-0000-000000000000" {
					_ = tx.Bucket([]byte(DEVICES_USERID_INDEX)).Delete([]byte(uid + "/" + id))
				}
				if D.WireGuardKey != "" {
					_ = tx.Bucket([]byte(DEVICES_WGKEY_INDEX)).Delete([]byte(D.WireGuardKey))
				}
			}
		}
		return b.Delete([]byte(id))
	})
}

func BBolt_UpdateDevice(D *types.Device) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(DEVICES_BUCKET))
		id := D.ID.String()
		devUserIdx := tx.Bucket([]byte(DEVICES_USERID_INDEX))
		wgIdx := tx.Bucket([]byte(DEVICES_WGKEY_INDEX))

		var oldWGKey string
		if old := b.Get([]byte(id)); old != nil {
			oldD := new(types.Device)
			if err := bboltUnmarshal(old, oldD); err == nil {
				if oldD.UserID != D.UserID {
					oldUID := oldD.UserID.String()
					if oldUID != "00000000-0000-0000-0000-000000000000" {
						_ = devUserIdx.Delete([]byte(oldUID + "/" + id))
					}
				}
				oldWGKey = oldD.WireGuardKey
			}
		}

		if D.WireGuardKey != "" && D.WireGuardKey != oldWGKey {
			if existing := wgIdx.Get([]byte(D.WireGuardKey)); existing != nil && string(existing) != id {
				return errors.New("WireGuard key already in use")
			}
		}

		data, err := bboltMarshal(D)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}

		uid := D.UserID.String()
		if uid != "00000000-0000-0000-0000-000000000000" {
			if err := devUserIdx.Put([]byte(uid+"/"+id), nil); err != nil {
				return err
			}
		}

		if oldWGKey != "" && oldWGKey != D.WireGuardKey {
			_ = wgIdx.Delete([]byte(oldWGKey))
		}
		if D.WireGuardKey != "" {
			if err := wgIdx.Put([]byte(D.WireGuardKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func BBolt_GetDevices(limit, offset int64) ([]*types.Device, error) {
	DL := make([]*types.Device, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(DEVICES_BUCKET))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if skipped < offset {
				skipped++
				continue
			}
			if int64(len(DL)) >= limit {
				break
			}
			D := new(types.Device)
			if err := bboltUnmarshal(v, D); err == nil {
				DL = append(DL, D)
			}
		}
		return nil
	})
	return DL, err
}

func BBolt_GetDevicesByUserID(userID uuid.UUID) ([]*types.Device, error) {
	DL := make([]*types.Device, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		devices := tx.Bucket([]byte(DEVICES_BUCKET))
		idx := tx.Bucket([]byte(DEVICES_USERID_INDEX))
		prefix := []byte(userID.String() + "/")
		c := idx.Cursor()
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			devID := k[len(prefix):]
			v := devices.Get(devID)
			if v == nil {
				continue
			}
			D := new(types.Device)
			if err := bboltUnmarshal(v, D); err == nil {
				DL = append(DL, D)
			}
		}
		return nil
	})
	return DL, err
}

func BBolt_getUsers(limit, offset int64) ([]*User, error) {
	UL := make([]*User, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
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

func BBolt_findUserByAPIKey(Key string) (*User, error) {
	var found *User
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		uid := tx.Bucket([]byte(USERS_APIKEY_INDEX)).Get([]byte(Key))
		if uid == nil {
			return nil
		}
		v := tx.Bucket([]byte(USERS_BUCKET)).Get(uid)
		if v == nil {
			return nil
		}
		found = new(User)
		return bboltUnmarshal(v, found)
	})
	return found, err
}

func BBolt_findUserByID(UID string) (*User, error) {
	var U *User
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		v := b.Get([]byte(UID))
		if v == nil {
			return nil
		}
		U = new(User)
		return bboltUnmarshal(v, U)
	})
	return U, err
}

func BBolt_CreateUser(U *User) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		id := U.ID.String()
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if err := tx.Bucket([]byte(USERS_EMAIL_INDEX)).Put([]byte(U.Email), []byte(id)); err != nil {
			return err
		}
		if U.APIKey != "" {
			if err := tx.Bucket([]byte(USERS_APIKEY_INDEX)).Put([]byte(U.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func BBolt_findUserByEmail(Email string) (*User, error) {
	var found *User
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		uid := tx.Bucket([]byte(USERS_EMAIL_INDEX)).Get([]byte(Email))
		if uid == nil {
			return nil
		}
		v := tx.Bucket([]byte(USERS_BUCKET)).Get(uid)
		if v == nil {
			return nil
		}
		found = new(User)
		return bboltUnmarshal(v, found)
	})
	return found, err
}

func BBolt_updateUserDeviceTokens(TU *UPDATE_USER_TOKENS) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		id := TU.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.Tokens = TU.Tokens
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func BBolt_updateUserSubTime(u *User) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		uid := tx.Bucket([]byte(USERS_EMAIL_INDEX)).Get([]byte(u.Email))
		if uid == nil {
			return errors.New("user not found")
		}
		b := tx.Bucket([]byte(USERS_BUCKET))
		v := b.Get(uid)
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
		return b.Put(uid, data)
	})
}

func BBolt_updateUser(UF *USER_UPDATE_FORM) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		id := UF.UID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		oldAPIKey := U.APIKey
		U.APIKey = UF.APIKey
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		apikeyIdx := tx.Bucket([]byte(USERS_APIKEY_INDEX))
		if oldAPIKey != "" {
			if err := apikeyIdx.Delete([]byte(oldAPIKey)); err != nil {
				return err
			}
		}
		if UF.APIKey != "" {
			if err := apikeyIdx.Put([]byte(UF.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func BBolt_updateUserAdmin(UF *USER_ADMIN_UPDATE_FORM) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		id := UF.TargetUserID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}

		oldEmail := U.Email

		if UF.Email != "" {
			U.Email = UF.Email
		}

		if !UF.SubExpiration.IsZero() {
			U.SubExpiration = UF.SubExpiration
		}

		U.Disabled = UF.Disabled
		U.Trial = UF.Trial

		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}

		if UF.Email != "" && UF.Email != oldEmail {
			emailIdx := tx.Bucket([]byte(USERS_EMAIL_INDEX))
			if err := emailIdx.Delete([]byte(oldEmail)); err != nil {
				return err
			}
			if err := emailIdx.Put([]byte(UF.Email), []byte(id)); err != nil {
				return err
			}
		}

		return nil
	})
}

func BBolt_userUpdateTwoFactorCodes(TFP *TWO_FACTOR_DB_PACKAGE) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
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

func BBolt_userResetPassword(user *User) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
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

func BBolt_FindServersWithoutGroups(limit, offset int64) ([]*types.Server, error) {
	DL := make([]*types.Server, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			S := new(types.Server)
			if err := bboltUnmarshal(v, S); err == nil {
				if len(S.Groups) == 0 {
					if skipped < offset {
						skipped++
						continue
					}
					if int64(len(DL)) >= limit {
						break
					}
					DL = append(DL, S)
				}
			}
		}
		return nil
	})
	return DL, err
}

func BBolt_FindServersByGroups(groups []uuid.UUID, limit, offset int64) ([]*types.Server, error) {
	DL := make([]*types.Server, 0)
	groupSet := make(map[uuid.UUID]struct{})
	for _, g := range groups {
		groupSet[g] = struct{}{}
	}
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			S := new(types.Server)
			if err := bboltUnmarshal(v, S); err == nil {
				for _, gid := range S.Groups {
					if _, ok := groupSet[gid]; ok {
						if skipped < offset {
							skipped++
							continue
						}
						if int64(len(DL)) >= limit {
							break
						}
						DL = append(DL, S)
						break
					}
				}
			}
		}
		return nil
	})
	return DL, err
}

func BBolt_FindEntitiesByGroupID(id string, objType string, limit, offset int64) ([]any, error) {
	IL := make([]any, 0)
	bucket := ""
	switch objType {
	case "user":
		bucket = USERS_BUCKET
	case "server":
		bucket = SERVERS_BUCKET
	case "device":
		bucket = DEVICES_BUCKET
	default:
		return nil, fmt.Errorf("unknown type")
	}
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()
		var skipped int64
		for _, v := c.First(); v != nil; _, v = c.Next() {
			var match bool
			switch objType {
			case "server":
				E := new(types.Server)
				if err := bboltUnmarshal(v, E); err == nil {
					if slices.Contains(uuidSliceToString(E.Groups), id) {
						match = true
					}
					if match {
						if skipped < offset {
							skipped++
							continue
						}
						if int64(len(IL)) >= limit {
							break
						}
						IL = append(IL, E)
					}
				}
			case "user":
				E := new(User)
				if err := bboltUnmarshal(v, E); err == nil {
					if slices.Contains(uuidSliceToString(E.Groups), id) {
						match = true
					}
					if match {
						if skipped < offset {
							skipped++
							continue
						}
						if int64(len(IL)) >= limit {
							break
						}
						IL = append(IL, E)
					}
				}
			case "device":
				E := new(types.Device)
				if err := bboltUnmarshal(v, E); err == nil {
					if slices.Contains(uuidSliceToString(E.Groups), id) {
						match = true
					}
					if match {
						if skipped < offset {
							skipped++
							continue
						}
						if int64(len(IL)) >= limit {
							break
						}
						IL = append(IL, E)
					}
				}
			}
		}
		return nil
	})
	return IL, err
}

func BBolt_UpdateGroup(G *Group) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(GROUPS_BUCKET))
		id := G.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("group not found")
		}
		GG := new(Group)
		if err := bboltUnmarshal(v, GG); err != nil {
			return err
		}
		GG.Tag = G.Tag
		GG.Description = G.Description
		data, err := bboltMarshal(GG)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func BBolt_UpdateServer(S *types.Server) (*types.Server, error) {
	var RS *types.Server
	err := BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		apikeyIdx := tx.Bucket([]byte(SERVERS_APIKEY_INDEX))
		id := S.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("server not found")
		}
		SS := new(types.Server)
		if err := bboltUnmarshal(v, SS); err != nil {
			return err
		}
		oldAPIKey := SS.APIKey
		SS.Tag = S.Tag
		SS.Country = S.Country
		SS.IP = S.IP
		SS.Port = S.Port
		SS.APIKey = S.APIKey
		SS.WireGuardPort = S.WireGuardPort
		SS.WireGuardPubKey = S.WireGuardPubKey
		SS.WireGuardIface = S.WireGuardIface
		SS.WireGuardSubnet = S.WireGuardSubnet
		SS.WireGuardSubnet6 = S.WireGuardSubnet6
		SS.InternetIface = S.InternetIface
		SS.PacketInspection = S.PacketInspection
		SS.InsecureSkipVerify = S.InsecureSkipVerify

		if S.APIKey != "" && S.APIKey != oldAPIKey {
			if existing := apikeyIdx.Get([]byte(S.APIKey)); existing != nil && string(existing) != id {
				return errors.New("APIKey already in use")
			}
		}

		data, err := bboltMarshal(SS)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if oldAPIKey != "" && oldAPIKey != S.APIKey {
			_ = apikeyIdx.Delete([]byte(oldAPIKey))
		}
		if S.APIKey != "" {
			if err := apikeyIdx.Put([]byte(S.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		RS = SS
		return nil
	})
	return RS, err
}

func BBolt_CreateDevice(D *types.Device) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(DEVICES_BUCKET))
		id := D.ID.String()

		if D.WireGuardKey != "" {
			wgIdx := tx.Bucket([]byte(DEVICES_WGKEY_INDEX))
			if existing := wgIdx.Get([]byte(D.WireGuardKey)); existing != nil && string(existing) != id {
				return errors.New("WireGuard key already in use")
			}
		}

		data, err := bboltMarshal(D)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		uid := D.UserID.String()
		if uid != "00000000-0000-0000-0000-000000000000" {
			if err := tx.Bucket([]byte(DEVICES_USERID_INDEX)).Put([]byte(uid+"/"+id), nil); err != nil {
				return err
			}
		}
		if D.WireGuardKey != "" {
			if err := tx.Bucket([]byte(DEVICES_WGKEY_INDEX)).Put([]byte(D.WireGuardKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func BBolt_CreateGroup(G *Group) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(GROUPS_BUCKET))
		id := G.ID.String()
		data, err := bboltMarshal(G)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func BBolt_CreateServer(S *types.Server) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		id := S.ID.String()
		if S.APIKey != "" {
			apikeyIdx := tx.Bucket([]byte(SERVERS_APIKEY_INDEX))
			if existing := apikeyIdx.Get([]byte(S.APIKey)); existing != nil && string(existing) != id {
				return errors.New("APIKey already in use")
			}
		}
		data, err := bboltMarshal(S)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if S.APIKey != "" {
			if err := tx.Bucket([]byte(SERVERS_APIKEY_INDEX)).Put([]byte(S.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func BBolt_FindServerByAPIKey(apiKey string) (*types.Server, error) {
	var found *types.Server
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		id := tx.Bucket([]byte(SERVERS_APIKEY_INDEX)).Get([]byte(apiKey))
		if id == nil {
			return nil
		}
		v := tx.Bucket([]byte(SERVERS_BUCKET)).Get(id)
		if v == nil {
			return nil
		}
		S := new(types.Server)
		if err := bboltUnmarshal(v, S); err != nil {
			return nil
		}
		if subtle.ConstantTimeCompare([]byte(S.APIKey), []byte(apiKey)) != 1 {
			return nil
		}
		found = S
		return nil
	})
	return found, err
}

func BBolt_FindAllServers() ([]*types.Server, error) {
	var out []*types.Server
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		return b.ForEach(func(_, v []byte) error {
			S := new(types.Server)
			if err := bboltUnmarshal(v, S); err != nil {
				return err
			}
			out = append(out, S)
			return nil
		})
	})
	return out, err
}

func BBolt_FindServerByID(ID string) (*types.Server, error) {
	var S *types.Server
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		v := b.Get([]byte(ID))
		if v == nil {
			return nil
		}
		S = new(types.Server)
		return bboltUnmarshal(v, S)
	})
	return S, err
}

func BBolt_FindDeviceByID(id string) (*types.Device, error) {
	var dev *types.Device
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(DEVICES_BUCKET))
		v := b.Get([]byte(id))
		if v == nil {
			return nil
		}
		dev = new(types.Device)
		return bboltUnmarshal(v, dev)
	})
	return dev, err
}

func BBolt_FindDeviceByWGKey(wgKey string) (*types.Device, error) {
	var dev *types.Device
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		devID := tx.Bucket([]byte(DEVICES_WGKEY_INDEX)).Get([]byte(wgKey))
		if devID == nil {
			return nil
		}
		v := tx.Bucket([]byte(DEVICES_BUCKET)).Get(devID)
		if v == nil {
			return nil
		}
		dev = new(types.Device)
		return bboltUnmarshal(v, dev)
	})
	return dev, err
}

func BBolt_findGroupByID(id string) (*Group, error) {
	var G *Group
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(GROUPS_BUCKET))
		v := b.Get([]byte(id))
		if v == nil {
			return nil
		}
		G = new(Group)
		return bboltUnmarshal(v, G)
	})
	return G, err
}

func BBolt_DeleteGroupByID(id string) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(GROUPS_BUCKET))
		return b.Delete([]byte(id))
	})
}

func BBolt_WipeUserConfirmCode(UF *USER_ENABLE_QUERY) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		uid := tx.Bucket([]byte(USERS_EMAIL_INDEX)).Get([]byte(UF.Email))
		if uid == nil {
			return errors.New("user not found")
		}
		b := tx.Bucket([]byte(USERS_BUCKET))
		v := b.Get(uid)
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.ConfirmCode = ""
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put(uid, data)
	})
}

func BBolt_UserActivateKey(SubExpiration time.Time, Key *LicenseKey, userID string) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		v := b.Get([]byte(userID))
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

func BBolt_AddToGroup(groupID, typeID, objType string) error {
	bucket := ""
	switch objType {
	case "user":
		bucket = USERS_BUCKET
	case "server":
		bucket = SERVERS_BUCKET
	case "device":
		bucket = DEVICES_BUCKET
	default:
		return fmt.Errorf("unknown type")
	}
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(typeID))
		if v == nil {
			return errors.New("object not found")
		}
		var err error
		switch objType {
		case "device":
			D := new(types.Device)
			_ = bboltUnmarshal(v, D)
			groups := uuidSliceToString(D.Groups)
			if !contains(groups, groupID) {
				groups = append(groups, groupID)
				D.Groups = stringSliceToUUID(groups)
			}
			v, err = bboltMarshal(D)
			if err != nil {
				return err
			}
			return b.Put([]byte(typeID), v)
		case "user":
			U := new(User)
			_ = bboltUnmarshal(v, U)
			groups := uuidSliceToString(U.Groups)
			if !contains(groups, groupID) {
				groups = append(groups, groupID)
				U.Groups = stringSliceToUUID(groups)
			}
			v, err = bboltMarshal(U)
		case "server":
			S := new(types.Server)
			_ = bboltUnmarshal(v, S)
			groups := uuidSliceToString(S.Groups)
			if !contains(groups, groupID) {
				groups = append(groups, groupID)
				S.Groups = stringSliceToUUID(groups)
			}
			v, err = bboltMarshal(S)
		}
		if err != nil {
			return err
		}
		return b.Put([]byte(typeID), v)
	})
}

func BBolt_RemoveFromGroup(groupID, typeID, objType string) error {
	bucket := ""
	switch objType {
	case "user":
		bucket = USERS_BUCKET
	case "server":
		bucket = SERVERS_BUCKET
	case "device":
		bucket = DEVICES_BUCKET
	default:
		return fmt.Errorf("unknown type")
	}
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(typeID))
		if v == nil {
			return errors.New("object not found")
		}
		var err error
		switch objType {
		case "user":
			U := new(User)
			_ = bboltUnmarshal(v, U)
			groups := uuidSliceToString(U.Groups)
			groups = removeString(groups, groupID)
			U.Groups = stringSliceToUUID(groups)
			v, err = bboltMarshal(U)
		case "server":
			S := new(types.Server)
			_ = bboltUnmarshal(v, S)
			groups := uuidSliceToString(S.Groups)
			groups = removeString(groups, groupID)
			S.Groups = stringSliceToUUID(groups)
			v, err = bboltMarshal(S)
		case "device":
			D := new(types.Device)
			_ = bboltUnmarshal(v, D)
			groups := uuidSliceToString(D.Groups)
			groups = removeString(groups, groupID)
			D.Groups = stringSliceToUUID(groups)
			v, err = bboltMarshal(D)
		}
		if err != nil {
			return err
		}
		return b.Put([]byte(typeID), v)
	})
}

func BBolt_findGroups() ([]*Group, error) {
	gl := make([]*Group, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(GROUPS_BUCKET))
		c := b.Cursor()
		for _, v := c.First(); v != nil; _, v = c.Next() {
			D := new(Group)
			if err := bboltUnmarshal(v, D); err == nil {
				gl = append(gl, D)
			}
		}
		return nil
	})
	return gl, err
}

func contains(slice []string, s string) bool {
	return slices.Contains(slice, s)
}

func removeString(slice []string, s string) []string {
	res := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != s {
			res = append(res, v)
		}
	}
	return res
}

func uuidSliceToString(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func stringSliceToUUID(slice []string) []uuid.UUID {
	var out []uuid.UUID
	for _, s := range slice {
		id, err := uuid.Parse(s)
		if err == nil {
			out = append(out, id)
		}
	}
	return out
}
