package main

import (
	"crypto/subtle"
	"errors"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func findServersWithoutGroups(limit, offset int64) ([]*types.Server, error) {
	servers := make([]*types.Server, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			server := new(types.Server)
			if err := bboltUnmarshal(v, server); err == nil {
				if len(server.Groups) == 0 {
					if skipped < offset {
						skipped++
						continue
					}
					if int64(len(servers)) >= limit {
						break
					}
					servers = append(servers, server)
				}
			}
		}
		return nil
	})
	return servers, err
}

func findServersByGroups(groups []uuid.UUID, limit, offset int64) ([]*types.Server, error) {
	servers := make([]*types.Server, 0)
	groupSet := make(map[uuid.UUID]struct{})
	for _, g := range groups {
		groupSet[g] = struct{}{}
	}
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			server := new(types.Server)
			if err := bboltUnmarshal(v, server); err == nil {
				for _, gid := range server.Groups {
					if _, ok := groupSet[gid]; ok {
						if skipped < offset {
							skipped++
							continue
						}
						if int64(len(servers)) >= limit {
							break
						}
						servers = append(servers, server)
						break
					}
				}
			}
		}
		return nil
	})
	return servers, err
}

func updateServer(server *types.Server) (*types.Server, error) {
	var result *types.Server
	err := db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		apikeyIdx := tx.Bucket([]byte(bucketServersAPIKeyIndex))
		id := server.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("server not found")
		}
		existing := new(types.Server)
		if err := bboltUnmarshal(v, existing); err != nil {
			return err
		}
		oldAPIKey := existing.APIKey
		existing.Tag = server.Tag
		existing.InfraTag = server.InfraTag
		existing.Country = server.Country
		existing.IP = server.IP
		existing.Port = server.Port
		existing.APIKey = server.APIKey
		existing.WireGuardPort = server.WireGuardPort
		if oldAPIKey != server.APIKey {
			existing.WireGuardPubKey = ""
		}
		existing.WireGuardIface = server.WireGuardIface
		existing.WireGuardSubnet = server.WireGuardSubnet
		existing.WireGuardSubnet6 = server.WireGuardSubnet6
		existing.InternetIface = server.InternetIface
		existing.InsecureSkipVerify = server.InsecureSkipVerify
		existing.EnableFirewall = server.EnableFirewall
		existing.WANID = server.WANID
		existing.MeshGroupID = server.MeshGroupID
		existing.WireGuardMeshPort = server.WireGuardMeshPort

		if server.APIKey != "" && server.APIKey != oldAPIKey {
			if existing := apikeyIdx.Get([]byte(server.APIKey)); existing != nil && string(existing) != id {
				return errors.New("APIKey already in use")
			}
		}

		data, err := bboltMarshal(existing)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if oldAPIKey != "" && oldAPIKey != server.APIKey {
			_ = apikeyIdx.Delete([]byte(oldAPIKey))
		}
		if server.APIKey != "" {
			if err := apikeyIdx.Put([]byte(server.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		result = existing
		return nil
	})
	return result, err
}

func setServerWireGuardPubKey(id uuid.UUID, pubKey string) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		key := []byte(id.String())
		v := b.Get(key)
		if v == nil {
			return errors.New("server not found")
		}
		existing := new(types.Server)
		if err := bboltUnmarshal(v, existing); err != nil {
			return err
		}
		existing.WireGuardPubKey = pubKey
		data, err := bboltMarshal(existing)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
}

func createServer(server *types.Server) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))

		server.WAN = nil
		id := server.ID.String()
		if server.APIKey != "" {
			apikeyIdx := tx.Bucket([]byte(bucketServersAPIKeyIndex))
			if existing := apikeyIdx.Get([]byte(server.APIKey)); existing != nil && string(existing) != id {
				return errors.New("APIKey already in use")
			}
		}
		data, err := bboltMarshal(server)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if server.APIKey != "" {
			if err := tx.Bucket([]byte(bucketServersAPIKeyIndex)).Put([]byte(server.APIKey), []byte(id)); err != nil {
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
		server := new(types.Server)
		if err := bboltUnmarshal(v, server); err != nil {
			return nil
		}
		if subtle.ConstantTimeCompare([]byte(server.APIKey), []byte(apiKey)) != 1 {
			return nil
		}
		found = server
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
			server := new(types.Server)
			if err := bboltUnmarshal(v, server); err != nil {
				return err
			}
			out = append(out, server)
		}
		return nil
	})
	return out, err
}

func findServerByID(id uuid.UUID) (*types.Server, error) {
	idStr := id.String()
	var server *types.Server
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		v := b.Get([]byte(idStr))
		if v == nil {
			return nil
		}
		server = new(types.Server)
		return bboltUnmarshal(v, server)
	})
	return server, err
}

func deleteServerByID(id uuid.UUID) error {
	idStr := id.String()
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		v := b.Get([]byte(idStr))
		if v != nil {
			server := new(types.Server)
			if err := bboltUnmarshal(v, server); err == nil {
				if server.APIKey != "" {
					_ = tx.Bucket([]byte(bucketServersAPIKeyIndex)).Delete([]byte(server.APIKey))
				}
			}
		}

		deleteDevicesTx(tx, func(devID string, D *types.Device) bool {
			return D.ServerID.String() == idStr
		})
		return b.Delete([]byte(idStr))
	})
}
