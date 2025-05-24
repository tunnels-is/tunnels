package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"slices"

	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	BBoltDB *gobolt.DB
	bboltMu sync.RWMutex
)

const (
	USERS_BUCKET   = "users"
	DEVICES_BUCKET = "devices"
	ORGS_BUCKET    = "orgs"
	GROUPS_BUCKET  = "groups"
	SERVERS_BUCKET = "servers"
)

func ConnectToBBoltDB(path string) (err error) {
	BBoltDB, err = gobolt.Open(path, 0600, &gobolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		buckets := []string{USERS_BUCKET, DEVICES_BUCKET, ORGS_BUCKET, GROUPS_BUCKET, SERVERS_BUCKET}
		for _, b := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(b))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Helper: marshal/unmarshal
func bboltMarshal(v any) ([]byte, error)      { return json.Marshal(v) }
func bboltUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

// Delete a device by string ID
func BBolt_DeleteDeviceByID(id string) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(DEVICES_BUCKET))
		return b.Delete([]byte(id))
	})
}

// Update a device (by ID in D.ID)
func BBolt_UpdateDevice(D *types.Device) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(DEVICES_BUCKET))
		id := objectIDToString(D.ID)
		data, err := bboltMarshal(D)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

// Get devices with limit/offset (inefficient scan)
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

// User struct must be defined somewhere, using same as dbwrapper.go
// type User struct { ... }

// Get users with limit/offset
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

// Find user by APIKey
func BBolt_findUserByAPIKey(Key string) (*User, error) {
	var found *User
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		c := b.Cursor()
		for _, v := c.First(); v != nil; _, v = c.Next() {
			U := new(User)
			if err := bboltUnmarshal(v, U); err == nil && U.APIKey == Key {
				found = U
				break
			}
		}
		return nil
	})
	return found, err
}

// Find user by ID (string)
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

// Create user
func BBolt_CreateUser(U *User) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		id := objectIDToString(U.ID)
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

// Find user by email
func BBolt_findUserByEmail(Email string) (*User, error) {
	var found *User
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		c := b.Cursor()
		for _, v := c.First(); v != nil; _, v = c.Next() {
			U := new(User)
			if err := bboltUnmarshal(v, U); err == nil && U.Email == Email {
				found = U
				break
			}
		}
		return nil
	})
	return found, err
}

// Update user device tokens
func BBolt_updateUserDeviceTokens(TU *UPDATE_USER_TOKENS) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		id := objectIDToString(TU.ID)
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

// Update user subscription expiration time
func BBolt_updateUserSubTime(u *User) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			U := new(User)
			if err := bboltUnmarshal(v, U); err == nil && U.Email == u.Email {
				U.SubExpiration = u.SubExpiration
				data, err := bboltMarshal(U)
				if err != nil {
					return err
				}
				return b.Put(k, data)
			}
		}
		return errors.New("user not found")
	})
}

// Update user (APIKey, AdditionalInformation)
func BBolt_updateUser(UF *USER_UPDATE_FORM) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		id := objectIDToString(UF.UID)
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.APIKey = UF.APIKey
		U.AdditionalInformation = UF.AdditionalInformation
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

// Toggle user subscription status
func BBolt_toggleUserSubscriptionStatus(UF *USER_UPDATE_SUB_FORM) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			U := new(User)
			if err := bboltUnmarshal(v, U); err == nil && U.Email == UF.Email {
				//U.CancelSub = UF.Disable
				data, err := bboltMarshal(U)
				if err != nil {
					return err
				}
				return b.Put(k, data)
			}
		}
		return errors.New("user not found")
	})
}

// Update two-factor codes for user
func BBolt_userUpdateTwoFactorCodes(TFP *TWO_FACTOR_DB_PACKAGE) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		v := b.Get([]byte(objectIDToString(TFP.UID)))
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
		return b.Put([]byte(objectIDToString(U.ID)), data)
	})
}

// Reset user password
func BBolt_userResetPassword(user *User) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		v := b.Get([]byte(objectIDToString(user.ID)))
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.Password = user.Password
		U.Tokens = []*DeviceToken{}
		U.ResetCode = ""
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put([]byte(objectIDToString(U.ID)), data)
	})
}

// Update user reset code
func BBolt_userUpdateResetCode(user *User) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		v := b.Get([]byte(objectIDToString(user.ID)))
		if v == nil {
			return errors.New("user not found")
		}
		U := new(User)
		if err := bboltUnmarshal(v, U); err != nil {
			return err
		}
		U.ResetCode = user.ResetCode
		U.LastResetRequest = user.LastResetRequest
		data, err := bboltMarshal(U)
		if err != nil {
			return err
		}
		return b.Put([]byte(objectIDToString(U.ID)), data)
	})
}

// Find servers without groups
func BBolt_FindServersWithoutGroups(limit, offset int64) ([]*types.Server, error) {
	DL := make([]*types.Server, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			S := new(types.Server)
			if err := bboltUnmarshal(v, S); err == nil {
				if S.Groups == nil || len(S.Groups) == 0 {
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

// Find servers by group IDs
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
				for _, gid := range objectIDSliceToString(S.Groups) {
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

// Find entities by group ID and type
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
					if slices.Contains(objectIDSliceToString(E.Groups), id) {
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
					if slices.Contains(objectIDSliceToString(E.Groups), id) {
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
					if slices.Contains(objectIDSliceToString(E.Groups), id) {
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

// Update group
func BBolt_UpdateGroup(G *Group) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(GROUPS_BUCKET))
		id := objectIDToString(G.ID)
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

// Update server
func BBolt_UpdateServer(S *types.Server) (*types.Server, error) {
	var RS *types.Server
	err := BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		id := objectIDToString(S.ID)
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
		SS.DataPort = S.DataPort
		SS.PubKey = S.PubKey
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

// Create device
func BBolt_CreateDevice(D *types.Device) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(DEVICES_BUCKET))
		id := objectIDToString(D.ID)
		data, err := bboltMarshal(D)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

// Create group
func BBolt_CreateGroup(G *Group) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(GROUPS_BUCKET))
		id := objectIDToString(G.ID)
		data, err := bboltMarshal(G)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

// Create server
func BBolt_CreateServer(S *types.Server) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(SERVERS_BUCKET))
		id := objectIDToString(S.ID)
		data, err := bboltMarshal(S)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

// Find server by ID
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

// Find device by ID
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

// Find group by ID
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

// Delete group by ID
func BBolt_DeleteGroupByID(id string) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(GROUPS_BUCKET))
		return b.Delete([]byte(id))
	})
}

// Wipe user confirm code
func BBolt_WipeUserConfirmCode(UF *USER_ENABLE_QUERY) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(USERS_BUCKET))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			U := new(User)
			if err := bboltUnmarshal(v, U); err == nil && U.Email == UF.Email {
				U.ConfirmCode = ""
				data, err := bboltMarshal(U)
				if err != nil {
					return err
				}
				return b.Put(k, data)
			}
		}
		return errors.New("user not found")
	})
}

// User activate key (update sub expiration, key, etc)
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
		id := objectIDToString(U.ID)
		return b.Put([]byte(id), data)
	})
}

// Add to group
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
			err = bboltUnmarshal(v, D)
			groups := objectIDSliceToString(D.Groups)
			if !contains(groups, groupID) {
				groups = append(groups, groupID)
				D.Groups = stringSliceToObjectID(groups)
			}
			v, err = bboltMarshal(D)
			if err != nil {
				return err
			}
			return b.Put([]byte(typeID), v)
		case "user":
			U := new(User)
			err = bboltUnmarshal(v, U)
			groups := objectIDSliceToString(U.Groups)
			if !contains(groups, groupID) {
				groups = append(groups, groupID)
				U.Groups = stringSliceToObjectID(groups)
			}
			v, err = bboltMarshal(U)
		case "server":
			S := new(types.Server)
			err = bboltUnmarshal(v, S)
			groups := objectIDSliceToString(S.Groups)
			if !contains(groups, groupID) {
				groups = append(groups, groupID)
				S.Groups = stringSliceToObjectID(groups)
			}
			v, err = bboltMarshal(S)
		}
		if err != nil {
			return err
		}
		return b.Put([]byte(typeID), v)
	})
}

// Remove from group
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
			err = bboltUnmarshal(v, U)
			groups := objectIDSliceToString(U.Groups)
			groups = removeString(groups, groupID)
			U.Groups = stringSliceToObjectID(groups)
			v, err = bboltMarshal(U)
		case "server":
			S := new(types.Server)
			err = bboltUnmarshal(v, S)
			groups := objectIDSliceToString(S.Groups)
			groups = removeString(groups, groupID)
			S.Groups = stringSliceToObjectID(groups)
			v, err = bboltMarshal(S)
		case "device":
			D := new(types.Device)
			err = bboltUnmarshal(v, D)
			groups := objectIDSliceToString(D.Groups)
			groups = removeString(groups, groupID)
			D.Groups = stringSliceToObjectID(groups)
			v, err = bboltMarshal(D)
		}
		if err != nil {
			return err
		}
		return b.Put([]byte(typeID), v)
	})
}

// Find all groups
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

// Helper functions
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

// Helper: convert primitive.ObjectID to string and vice versa
func objectIDToString(id interface{}) string {
	switch v := id.(type) {
	case string:
		return v
	case [12]byte:
		return primitive.ObjectID(v).Hex()
	case primitive.ObjectID:
		return v.Hex()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func stringToObjectID(id string) primitive.ObjectID {
	objID, _ := primitive.ObjectIDFromHex(id)
	return objID
}

// Helper: convert []primitive.ObjectID <-> []string
func objectIDSliceToString(slice interface{}) []string {
	var out []string
	switch v := slice.(type) {
	case []string:
		return v
	case []primitive.ObjectID:
		for _, id := range v {
			out = append(out, id.Hex())
		}
	}
	return out
}

func stringSliceToObjectID(slice []string) []primitive.ObjectID {
	var out []primitive.ObjectID
	for _, s := range slice {
		id, err := primitive.ObjectIDFromHex(s)
		if err == nil {
			out = append(out, id)
		}
	}
	return out
}
