package main

import (
	"errors"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func createWAN(wan *types.WAN) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketWANs))
		data, err := bboltMarshal(wan)
		if err != nil {
			return err
		}
		return b.Put([]byte(wan.ID.String()), data)
	})
}

func updateWAN(wan *types.WAN) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketWANs))
		id := wan.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("WAN not found")
		}
		WW := new(types.WAN)
		if err := bboltUnmarshal(v, WW); err != nil {
			return err
		}
		WW.Tag = wan.Tag
		WW.CIDR = wan.CIDR
		WW.Description = wan.Description
		data, err := bboltMarshal(WW)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func findWANByID(id uuid.UUID) (*types.WAN, error) {
	idStr := id.String()
	var wan *types.WAN
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketWANs))
		v := b.Get([]byte(idStr))
		if v == nil {
			return nil
		}
		wan = new(types.WAN)
		return bboltUnmarshal(v, wan)
	})
	return wan, err
}

func deleteWANByID(id uuid.UUID) error {
	idStr := id.String()
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketWANs))
		return b.Delete([]byte(idStr))
	})
}

func listWANs(limit, offset int64) ([]*types.WAN, error) {
	wl := make([]*types.WAN, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketWANs))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if skipped < offset {
				skipped++
				continue
			}
			if int64(len(wl)) >= limit {
				break
			}
			wan := new(types.WAN)
			if err := bboltUnmarshal(v, wan); err == nil {
				wl = append(wl, wan)
			}
		}
		return nil
	})
	return wl, err
}
