package main

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func deleteDeviceByID(id uuid.UUID) error {
	idStr := id.String()
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		v := b.Get([]byte(idStr))
		if v != nil {
			D := new(types.Device)
			if err := bboltUnmarshal(v, D); err == nil {
				uid := D.UserID.String()
				if uid != "00000000-0000-0000-0000-000000000000" {
					_ = tx.Bucket([]byte(bucketDevicesUserIDIndex)).Delete([]byte(uid + "/" + idStr))
				}
				if D.WireGuardKey != "" {
					_ = tx.Bucket([]byte(bucketDevicesWGKeyIndex)).Delete([]byte(D.WireGuardKey))
				}
			}
		}
		return b.Delete([]byte(idStr))
	})
}

func updateDevice(D *types.Device) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		id := D.ID.String()
		devUserIdx := tx.Bucket([]byte(bucketDevicesUserIDIndex))
		wgIdx := tx.Bucket([]byte(bucketDevicesWGKeyIndex))

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

		if err := deviceAddressConflicts(tx, D); err != nil {
			return err
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

func deviceAddressConflicts(tx *gobolt.Tx, D *types.Device) error {
	if D == nil {
		return nil
	}
	if D.ServerID != uuid.Nil && (D.WireGuardIP != "" || D.WireGuardIPv6 != "") {
		sv := tx.Bucket([]byte(bucketServers)).Get([]byte(D.ServerID.String()))
		if sv != nil {
			s := new(types.Server)
			if err := bboltUnmarshal(sv, s); err == nil {
				if s.WireGuardSubnet != "" || s.WireGuardSubnet6 != "" {
					if err := types.ValidateDeviceWireGuardAddrs(s.WireGuardSubnet, s.WireGuardSubnet6, D.WireGuardIP, D.WireGuardIPv6); err != nil {
						return fmt.Errorf("%w: %s", errDeviceIPReserved, err.Error())
					}
				}
			}
		}
	}
	if D.WireGuardIP == "" && D.WireGuardIPv6 == "" {
		return nil
	}
	id := D.ID.String()
	b := tx.Bucket([]byte(bucketDevices))
	c := b.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		if string(k) == id {
			continue
		}
		other := new(types.Device)
		if err := bboltUnmarshal(v, other); err != nil {
			continue
		}
		if other.ServerID != D.ServerID {
			continue
		}
		if D.WireGuardIP != "" && other.WireGuardIP == D.WireGuardIP {
			return errDeviceIPInUse
		}
		if D.WireGuardIPv6 != "" && other.WireGuardIPv6 == D.WireGuardIPv6 {
			return errDeviceIPv6InUse
		}
	}
	return nil
}

func getDevices(limit, offset int64) ([]*types.Device, error) {
	DL := make([]*types.Device, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
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

func getAllDevices() ([]*types.Device, error) {
	DL := make([]*types.Device, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			D := new(types.Device)
			if err := bboltUnmarshal(v, D); err == nil {
				DL = append(DL, D)
			}
		}
		return nil
	})
	return DL, err
}

func getDevicesByUserID(userID uuid.UUID) ([]*types.Device, error) {
	DL := make([]*types.Device, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		devices := tx.Bucket([]byte(bucketDevices))
		idx := tx.Bucket([]byte(bucketDevicesUserIDIndex))
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

func createDevice(D *types.Device) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		id := D.ID.String()

		if D.WireGuardKey != "" {
			wgIdx := tx.Bucket([]byte(bucketDevicesWGKeyIndex))
			if existing := wgIdx.Get([]byte(D.WireGuardKey)); existing != nil && string(existing) != id {
				return errors.New("WireGuard key already in use")
			}
		}

		if err := deviceAddressConflicts(tx, D); err != nil {
			return err
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
			if err := tx.Bucket([]byte(bucketDevicesUserIDIndex)).Put([]byte(uid+"/"+id), nil); err != nil {
				return err
			}
		}
		if D.WireGuardKey != "" {
			if err := tx.Bucket([]byte(bucketDevicesWGKeyIndex)).Put([]byte(D.WireGuardKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func findDeviceByID(id uuid.UUID) (*types.Device, error) {
	idStr := id.String()
	var dev *types.Device
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		v := b.Get([]byte(idStr))
		if v == nil {
			return nil
		}
		dev = new(types.Device)
		return bboltUnmarshal(v, dev)
	})
	return dev, err
}

func findDeviceByWGKey(wgKey string) (*types.Device, error) {
	var dev *types.Device
	err := db.View(func(tx *gobolt.Tx) error {
		devID := tx.Bucket([]byte(bucketDevicesWGKeyIndex)).Get([]byte(wgKey))
		if devID == nil {
			return nil
		}
		v := tx.Bucket([]byte(bucketDevices)).Get(devID)
		if v == nil {
			return nil
		}
		dev = new(types.Device)
		return bboltUnmarshal(v, dev)
	})
	return dev, err
}

func deleteDevicesTx(tx *gobolt.Tx, pred func(devID string, d *types.Device) bool) {
	devB := tx.Bucket([]byte(bucketDevices))
	uidIdx := tx.Bucket([]byte(bucketDevicesUserIDIndex))
	wgIdx := tx.Bucket([]byte(bucketDevicesWGKeyIndex))

	type match struct {
		devID  string
		wgKey  string
		userID string
	}
	var matches []match
	c := devB.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		D := new(types.Device)
		if err := bboltUnmarshal(v, D); err != nil {
			continue
		}
		devID := string(k)
		if !pred(devID, D) {
			continue
		}
		matches = append(matches, match{devID: devID, wgKey: D.WireGuardKey, userID: D.UserID.String()})
	}
	for _, m := range matches {
		if m.wgKey != "" {
			_ = wgIdx.Delete([]byte(m.wgKey))
		}
		if m.userID != "00000000-0000-0000-0000-000000000000" {
			_ = uidIdx.Delete([]byte(m.userID + "/" + m.devID))
		}
		_ = devB.Delete([]byte(m.devID))
	}
}
