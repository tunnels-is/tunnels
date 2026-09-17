package main

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

type createWANRequest struct {
	WAN *types.WAN `json:"WAN"`
}

type updateWANRequest struct {
	WAN *types.WAN `json:"WAN"`
}

type deleteWANRequest struct {
	WANID uuid.UUID `json:"WANID"`
}

type getWANRequest struct {
	WANID uuid.UUID `json:"WANID"`
}

type listWANsRequest struct {
	Limit  int `json:"Limit"`
	Offset int `json:"Offset"`
}

func handleAdminWANCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(createWANRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if err := validateWAN(form.WAN); err != nil {
		sendError(w, 400, err.Error())
		return
	}

	form.WAN.ID = uuid.New()

	if err := createWAN(form.WAN); err != nil {
		ERR(err)
		sendError(w, 500, "Unable to create WAN, please try again later")
		return
	}

	sendObject(w, form.WAN)
}

func handleAdminWANUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(updateWANRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if form.WAN == nil || form.WAN.ID == uuid.Nil {
		sendError(w, 400, "WAN id is required")
		return
	}
	if err := validateWAN(form.WAN); err != nil {
		sendError(w, 400, err.Error())
		return
	}

	if err := updateWAN(form.WAN); err != nil {
		ERR(err)
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminWANDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(deleteWANRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if err := deleteWANByID(form.WANID); err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminWANGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(getWANRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	wan, err := findWANByID(form.WANID)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if wan == nil {
		w.WriteHeader(204)
		return
	}

	sendObject(w, wan)
}

func handleAdminWANList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(listWANsRequest)
	if err := decodeBody(r, form); err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	limit := form.Limit
	if limit <= 0 {
		limit = 1000
	}

	wans, err := listWANs(int64(limit), int64(form.Offset))
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, wans)
}

func attachWANs(servers ...*types.Server) {
	hasRef := false
	for _, s := range servers {
		if s != nil && s.WANID != "" {
			hasRef = true
			break
		}
	}
	if !hasRef {
		return
	}

	wans, err := listWANs(1000000, 0)
	if err != nil {
		ERR(err)
		return
	}
	byID := make(map[string]*types.WAN, len(wans))
	for _, wan := range wans {
		byID[wan.ID.String()] = wan
	}

	for _, s := range servers {
		if s != nil && s.WANID != "" {
			s.WAN = byID[s.WANID]
		}
	}
}

func wanCIDRForServer(s *types.Server) string {
	if s == nil || s.WANID == "" {
		return ""
	}
	id, err := uuid.Parse(s.WANID)
	if err != nil {
		return ""
	}
	wan, err := findWANByID(id)
	if err != nil || wan == nil {
		return ""
	}
	return wan.CIDR
}

func validateWAN(wan *types.WAN) error {
	if wan == nil {
		return errors.New("WAN is required")
	}
	if wan.Tag == "" {
		return errors.New("WAN tag is required")
	}
	if wan.CIDR == "" {
		return errors.New("WAN CIDR is required")
	}
	if err := types.ValidateWANCIDR(wan.CIDR); err != nil {
		return err
	}
	return nil
}
