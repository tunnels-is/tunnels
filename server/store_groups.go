package main

import (
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

func findEntitiesByGroupID(id uuid.UUID, objType string, limit, offset int64) ([]any, error) {
	idStr := id.String()
	items := make([]any, 0)
	bucket := ""
	switch objType {
	case "user":
		bucket = bucketUsers
	case "server":
		bucket = bucketServers
	default:
		return nil, fmt.Errorf("unknown type")
	}
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()
		var skipped int64

	cursorLoop:
		for _, v := c.First(); v != nil; _, v = c.Next() {
			var match bool
			switch objType {
			case "server":
				server := new(types.Server)
				if err := bboltUnmarshal(v, server); err == nil {
					if slices.Contains(uuidSliceToString(server.Groups), idStr) {
						match = true
					}
					if match {
						if skipped < offset {
							skipped++
							continue
						}
						if int64(len(items)) >= limit {
							break cursorLoop
						}
						items = append(items, server)
					}
				}
			case "user":
				user := new(User)
				if err := bboltUnmarshal(v, user); err == nil {
					if slices.Contains(uuidSliceToString(user.Groups), idStr) {
						match = true
					}
					if match {
						if skipped < offset {
							skipped++
							continue
						}
						if int64(len(items)) >= limit {
							break cursorLoop
						}
						items = append(items, user)
					}
				}
			}
		}
		return nil
	})
	return items, err
}

func updateGroup(group *Group) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroups))
		id := group.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("group not found")
		}
		stored := new(Group)
		if err := bboltUnmarshal(v, stored); err != nil {
			return err
		}
		stored.Tag = group.Tag
		stored.Description = group.Description
		data, err := bboltMarshal(stored)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func createGroup(group *Group) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroups))
		id := group.ID.String()
		data, err := bboltMarshal(group)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func findGroupByID(id uuid.UUID) (*Group, error) {
	idStr := id.String()
	var group *Group
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroups))
		v := b.Get([]byte(idStr))
		if v == nil {
			return nil
		}
		group = new(Group)
		return bboltUnmarshal(v, group)
	})
	return group, err
}

func deleteGroupByID(id uuid.UUID) error {
	idStr := id.String()
	return db.Update(func(tx *gobolt.Tx) error {

		scrub := func(bucketName string, unmarshalGroups func([]byte) ([]string, any, error), rebuild func(any, []string) ([]byte, error)) error {
			b := tx.Bucket([]byte(bucketName))
			type upd struct {
				key  []byte
				data []byte
			}
			var updates []upd
			c := b.Cursor()
			for k, v := c.First(); k != nil; k, v = c.Next() {
				groups, obj, err := unmarshalGroups(v)
				if err != nil {
					continue
				}
				newGroups := removeString(groups, idStr)
				if len(newGroups) == len(groups) {
					continue
				}
				data, err := rebuild(obj, newGroups)
				if err != nil {
					return err
				}
				updates = append(updates, upd{append([]byte(nil), k...), data})
			}
			for _, u := range updates {
				if err := b.Put(u.key, u.data); err != nil {
					return err
				}
			}
			return nil
		}

		if err := scrub(bucketUsers,
			func(v []byte) ([]string, any, error) {
				user := new(User)
				if err := bboltUnmarshal(v, user); err != nil {
					return nil, nil, err
				}
				return uuidSliceToString(user.Groups), user, nil
			},
			func(obj any, g []string) ([]byte, error) {
				user := obj.(*User)
				user.Groups = stringSliceToUUID(g)
				return bboltMarshal(user)
			}); err != nil {
			return err
		}

		if err := scrub(bucketServers,
			func(v []byte) ([]string, any, error) {
				server := new(types.Server)
				if err := bboltUnmarshal(v, server); err != nil {
					return nil, nil, err
				}
				return uuidSliceToString(server.Groups), server, nil
			},
			func(obj any, g []string) ([]byte, error) {
				server := obj.(*types.Server)
				server.Groups = stringSliceToUUID(g)
				return bboltMarshal(server)
			}); err != nil {
			return err
		}

		return tx.Bucket([]byte(bucketGroups)).Delete([]byte(idStr))
	})
}

func addToGroup(groupID, typeID uuid.UUID, objType string) error {
	groupIDStr := groupID.String()
	typeIDStr := typeID.String()
	bucket := ""
	switch objType {
	case "user":
		bucket = bucketUsers
	case "server":
		bucket = bucketServers
	default:
		return fmt.Errorf("unknown type")
	}
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(typeIDStr))
		if v == nil {
			return errors.New("object not found")
		}
		var err error
		switch objType {
		case "user":
			user := new(User)
			_ = bboltUnmarshal(v, user)
			groups := uuidSliceToString(user.Groups)
			if !contains(groups, groupIDStr) {
				groups = append(groups, groupIDStr)
				user.Groups = stringSliceToUUID(groups)
			}
			v, err = bboltMarshal(user)
		case "server":
			server := new(types.Server)
			_ = bboltUnmarshal(v, server)
			groups := uuidSliceToString(server.Groups)
			if !contains(groups, groupIDStr) {
				groups = append(groups, groupIDStr)
				server.Groups = stringSliceToUUID(groups)
			}
			v, err = bboltMarshal(server)
		}
		if err != nil {
			return err
		}
		return b.Put([]byte(typeIDStr), v)
	})
}

func removeFromGroup(groupID, typeID uuid.UUID, objType string) error {
	groupIDStr := groupID.String()
	typeIDStr := typeID.String()
	bucket := ""
	switch objType {
	case "user":
		bucket = bucketUsers
	case "server":
		bucket = bucketServers
	default:
		return fmt.Errorf("unknown type")
	}
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		v := b.Get([]byte(typeIDStr))
		if v == nil {
			return errors.New("object not found")
		}
		var err error
		switch objType {
		case "user":
			user := new(User)
			_ = bboltUnmarshal(v, user)
			groups := uuidSliceToString(user.Groups)
			groups = removeString(groups, groupIDStr)
			user.Groups = stringSliceToUUID(groups)
			v, err = bboltMarshal(user)
		case "server":
			server := new(types.Server)
			_ = bboltUnmarshal(v, server)
			groups := uuidSliceToString(server.Groups)
			groups = removeString(groups, groupIDStr)
			server.Groups = stringSliceToUUID(groups)
			v, err = bboltMarshal(server)
		}
		if err != nil {
			return err
		}
		return b.Put([]byte(typeIDStr), v)
	})
}

func listGroups(limit, offset int64) ([]*Group, error) {
	gl := make([]*Group, 0)
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroups))
		c := b.Cursor()
		var skipped int64
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if skipped < offset {
				skipped++
				continue
			}
			if int64(len(gl)) >= limit {
				break
			}
			group := new(Group)
			if err := bboltUnmarshal(v, group); err == nil {
				gl = append(gl, group)
			}
		}
		return nil
	})
	return gl, err
}
