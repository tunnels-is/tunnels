package main

import (
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
		rotated := apiKeyRotated(oldAPIKey, server.APIKey)
		newAPIKey := oldAPIKey
		if server.APIKey == "" {
			newAPIKey = ""
		} else if rotated {
			newAPIKey = storeSecret(server.APIKey)
		}
		existing.Tag = server.Tag
		existing.InfraTag = server.InfraTag
		existing.Country = server.Country
		existing.IP = server.IP
		existing.Port = server.Port
		existing.APIKey = newAPIKey
		existing.WireGuardPort = server.WireGuardPort
		if rotated {
			existing.WireGuardPubKey = ""
			existing.DiscoKey = ""
		}
		existing.WireGuardIface = server.WireGuardIface
		existing.WireGuardSubnet = server.WireGuardSubnet
		existing.WireGuardSubnet6 = server.WireGuardSubnet6
		existing.InternetIface = server.InternetIface
		existing.InsecureSkipVerify = server.InsecureSkipVerify
		existing.EnableFirewall = true
		existing.WANID = server.WANID
		existing.MeshGroupID = server.MeshGroupID
		existing.WireGuardMeshPort = server.WireGuardMeshPort

		if rotated && newAPIKey != "" && apiKeyIndexTaken(apikeyIdx, server.APIKey, id) {
			return errors.New("APIKey already in use")
		}

		data, err := bboltMarshal(existing)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if oldAPIKey != "" && oldAPIKey != newAPIKey {
			_ = apikeyIdx.Delete([]byte(oldAPIKey))
		}
		if newAPIKey != "" {
			if err := apikeyIdx.Put([]byte(newAPIKey), []byte(id)); err != nil {
				return err
			}
		}
		result = existing
		return nil
	})
	return result, err
}

func setServerDiscoKey(id uuid.UUID, discoKey string) error {
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
		existing.DiscoKey = discoKey
		data, err := bboltMarshal(existing)
		if err != nil {
			return err
		}
		return b.Put(key, data)
	})
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

func apiKeyIndexTaken(idx *gobolt.Bucket, rawOrStored, exceptID string) bool {
	if idx == nil || rawOrStored == "" {
		return false
	}
	keys := []string{storeSecret(rawOrStored)}
	if !storedSecretIsHash(rawOrStored) {
		keys = append(keys, rawOrStored)
	}
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if existing := idx.Get([]byte(key)); existing != nil && string(existing) != exceptID {
			return true
		}
	}
	return false
}

func createServer(server *types.Server) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))

		server.WAN = nil
		server.EnableFirewall = true
		stored := *server
		stored.APIKey = storeSecret(server.APIKey)
		id := server.ID.String()
		if server.APIKey != "" {
			apikeyIdx := tx.Bucket([]byte(bucketServersAPIKeyIndex))
			if apiKeyIndexTaken(apikeyIdx, server.APIKey, id) {
				return errors.New("APIKey already in use")
			}
		}
		data, err := bboltMarshal(&stored)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		if stored.APIKey != "" {
			if err := tx.Bucket([]byte(bucketServersAPIKeyIndex)).Put([]byte(stored.APIKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func findServerByAPIKey(apiKey string) (*types.Server, error) {
	if apiKey == "" {
		return nil, nil
	}
	found, legacy, err := lookupServerByAPIKey(apiKey)
	if err != nil || found == nil || !legacy {
		return found, err
	}
	if err := upgradeServerAPIKey(found.ID, apiKey); err != nil {
		return nil, err
	}
	found.APIKey = hashSecret(apiKey)
	return found, nil
}

func lookupServerByAPIKey(apiKey string) (*types.Server, bool, error) {
	var found *types.Server
	err := db.View(func(tx *gobolt.Tx) error {
		idx := tx.Bucket([]byte(bucketServersAPIKeyIndex))
		load := func(id []byte) (*types.Server, bool) {
			if id == nil {
				return nil, false
			}
			v := tx.Bucket([]byte(bucketServers)).Get(id)
			if v == nil {
				return nil, false
			}
			server := new(types.Server)
			if err := bboltUnmarshal(v, server); err != nil {
				return nil, false
			}
			if !secretMatches(server.APIKey, apiKey) {
				return nil, false
			}
			return server, true
		}
		if server, ok := load(idx.Get([]byte(hashSecret(apiKey)))); ok {
			found = server
			return nil
		}
		if server, ok := load(idx.Get([]byte(apiKey))); ok {
			found = server
			return nil
		}
		return nil
	})
	if found == nil {
		return nil, false, err
	}
	return found, !storedSecretIsHash(found.APIKey), err
}

func upgradeServerAPIKey(id uuid.UUID, presented string) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketServers))
		key := []byte(id.String())
		v := b.Get(key)
		if v == nil {
			return nil
		}
		existing := new(types.Server)
		if err := bboltUnmarshal(v, existing); err != nil {
			return err
		}
		if storedSecretIsHash(existing.APIKey) || !secretMatches(existing.APIKey, presented) {
			return nil
		}
		old := existing.APIKey
		existing.APIKey = hashSecret(presented)
		data, err := bboltMarshal(existing)
		if err != nil {
			return err
		}
		if err := b.Put(key, data); err != nil {
			return err
		}
		idx := tx.Bucket([]byte(bucketServersAPIKeyIndex))
		_ = idx.Delete([]byte(old))
		return idx.Put([]byte(existing.APIKey), key)
	})
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
