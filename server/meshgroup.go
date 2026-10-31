package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
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
	form := new(createMeshGroupRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if err := validateMeshGroup(form.MeshGroup); err != nil {
		sendError(w, 400, err.Error())
		return
	}

	form.MeshGroup.ID = uuid.New()
	form.MeshGroup.CreatedAt = time.Now()

	if err := createMeshGroup(form.MeshGroup); err != nil {
		ERR(err)
		sendError(w, 500, "Unable to create mesh group, please try again later")
		return
	}

	sendObject(w, form.MeshGroup)
}

func handleAdminMeshGroupUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(updateMeshGroupRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if form.MeshGroup == nil || form.MeshGroup.ID == uuid.Nil {
		sendError(w, 400, "MeshGroup id is required")
		return
	}
	if err := validateMeshGroup(form.MeshGroup); err != nil {
		sendError(w, 400, err.Error())
		return
	}

	if err := updateMeshGroup(form.MeshGroup); err != nil {
		ERR(err)
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminMeshGroupDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(deleteMeshGroupRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if servers, err := findServersByMeshGroup(form.MeshGroupID.String()); err == nil {
		for _, s := range servers {
			s.MeshGroupID = ""
			if _, uerr := updateServer(s); uerr != nil {
				ERR(uerr)
			}
		}
	}

	if err := deleteMeshGroupByID(form.MeshGroupID); err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
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

	if s.WireGuardSubnet == "" {
		return errors.New("WireGuardSubnet is required for mesh group members")
	}
	own4, err := types.ValidateMeshSubnet(s.WireGuardSubnet)
	if err != nil {
		return fmt.Errorf("WireGuardSubnet: %w", err)
	}
	if !own4.Addr().Is4() {
		return fmt.Errorf("WireGuardSubnet %s must be an IPv4 prefix", s.WireGuardSubnet)
	}
	var own6 netip.Prefix
	if s.WireGuardSubnet6 != "" {
		own6, err = types.ValidateMeshSubnet(s.WireGuardSubnet6)
		if err != nil {
			return fmt.Errorf("WireGuardSubnet6: %w", err)
		}
		if !own6.Addr().Is6() {
			return fmt.Errorf("WireGuardSubnet6 %s must be an IPv6 prefix", s.WireGuardSubnet6)
		}
	}

	siblings, err := findServersByMeshGroup(s.MeshGroupID)
	if err != nil {
		return err
	}
	for _, sib := range siblings {
		if sib.ID == s.ID {
			continue
		}
		for _, sibSubnet := range []string{sib.WireGuardSubnet, sib.WireGuardSubnet6} {
			sp, ok := types.ParseLocalSubnet(sibSubnet)
			if !ok {
				continue
			}
			if types.MeshSubnetsOverlap(own4, sp) {
				return fmt.Errorf("WireGuardSubnet %s overlaps mesh sibling %q (%s)", s.WireGuardSubnet, sib.Tag, sibSubnet)
			}
			if types.MeshSubnetsOverlap(own6, sp) {
				return fmt.Errorf("WireGuardSubnet6 %s overlaps mesh sibling %q (%s)", s.WireGuardSubnet6, sib.Tag, sibSubnet)
			}
		}
	}
	return nil
}

func handleAdminMeshGroupGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(getMeshGroupRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	mg, err := findMeshGroupByID(form.MeshGroupID)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
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
	form := new(listMeshGroupsRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	limit := form.Limit
	if limit <= 0 {
		limit = 1000
	}

	mgs, err := listMeshGroups(int64(limit), int64(form.Offset))
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
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
