package main

import (
	"errors"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func findServersByMeshGroup(meshGroupID string) ([]*types.Server, error) {
	all, err := findAllServers(1000000, 0)
	if err != nil {
		return nil, err
	}
	out := make([]*types.Server, 0)
	for _, s := range all {
		if s.MeshGroupID != "" && s.MeshGroupID == meshGroupID {
			out = append(out, s)
		}
	}
	return out, nil
}

func createMeshGroup(mg *types.MeshGroup) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketMeshGroups))
		data, err := bboltMarshal(mg)
		if err != nil {
			return err
		}
		return b.Put([]byte(mg.ID.String()), data)
	})
}

func updateMeshGroup(mg *types.MeshGroup) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketMeshGroups))
		id := mg.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("mesh group not found")
		}
		meshGroup := new(types.MeshGroup)
		if err := bboltUnmarshal(v, meshGroup); err != nil {
			return err
		}
		meshGroup.Tag = mg.Tag
		meshGroup.Description = mg.Description
		data, err := bboltMarshal(meshGroup)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func findMeshGroupByID(id uuid.UUID) (*types.MeshGroup, error) {
	idStr := id.String()
	var mg *types.MeshGroup
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketMeshGroups))
		v := b.Get([]byte(idStr))
		if v == nil {
			return nil
		}
		mg = new(types.MeshGroup)
		return bboltUnmarshal(v, mg)
	})
	return mg, err
}

func deleteMeshGroupByID(id uuid.UUID) error {
	idStr := id.String()
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketMeshGroups))
		return b.Delete([]byte(idStr))
	})
}

func listMeshGroups(limit, offset int64) ([]*types.MeshGroup, error) {
	list := make([]*types.MeshGroup, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketMeshGroups))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if skipped < offset {
				skipped++
				continue
			}
			if int64(len(list)) >= limit {
				break
			}
			mg := new(types.MeshGroup)
			if err := bboltUnmarshal(v, mg); err == nil {
				list = append(list, mg)
			}
		}
		return nil
	})
	return list, err
}
