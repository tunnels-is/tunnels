package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

var (
	errDeviceIPInUse    = errors.New("WireGuard IP already in use")
	errDeviceIPReserved = errors.New("WireGuard IP is reserved")
	errDeviceIPv6InUse  = errors.New("WireGuard IPv6 already in use")
	errEmailRegistered  = errors.New("email already registered")
)

var db *gobolt.DB

const (
	bucketUsers              = "users"
	bucketUsersEmailIndex    = "users_by_email"
	bucketUsersAPIKeyIndex   = "users_by_apikey"
	bucketDevices            = "devices"
	bucketDevicesUserIDIndex = "devices_by_user_id"
	bucketDevicesWGKeyIndex  = "devices_by_wg_key"
	bucketOrgs               = "orgs"
	bucketGroups             = "groups"
	bucketServers            = "servers"
	bucketServersAPIKeyIndex = "servers_by_apikey"
	bucketWANs               = "wans"
	bucketMeshGroups         = "meshgroups"
)

func openDB(path string) (err error) {
	if st, statErr := os.Stat(path); statErr == nil {
		// Fail only if world-accessible. Group-readable (0640) is common
		// for a dedicated service account and must not block production.
		if mode := st.Mode().Perm(); mode&0o007 != 0 {
			return fmt.Errorf("database %s is world-accessible (mode %o, want 0600)", path, mode)
		}
	}
	db, err = gobolt.Open(path, 0o600, &gobolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return err
	}
	return db.Update(func(tx *gobolt.Tx) error {
		if err := ensureBuckets(tx); err != nil {
			return err
		}
		if err := backfillUserIndexes(tx); err != nil {
			return err
		}
		if err := backfillDeviceIndexes(tx); err != nil {
			return err
		}
		if err := backfillServerIndexes(tx); err != nil {
			return err
		}
		return nil
	})
}

func ensureBuckets(tx *gobolt.Tx) error {
	buckets := []string{
		bucketUsers, bucketUsersEmailIndex, bucketUsersAPIKeyIndex,
		bucketDevices, bucketDevicesUserIDIndex, bucketDevicesWGKeyIndex,
		bucketOrgs, bucketGroups, bucketServers, bucketServersAPIKeyIndex,
		bucketWANs, bucketMeshGroups,
	}
	for _, b := range buckets {
		_, err := tx.CreateBucketIfNotExists([]byte(b))
		if err != nil {
			return err
		}
	}
	return nil
}

func backfillUserIndexes(tx *gobolt.Tx) error {
	users := tx.Bucket([]byte(bucketUsers))
	emailIdx := tx.Bucket([]byte(bucketUsersEmailIndex))
	apikeyIdx := tx.Bucket([]byte(bucketUsersAPIKeyIndex))
	uc := users.Cursor()
	type hashedUser struct {
		id     []byte
		user   *User
		oldKey string
	}
	var rewritten []hashedUser
	for k, v := uc.First(); k != nil; k, v = uc.Next() {
		user := new(User)
		if err := bboltUnmarshal(v, user); err != nil {
			continue
		}
		if user.Email != "" {
			if err := emailIdx.Put([]byte(user.Email), k); err != nil {
				return err
			}
		}
		oldKey, changed := hashUserSecrets(user)
		if changed {
			id := append([]byte(nil), k...)
			rewritten = append(rewritten, hashedUser{id: id, user: user, oldKey: oldKey})
			continue
		}
		if user.APIKey != "" {
			if err := apikeyIdx.Put([]byte(user.APIKey), k); err != nil {
				return err
			}
		}
	}
	for _, item := range rewritten {
		data, err := bboltMarshal(item.user)
		if err != nil {
			return err
		}
		if err := users.Put(item.id, data); err != nil {
			return err
		}
		if item.oldKey != "" {
			if err := apikeyIdx.Delete([]byte(item.oldKey)); err != nil {
				return err
			}
		}
		if item.user.APIKey != "" {
			if err := apikeyIdx.Put([]byte(item.user.APIKey), item.id); err != nil {
				return err
			}
		}
	}
	return sweepPlainAPIKeyIndex(tx, apikeyIdx, bucketUsers, func(u *User) string { return u.APIKey })
}

func backfillDeviceIndexes(tx *gobolt.Tx) error {
	devices := tx.Bucket([]byte(bucketDevices))
	devUserIdx := tx.Bucket([]byte(bucketDevicesUserIDIndex))
	dc := devices.Cursor()
	for k, v := dc.First(); k != nil; k, v = dc.Next() {
		device := new(types.Device)
		if err := bboltUnmarshal(v, device); err != nil {
			continue
		}
		uid := device.UserID.String()
		if uid != "00000000-0000-0000-0000-000000000000" {
			compositeKey := []byte(uid + "/" + string(k))
			if err := devUserIdx.Put(compositeKey, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func backfillServerIndexes(tx *gobolt.Tx) error {
	servers := tx.Bucket([]byte(bucketServers))
	srvApikeyIdx := tx.Bucket([]byte(bucketServersAPIKeyIndex))
	sc := servers.Cursor()
	type hashedServer struct {
		id     []byte
		server *types.Server
		oldKey string
	}
	var rewritten []hashedServer
	for k, v := sc.First(); k != nil; k, v = sc.Next() {
		server := new(types.Server)
		if err := bboltUnmarshal(v, server); err != nil {
			continue
		}
		if server.APIKey != "" && !storedSecretIsHash(server.APIKey) {
			id := append([]byte(nil), k...)
			rewritten = append(rewritten, hashedServer{id: id, server: server, oldKey: server.APIKey})
			continue
		}
		if server.APIKey != "" {
			if err := srvApikeyIdx.Put([]byte(server.APIKey), k); err != nil {
				return err
			}
		}
	}
	for _, item := range rewritten {
		item.server.APIKey = hashSecret(item.oldKey)
		data, err := bboltMarshal(item.server)
		if err != nil {
			return err
		}
		if err := servers.Put(item.id, data); err != nil {
			return err
		}
		if err := srvApikeyIdx.Delete([]byte(item.oldKey)); err != nil {
			return err
		}
		if err := srvApikeyIdx.Put([]byte(item.server.APIKey), item.id); err != nil {
			return err
		}
	}
	return sweepPlainAPIKeyIndex(tx, srvApikeyIdx, bucketServers, func(s *types.Server) string { return s.APIKey })
}

// sweepPlainAPIKeyIndex drops an index entry whose key is still the raw secret
// after the record itself stores the hash.
func sweepPlainAPIKeyIndex[T any](tx *gobolt.Tx, idx *gobolt.Bucket, recordBucket string, apiKey func(*T) string) error {
	if idx == nil {
		return nil
	}
	records := tx.Bucket([]byte(recordBucket))
	var stale [][]byte
	c := idx.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		if storedSecretIsHash(string(k)) || len(v) == 0 {
			continue
		}
		raw := records.Get(v)
		if raw == nil {
			continue
		}
		item := new(T)
		if err := bboltUnmarshal(raw, item); err != nil {
			continue
		}
		stored := apiKey(item)
		if stored != "" && stored != string(k) && secretMatches(stored, string(k)) {
			stale = append(stale, append([]byte(nil), k...))
		}
	}
	for _, k := range stale {
		if err := idx.Delete(k); err != nil {
			return err
		}
	}
	return nil
}

func bboltMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func bboltUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

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
