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
)

type createMeshGroupRequest struct {
	MeshGroup *types.MeshGroup `json:"MeshGroup"`
}

type updateMeshGroupRequest struct {
	MeshGroup *types.MeshGroup `json:"MeshGroup"`
}

type deleteMeshGroupRequest struct {
	MeshGroupID uuid.UUID `json:"MeshGroupID"`
}

type getMeshGroupRequest struct {
	MeshGroupID uuid.UUID `json:"MeshGroupID"`
}

type listMeshGroupsRequest struct {
	Limit  int `json:"Limit"`
	Offset int `json:"Offset"`
}

func handleAdminMeshGroupCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(createMeshGroupRequest)
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

	if err := createMeshGroup(F.MeshGroup); err != nil {
		ERR(err)
		senderr(w, 500, "Unable to create mesh group, please try again later")
		return
	}

	sendObject(w, F.MeshGroup)
}

func handleAdminMeshGroupUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(updateMeshGroupRequest)
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

	if err := updateMeshGroup(F.MeshGroup); err != nil {
		ERR(err)
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminMeshGroupDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(deleteMeshGroupRequest)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if servers, err := findServersByMeshGroup(F.MeshGroupID.String()); err == nil {
		for _, s := range servers {
			s.MeshGroupID = ""
			if _, uerr := updateServer(s); uerr != nil {
				ERR(uerr)
			}
		}
	}

	if err := deleteMeshGroupByID(F.MeshGroupID); err != nil {
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
	mg, err := findMeshGroupByID(gid)
	if err != nil {
		return err
	}
	if mg == nil {
		return errors.New("mesh group not found")
	}

	siblings, err := findServersByMeshGroup(s.MeshGroupID)
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

func handleAdminMeshGroupGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(getMeshGroupRequest)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	mg, err := findMeshGroupByID(F.MeshGroupID)
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

func handleAdminMeshGroupList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(listMeshGroupsRequest)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	limit := F.Limit
	if limit <= 0 {
		limit = 1000
	}

	mgs, err := listMeshGroups(int64(limit), int64(F.Offset))
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
