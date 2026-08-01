package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

type FORM_CREATE_MESHGROUP struct {
	MeshGroup *types.MeshGroup `json:"MeshGroup"`
}

type FORM_UPDATE_MESHGROUP struct {
	MeshGroup *types.MeshGroup `json:"MeshGroup"`
}

type FORM_DELETE_MESHGROUP struct {
	MeshGroupID uuid.UUID `json:"MeshGroupID"`
}

type FORM_GET_MESHGROUP struct {
	MeshGroupID uuid.UUID `json:"MeshGroupID"`
}

type FORM_LIST_MESHGROUP struct {
	Limit  int `json:"Limit"`
	Offset int `json:"Offset"`
}

func API_AdminMeshGroupCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_CREATE_MESHGROUP)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if err := validateMeshGroup(F.MeshGroup); err != nil {
		senderr(w, 400, err.Error())
		return
	}

	F.MeshGroup.ID = uuid.New()
	F.MeshGroup.CreatedAt = time.Now()

	if err := DB_CreateMeshGroup(F.MeshGroup); err != nil {
		ERR(err)
		senderr(w, 500, "Unable to create mesh group, please try again later")
		return
	}

	sendObject(w, F.MeshGroup)
}

func API_AdminMeshGroupUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_MESHGROUP)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if F.MeshGroup == nil || F.MeshGroup.ID == uuid.Nil {
		senderr(w, 400, "MeshGroup id is required")
		return
	}
	if err := validateMeshGroup(F.MeshGroup); err != nil {
		senderr(w, 400, err.Error())
		return
	}

	if err := DB_UpdateMeshGroup(F.MeshGroup); err != nil {
		ERR(err)
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminMeshGroupDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_MESHGROUP)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if servers, err := DB_FindServersByMeshGroup(F.MeshGroupID.String()); err == nil {
		for _, s := range servers {
			s.MeshGroupID = ""
			if _, uerr := DB_UpdateServer(s); uerr != nil {
				ERR(uerr)
			}
		}
	}

	if err := DB_DeleteMeshGroupByID(F.MeshGroupID); err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func validateServerMesh(s *types.Server) error {
	if s.WireGuardMeshPort != 0 && s.WireGuardMeshPort == s.WireGuardPort {
		return errors.New("WireGuardMeshPort must differ from WireGuardPort")
	}
	if s.MeshGroupID == "" {
		return nil
	}
	gid, err := uuid.Parse(s.MeshGroupID)
	if err != nil {
		return errors.New("invalid MeshGroupID")
	}
	mg, err := DB_findMeshGroupByID(gid)
	if err != nil {
		return err
	}
	if mg == nil {
		return errors.New("mesh group not found")
	}

	siblings, err := DB_FindServersByMeshGroup(s.MeshGroupID)
	if err != nil {
		return err
	}
	for _, sib := range siblings {
		if sib.ID == s.ID {
			continue
		}
		if cidrsOverlap(s.WireGuardSubnet, sib.WireGuardSubnet) {
			return fmt.Errorf("WireGuardSubnet %s overlaps mesh sibling %q (%s)", s.WireGuardSubnet, sib.Tag, sib.WireGuardSubnet)
		}
		if cidrsOverlap(s.WireGuardSubnet6, sib.WireGuardSubnet6) {
			return fmt.Errorf("WireGuardSubnet6 %s overlaps mesh sibling %q (%s)", s.WireGuardSubnet6, sib.Tag, sib.WireGuardSubnet6)
		}
	}
	return nil
}

func cidrsOverlap(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	_, na, err1 := net.ParseCIDR(a)
	_, nb, err2 := net.ParseCIDR(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return na.Contains(nb.IP) || nb.Contains(na.IP)
}

func API_AdminMeshGroupGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_MESHGROUP)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	mg, err := DB_findMeshGroupByID(F.MeshGroupID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if mg == nil {
		w.WriteHeader(204)
		return
	}

	sendObject(w, mg)
}

func API_AdminMeshGroupList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_MESHGROUP)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	limit := F.Limit
	if limit <= 0 {
		limit = 1000
	}

	mgs, err := DB_ListMeshGroups(int64(limit), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, mgs)
}

func validateMeshGroup(mg *types.MeshGroup) error {
	if mg == nil {
		return errors.New("MeshGroup is required")
	}
	if mg.Tag == "" {
		return errors.New("MeshGroup tag is required")
	}
	return nil
}

func DB_CreateMeshGroup(mg *types.MeshGroup) error { return BBolt_CreateMeshGroup(mg) }
func DB_UpdateMeshGroup(mg *types.MeshGroup) error { return BBolt_UpdateMeshGroup(mg) }
func DB_DeleteMeshGroupByID(id uuid.UUID) error    { return BBolt_DeleteMeshGroupByID(id.String()) }
func DB_findMeshGroupByID(id uuid.UUID) (*types.MeshGroup, error) {
	return BBolt_findMeshGroupByID(id.String())
}
func DB_ListMeshGroups(limit, offset int64) ([]*types.MeshGroup, error) {
	return BBolt_findMeshGroups(limit, offset)
}

func DB_FindServersByMeshGroup(meshGroupID string) ([]*types.Server, error) {
	all, err := BBolt_FindAllServers(1000000, 0)
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

func BBolt_CreateMeshGroup(mg *types.MeshGroup) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(MESHGROUPS_BUCKET))
		data, err := bboltMarshal(mg)
		if err != nil {
			return err
		}
		return b.Put([]byte(mg.ID.String()), data)
	})
}

func BBolt_UpdateMeshGroup(mg *types.MeshGroup) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(MESHGROUPS_BUCKET))
		id := mg.ID.String()
		v := b.Get([]byte(id))
		if v == nil {
			return errors.New("mesh group not found")
		}
		MG := new(types.MeshGroup)
		if err := bboltUnmarshal(v, MG); err != nil {
			return err
		}
		MG.Tag = mg.Tag
		MG.Description = mg.Description
		data, err := bboltMarshal(MG)
		if err != nil {
			return err
		}
		return b.Put([]byte(id), data)
	})
}

func BBolt_findMeshGroupByID(id string) (*types.MeshGroup, error) {
	var mg *types.MeshGroup
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(MESHGROUPS_BUCKET))
		v := b.Get([]byte(id))
		if v == nil {
			return nil
		}
		mg = new(types.MeshGroup)
		return bboltUnmarshal(v, mg)
	})
	return mg, err
}

func BBolt_DeleteMeshGroupByID(id string) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(MESHGROUPS_BUCKET))
		return b.Delete([]byte(id))
	})
}

func BBolt_findMeshGroups(limit, offset int64) ([]*types.MeshGroup, error) {
	list := make([]*types.MeshGroup, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(MESHGROUPS_BUCKET))
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
