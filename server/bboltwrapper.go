package main

import (
	"bytes"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

var BBoltDB *gobolt.DB

const (
	USERS_BUCKET              = "users"
	USERS_EMAIL_INDEX         = "users_by_email"
	USERS_APIKEY_INDEX        = "users_by_apikey"
	DEVICES_BUCKET            = "devices"
	DEVICES_USERID_INDEX      = "devices_by_user_id"
	DEVICES_WGKEY_INDEX       = "devices_by_wg_key"
	ORGS_BUCKET               = "orgs"
	GROUPS_BUCKET             = "groups"
	SERVERS_BUCKET            = "servers"
	WG_SERVER_CONFIGS_BUCKET  = "wg_server_configs"
	WGCONFIGS_APIKEY_INDEX    = "wg_configs_by_apikey"
	NETWORKS_BUCKET           = "networks"
	NETWORKS_ID_INDEX         = "networks_by_id"
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
			ORGS_BUCKET, GROUPS_BUCKET, SERVERS_BUCKET,
			WG_SERVER_CONFIGS_BUCKET, WGCONFIGS_APIKEY_INDEX,
			NETWORKS_BUCKET, NETWORKS_ID_INDEX,
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

		// Backfill wg_configs_by_apikey index.
		wgConfigs := tx.Bucket([]byte(WG_SERVER_CONFIGS_BUCKET))
		wgApikeyIdx := tx.Bucket([]byte(WGCONFIGS_APIKEY_INDEX))
		wc := wgConfigs.Cursor()
		for k, v := wc.First(); k != nil; k, v = wc.Next() {
			cfg := new(types.WGServerConfig)
			if err := bboltUnmarshal(v, cfg); err != nil {
				continue
			}
			if cfg.APIKey != "" {
				if err := wgApikeyIdx.Put([]byte(cfg.APIKey), k); err != nil {
					return err
				}
			}
		}

		// Backfill networks_by_id index.
		networks := tx.Bucket([]byte(NETWORKS_BUCKET))
		netIdIdx := tx.Bucket([]byte(NETWORKS_ID_INDEX))
		nc := networks.Cursor()
		for k, v := nc.First(); k != nil; k, v = nc.Next() {
			n := new(Network)
			if err := bboltUnmarshal(v, n); err != nil {
				continue
			}
			idStr := n.ID.String()
			if idStr != "00000000-0000-0000-0000-000000000000" {
				if err := netIdIdx.Put([]byte(idStr), k); err != nil {
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
		U.AdditionalInformation = UF.AdditionalInformation
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
		U.IsManager = UF.IsManager
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

func BBolt_toggleUserSubscriptionStatus(UF *USER_UPDATE_SUB_FORM) error {
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
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put(uid, data)
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

func BBolt_FindServersByGroups(groups []string, limit, offset int64) ([]*types.Server, error) {
	DL := make([]*types.Server, 0)
	groupSet := make(map[string]struct{})
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
				for _, gid := range uuidSliceToString(S.Groups) {
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
		id := S.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("server not found")
		}
		SS := new(types.Server)
		if err := bboltUnmarshal(v, SS); err != nil {
			return err
		}
		SS.Tag = S.Tag
		SS.Country = S.Country
		SS.IP = S.IP
		SS.Port = S.Port
		SS.WireGuardPort = S.WireGuardPort
		SS.WireGuardPubKey = S.WireGuardPubKey
		data, err := bboltMarshal(SS)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
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
		data, err := bboltMarshal(S)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func BBolt_SetServerWGSubnet(id, subnet string) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("server not found")
		}
		S := new(types.Server)
		if err := bboltUnmarshal(v, S); err != nil {
			return err
		}
		S.WireGuardSubnet = subnet
		data, err := bboltMarshal(S)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
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

func uuidToString(id uuid.UUID) string {
	return id.String()
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

func BBolt_CreateWGServerConfig(cfg *types.WGServerConfig) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(WG_SERVER_CONFIGS_BUCKET))
		id := cfg.ID.String()
		data, err := bboltMarshal(cfg)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if cfg.APIKey != "" {
			if err := tx.Bucket([]byte(WGCONFIGS_APIKEY_INDEX)).Put([]byte(cfg.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func BBolt_FindWGServerConfigByID(id string) (*types.WGServerConfig, error) {
	var cfg *types.WGServerConfig
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(WG_SERVER_CONFIGS_BUCKET))
		v := b.Get([]byte(id))
		if v == nil {
			return nil
		}
		cfg = new(types.WGServerConfig)
		return bboltUnmarshal(v, cfg)
	})
	return cfg, err
}

func BBolt_FindWGServerConfigByAPIKey(apiKey string) (*types.WGServerConfig, error) {
	var found *types.WGServerConfig
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		cfgID := tx.Bucket([]byte(WGCONFIGS_APIKEY_INDEX)).Get([]byte(apiKey))
		if cfgID == nil {
			return nil
		}
		v := tx.Bucket([]byte(WG_SERVER_CONFIGS_BUCKET)).Get(cfgID)
		if v == nil {
			return nil
		}
		cfg := new(types.WGServerConfig)
		if err := bboltUnmarshal(v, cfg); err != nil {
			return nil
		}
		if subtle.ConstantTimeCompare([]byte(cfg.APIKey), []byte(apiKey)) != 1 {
			return nil
		}
		found = cfg
		return nil
	})
	return found, err
}

func BBolt_UpdateWGServerConfig(cfg *types.WGServerConfig) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(WG_SERVER_CONFIGS_BUCKET))
		id := cfg.ID.String()
		wgApikeyIdx := tx.Bucket([]byte(WGCONFIGS_APIKEY_INDEX))

		// Remove old apikey index entry if key changed.
		if old := b.Get([]byte(id)); old != nil {
			oldCfg := new(types.WGServerConfig)
			if err := bboltUnmarshal(old, oldCfg); err == nil && oldCfg.APIKey != cfg.APIKey && oldCfg.APIKey != "" {
				_ = wgApikeyIdx.Delete([]byte(oldCfg.APIKey))
			}
		}

		data, err := bboltMarshal(cfg)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if cfg.APIKey != "" {
			if err := wgApikeyIdx.Put([]byte(cfg.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func BBolt_SetServerWGConfigID(serverID string, wgCfg *types.WGServerConfig, pubKey, subnet, subnet6 string) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		v := b.Get([]byte(serverID))
		if v == nil {
			return errors.New("server not found")
		}
		S := new(types.Server)
		if err := bboltUnmarshal(v, S); err != nil {
			return err
		}
		S.WGConfigID = wgCfg.ID
		S.WireGuardSubnet = subnet
		S.WireGuardSubnet6 = subnet6
		S.WireGuardPubKey = pubKey
		S.WireGuardPort = fmt.Sprintf("%d", wgCfg.WireGuardPort)
		data, err := bboltMarshal(S)
		if err != nil {
			return err
		}
		return b.Put([]byte(serverID), data)
	})
}

func BBolt_CountNetworks() (int64, error) {
	var count int64
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(NETWORKS_BUCKET))
		if b == nil {
			return nil
		}
		count = int64(b.Stats().KeyN)
		return nil
	})
	return count, err
}

// networkKey converts a CIDR string to a big-endian IP key for bbolt storage.
// Returns a 4-byte key for IPv4 or a 16-byte key for IPv6.
func networkKey(cidr string) ([]byte, error) {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	if v4 := ip.To4(); v4 != nil {
		key := make([]byte, 4)
		binary.BigEndian.PutUint32(key, binary.BigEndian.Uint32(v4))
		return key, nil
	}
	v6 := ip.To16()
	if v6 == nil {
		return nil, fmt.Errorf("invalid IP address: %s", cidr)
	}
	key := make([]byte, 16)
	copy(key, v6)
	return key, nil
}

func BBolt_CreateNetworksBatch(networks []*Network) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(NETWORKS_BUCKET))
		netIdIdx := tx.Bucket([]byte(NETWORKS_ID_INDEX))
		for _, n := range networks {
			key, err := networkKey(n.CIDR)
			if err != nil {
				return err
			}
			data, err := bboltMarshal(n)
			if err != nil {
				return err
			}
			if err := b.Put(key, data); err != nil {
				return err
			}
			idStr := n.ID.String()
			if idStr != "00000000-0000-0000-0000-000000000000" {
				if err := netIdIdx.Put([]byte(idStr), key); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func BBolt_GetNetworks(limit, offset int64) ([]*Network, error) {
	NL := make([]*Network, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(NETWORKS_BUCKET))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if skipped < offset {
				skipped++
				continue
			}
			if int64(len(NL)) >= limit {
				break
			}
			n := new(Network)
			if err := bboltUnmarshal(v, n); err == nil {
				NL = append(NL, n)
			}
		}
		return nil
	})
	return NL, err
}

func BBolt_FindNetworkByID(id uuid.UUID) (*Network, error) {
	var n *Network
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		netKey := tx.Bucket([]byte(NETWORKS_ID_INDEX)).Get([]byte(id.String()))
		if netKey == nil {
			return nil
		}
		v := tx.Bucket([]byte(NETWORKS_BUCKET)).Get(netKey)
		if v == nil {
			return nil
		}
		n = new(Network)
		return bboltUnmarshal(v, n)
	})
	return n, err
}

func BBolt_UpdateNetwork(n *Network) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(NETWORKS_BUCKET))
		key, err := networkKey(n.CIDR)
		if err != nil {
			return err
		}
		data, err := bboltMarshal(n)
		if err != nil {
			return err
		}
		if err := b.Put(key, data); err != nil {
			return err
		}
		idStr := n.ID.String()
		if idStr != "00000000-0000-0000-0000-000000000000" {
			if err := tx.Bucket([]byte(NETWORKS_ID_INDEX)).Put([]byte(idStr), key); err != nil {
				return err
			}
		}
		return nil
	})
}

func BBolt_ListWGServerConfigs() ([]*types.WGServerConfig, error) {
	configs := make([]*types.WGServerConfig, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(WG_SERVER_CONFIGS_BUCKET))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			cfg := new(types.WGServerConfig)
			if err := bboltUnmarshal(v, cfg); err == nil {
				configs = append(configs, cfg)
			}
		}
		return nil
	})
	return configs, err
}
