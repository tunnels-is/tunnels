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
			device := new(types.Device)
			if err := bboltUnmarshal(v, device); err == nil {
				uid := device.UserID.String()
				if uid != "00000000-0000-0000-0000-000000000000" {
					_ = tx.Bucket([]byte(bucketDevicesUserIDIndex)).Delete([]byte(uid + "/" + idStr))
				}
				if device.WireGuardKey != "" {
					_ = tx.Bucket([]byte(bucketDevicesWGKeyIndex)).Delete([]byte(device.WireGuardKey))
				}
			}
		}
		return b.Delete([]byte(idStr))
	})
}

func updateDevice(device *types.Device) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		id := device.ID.String()
		devUserIdx := tx.Bucket([]byte(bucketDevicesUserIDIndex))
		wgIdx := tx.Bucket([]byte(bucketDevicesWGKeyIndex))

		var oldWGKey string
		if old := b.Get([]byte(id)); old != nil {
			oldDevice := new(types.Device)
			if err := bboltUnmarshal(old, oldDevice); err == nil {
				if oldDevice.UserID != device.UserID {
					oldUID := oldDevice.UserID.String()
					if oldUID != "00000000-0000-0000-0000-000000000000" {
						_ = devUserIdx.Delete([]byte(oldUID + "/" + id))
					}
				}
				oldWGKey = oldDevice.WireGuardKey
			}
		}

		if device.WireGuardKey != "" && device.WireGuardKey != oldWGKey {
			if existing := wgIdx.Get([]byte(device.WireGuardKey)); existing != nil && string(existing) != id {
				return errors.New("WireGuard key already in use")
			}
		}

		if err := deviceAddressConflicts(tx, device); err != nil {
			return err
		}

		data, err := bboltMarshal(device)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}

		uid := device.UserID.String()
		if uid != "00000000-0000-0000-0000-000000000000" {
			if err := devUserIdx.Put([]byte(uid+"/"+id), nil); err != nil {
				return err
			}
		}

		if oldWGKey != "" && oldWGKey != device.WireGuardKey {
			_ = wgIdx.Delete([]byte(oldWGKey))
		}
		if device.WireGuardKey != "" {
			if err := wgIdx.Put([]byte(device.WireGuardKey), []byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func deviceAddressConflicts(tx *gobolt.Tx, device *types.Device) error {
	if device == nil {
		return nil
	}
	if device.ServerID != uuid.Nil && (device.WireGuardIP != "" || device.WireGuardIPv6 != "") {
		sv := tx.Bucket([]byte(bucketServers)).Get([]byte(device.ServerID.String()))
		if sv != nil {
			s := new(types.Server)
			if err := bboltUnmarshal(sv, s); err == nil {
				if s.WireGuardSubnet != "" || s.WireGuardSubnet6 != "" {
					if err := types.ValidateDeviceWireGuardAddrs(s.WireGuardSubnet, s.WireGuardSubnet6, device.WireGuardIP, device.WireGuardIPv6); err != nil {
						return fmt.Errorf("%w: %s", errDeviceIPReserved, err.Error())
					}
				}
			}
		}
	}
	if device.WireGuardIP == "" && device.WireGuardIPv6 == "" {
		return nil
	}
	id := device.ID.String()
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
		if other.ServerID != device.ServerID {
			continue
		}
		if device.WireGuardIP != "" && other.WireGuardIP == device.WireGuardIP {
			return errDeviceIPInUse
		}
		if device.WireGuardIPv6 != "" && other.WireGuardIPv6 == device.WireGuardIPv6 {
			return errDeviceIPv6InUse
		}
	}
	return nil
}

func getDevices(limit, offset int64) ([]*types.Device, error) {
	devices := make([]*types.Device, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if skipped < offset {
				skipped++
				continue
			}
			if int64(len(devices)) >= limit {
				break
			}
			device := new(types.Device)
			if err := bboltUnmarshal(v, device); err == nil {
				devices = append(devices, device)
			}
		}
		return nil
	})
	return devices, err
}

func getAllDevices() ([]*types.Device, error) {
	devices := make([]*types.Device, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			device := new(types.Device)
			if err := bboltUnmarshal(v, device); err == nil {
				devices = append(devices, device)
			}
		}
		return nil
	})
	return devices, err
}

func getDevicesByUserID(userID uuid.UUID) ([]*types.Device, error) {
	devices := make([]*types.Device, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		idx := tx.Bucket([]byte(bucketDevicesUserIDIndex))
		prefix := []byte(userID.String() + "/")
		c := idx.Cursor()
		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			devID := k[len(prefix):]
			v := b.Get(devID)
			if v == nil {
				continue
			}
			device := new(types.Device)
			if err := bboltUnmarshal(v, device); err == nil {
				devices = append(devices, device)
			}
		}
		return nil
	})
	return devices, err
}

func createDevice(device *types.Device) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketDevices))
		id := device.ID.String()

		if device.WireGuardKey != "" {
			wgIdx := tx.Bucket([]byte(bucketDevicesWGKeyIndex))
			if existing := wgIdx.Get([]byte(device.WireGuardKey)); existing != nil && string(existing) != id {
				return errors.New("WireGuard key already in use")
			}
		}

		if err := deviceAddressConflicts(tx, device); err != nil {
			return err
		}

		data, err := bboltMarshal(device)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(id), data); err != nil {
			return err
		}
		uid := device.UserID.String()
		if uid != "00000000-0000-0000-0000-000000000000" {
			if err := tx.Bucket([]byte(bucketDevicesUserIDIndex)).Put([]byte(uid+"/"+id), nil); err != nil {
				return err
			}
		}
		if device.WireGuardKey != "" {
			if err := tx.Bucket([]byte(bucketDevicesWGKeyIndex)).Put([]byte(device.WireGuardKey), []byte(id)); err != nil {
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
		device := new(types.Device)
		if err := bboltUnmarshal(v, device); err != nil {
			continue
		}
		devID := string(k)
		if !pred(devID, device) {
			continue
		}
		matches = append(matches, match{devID: devID, wgKey: device.WireGuardKey, userID: device.UserID.String()})
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
