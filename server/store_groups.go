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
	IL := make([]any, 0)
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
				E := new(types.Server)
				if err := bboltUnmarshal(v, E); err == nil {
					if slices.Contains(uuidSliceToString(E.Groups), idStr) {
						match = true
					}
					if match {
						if skipped < offset {
							skipped++
							continue
						}
						if int64(len(IL)) >= limit {
							break cursorLoop
						}
						IL = append(IL, E)
					}
				}
			case "user":
				E := new(User)
				if err := bboltUnmarshal(v, E); err == nil {
					if slices.Contains(uuidSliceToString(E.Groups), idStr) {
						match = true
					}
					if match {
						if skipped < offset {
							skipped++
							continue
						}
						if int64(len(IL)) >= limit {
							break cursorLoop
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

func updateGroup(G *Group) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroups))
		id := G.ID.String()
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

func createGroup(G *Group) error {
	return db.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroups))
		id := G.ID.String()
		data, err := bboltMarshal(G)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func findGroupByID(id uuid.UUID) (*Group, error) {
	idStr := id.String()
	var G *Group
	err := db.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(bucketGroups))
		v := b.Get([]byte(idStr))
		if v == nil {
			return nil
		}
		G = new(Group)
		return bboltUnmarshal(v, G)
	})
	return G, err
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
				U := new(User)
				if err := bboltUnmarshal(v, U); err != nil {
					return nil, nil, err
				}
				return uuidSliceToString(U.Groups), U, nil
			},
			func(obj any, g []string) ([]byte, error) {
				U := obj.(*User)
				U.Groups = stringSliceToUUID(g)
				return bboltMarshal(U)
			}); err != nil {
			return err
		}

		if err := scrub(bucketServers,
			func(v []byte) ([]string, any, error) {
				S := new(types.Server)
				if err := bboltUnmarshal(v, S); err != nil {
					return nil, nil, err
				}
				return uuidSliceToString(S.Groups), S, nil
			},
			func(obj any, g []string) ([]byte, error) {
				S := obj.(*types.Server)
				S.Groups = stringSliceToUUID(g)
				return bboltMarshal(S)
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
			U := new(User)
			_ = bboltUnmarshal(v, U)
			groups := uuidSliceToString(U.Groups)
			if !contains(groups, groupIDStr) {
				groups = append(groups, groupIDStr)
				U.Groups = stringSliceToUUID(groups)
			}
			v, err = bboltMarshal(U)
		case "server":
			S := new(types.Server)
			_ = bboltUnmarshal(v, S)
			groups := uuidSliceToString(S.Groups)
			if !contains(groups, groupIDStr) {
				groups = append(groups, groupIDStr)
				S.Groups = stringSliceToUUID(groups)
			}
			v, err = bboltMarshal(S)
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
			U := new(User)
			_ = bboltUnmarshal(v, U)
			groups := uuidSliceToString(U.Groups)
			groups = removeString(groups, groupIDStr)
			U.Groups = stringSliceToUUID(groups)
			v, err = bboltMarshal(U)
		case "server":
			S := new(types.Server)
			_ = bboltUnmarshal(v, S)
			groups := uuidSliceToString(S.Groups)
			groups = removeString(groups, groupIDStr)
			S.Groups = stringSliceToUUID(groups)
			v, err = bboltMarshal(S)
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
			D := new(Group)
			if err := bboltUnmarshal(v, D); err == nil {
				gl = append(gl, D)
			}
		}
		return nil
	})
	return gl, err
}
