package main

import (
	"crypto/subtle"
	"errors"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func findServersWithoutGroups(limit, offset int64) ([]*types.Server, error) {
	DL := make([]*types.Server, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
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

func findServersByGroups(groups []uuid.UUID, limit, offset int64) ([]*types.Server, error) {
	DL := make([]*types.Server, 0)
	groupSet := make(map[uuid.UUID]struct{})
	for _, g := range groups {
		groupSet[g] = struct{}{}
	}
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
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

func updateServer(S *types.Server) (*types.Server, error) {
	var RS *types.Server
	err := db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		apikeyIdx := tx.Bucket([]byte(bucketServersAPIKeyIndex))
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
		SS.InfraTag = S.InfraTag
		SS.Country = S.Country
		SS.IP = S.IP
		SS.Port = S.Port
		SS.APIKey = S.APIKey
		SS.WireGuardPort = S.WireGuardPort
		if oldAPIKey != S.APIKey {
			SS.WireGuardPubKey = ""
		}
		SS.WireGuardIface = S.WireGuardIface
		SS.WireGuardSubnet = S.WireGuardSubnet
		SS.WireGuardSubnet6 = S.WireGuardSubnet6
		SS.InternetIface = S.InternetIface
		SS.InsecureSkipVerify = S.InsecureSkipVerify
		SS.EnableFirewall = S.EnableFirewall
		SS.WANID = S.WANID
		SS.MeshGroupID = S.MeshGroupID
		SS.WireGuardMeshPort = S.WireGuardMeshPort

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

func setServerWireGuardPubKey(id uuid.UUID, pubKey string) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		key := []byte(id.String())
		v := b.Get(key)
		if v == nil {
			return errors.New("server not found")
		}
		SS := new(types.Server)
		if err := bboltUnmarshal(v, SS); err != nil {
			return err
		}
		SS.WireGuardPubKey = pubKey
		data, err := bboltMarshal(SS)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
}

func createServer(S *types.Server) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))

		S.WAN = nil
		id := S.ID.String()
		if S.APIKey != "" {
			apikeyIdx := tx.Bucket([]byte(bucketServersAPIKeyIndex))
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
			if err := tx.Bucket([]byte(bucketServersAPIKeyIndex)).Put([]byte(S.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func findServerByAPIKey(apiKey string) (*types.Server, error) {
	var found *types.Server
	err := db.View(func(tx *gobolt.Tx) error {
		id := tx.Bucket([]byte(bucketServersAPIKeyIndex)).Get([]byte(apiKey))
		if id == nil {
			return nil
		}
		v := tx.Bucket([]byte(bucketServers)).Get(id)
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

func findAllServers(limit, offset int64) ([]*types.Server, error) {
	out := make([]*types.Server, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if skipped < offset {
				skipped++
				continue
			}
			if int64(len(out)) >= limit {
				break
			}
			S := new(types.Server)
			if err := bboltUnmarshal(v, S); err != nil {
				return err
			}
			out = append(out, S)
		}
		return nil
	})
	return out, err
}

func findServerByID(ID uuid.UUID) (*types.Server, error) {
	idStr := ID.String()
	var S *types.Server
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		v := b.Get([]byte(idStr))
		if v == nil {
			return nil
		}
		S = new(types.Server)
		return bboltUnmarshal(v, S)
	})
	return S, err
}

func deleteServerByID(id uuid.UUID) error {
	idStr := id.String()
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		v := b.Get([]byte(idStr))
		if v != nil {
			S := new(types.Server)
			if err := bboltUnmarshal(v, S); err == nil {
				if S.APIKey != "" {
					_ = tx.Bucket([]byte(bucketServersAPIKeyIndex)).Delete([]byte(S.APIKey))
				}
			}
		}

		deleteDevicesTx(tx, func(devID string, D *types.Device) bool {
			return D.ServerID.String() == idStr
		})
		return b.Delete([]byte(idStr))
	})
}
