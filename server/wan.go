package main

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	gobolt "go.etcd.io/bbolt"
)

// WAN ("wide area network") is an over-arching network that aggregates one or
// more WireGuard server subnets (e.g. 10.0.0.0/8 covering 10.0.0.0/16 and
// 10.3.0.0/16). It is managed as a first-class entity here and referenced from
// Server.WAN.

// ---------------------------------------------------------------------------
// Forms
// ---------------------------------------------------------------------------

type FORM_CREATE_WAN struct {
	WAN *types.WAN `json:"WAN"`
}

type FORM_UPDATE_WAN struct {
	WAN *types.WAN `json:"WAN"`
}

type FORM_DELETE_WAN struct {
	WANID uuid.UUID `json:"WANID"`
}

type FORM_GET_WAN struct {
	WANID uuid.UUID `json:"WANID"`
}

type FORM_LIST_WAN struct {
	Limit  int `json:"Limit"`
	Offset int `json:"Offset"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func API_AdminWANCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_CREATE_WAN)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if err := validateWAN(F.WAN); err != nil {
		senderr(w, 400, err.Error())
		return
	}

	F.WAN.ID = uuid.New()

	if err := DB_CreateWAN(F.WAN); err != nil {
		ERR(err)
		senderr(w, 500, "Unable to create WAN, please try again later")
		return
	}

	sendObject(w, F.WAN)
}

func API_AdminWANUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_WAN)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if F.WAN == nil || F.WAN.ID == uuid.Nil {
		senderr(w, 400, "WAN id is required")
		return
	}
	if err := validateWAN(F.WAN); err != nil {
		senderr(w, 400, err.Error())
		return
	}

	if err := DB_UpdateWAN(F.WAN); err != nil {
		ERR(err)
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminWANDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_WAN)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if err := DB_DeleteWANByID(F.WANID); err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminWANGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_WAN)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	wan, err := DB_findWANByID(F.WANID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if wan == nil {
		w.WriteHeader(204)
		return
	}

	sendObject(w, wan)
}

func API_AdminWANList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_WAN)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	limit := F.Limit
	if limit <= 0 {
		limit = 1000
	}

	wans, err := DB_ListWANs(int64(limit), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, wans)
}

// validateWAN checks required fields and the CIDR format.
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
	if err := validateCIDR(wan.CIDR); err != nil {
		return errors.New("invalid CIDR: " + err.Error())
	}
	return nil
}

// ---------------------------------------------------------------------------
// DB wrappers
// ---------------------------------------------------------------------------

func DB_CreateWAN(wan *types.WAN) error   { return BBolt_CreateWAN(wan) }
func DB_UpdateWAN(wan *types.WAN) error   { return BBolt_UpdateWAN(wan) }
func DB_DeleteWANByID(id uuid.UUID) error { return BBolt_DeleteWANByID(id.String()) }
func DB_findWANByID(id uuid.UUID) (*types.WAN, error) {
	return BBolt_findWANByID(id.String())
}
func DB_ListWANs(limit, offset int64) ([]*types.WAN, error) {
	return BBolt_findWANs(limit, offset)
}

// ---------------------------------------------------------------------------
// bbolt storage
// ---------------------------------------------------------------------------

func BBolt_CreateWAN(wan *types.WAN) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(WANS_BUCKET))
		data, err := bboltMarshal(wan)
		if err != nil {
			return err
		}
		return b.Put([]byte(wan.ID.String()), data)
	})
}

func BBolt_UpdateWAN(wan *types.WAN) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(WANS_BUCKET))
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

func BBolt_findWANByID(id string) (*types.WAN, error) {
	var wan *types.WAN
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(WANS_BUCKET))
		v := b.Get([]byte(id))
		if v == nil {
			return nil
		}
		wan = new(types.WAN)
		return bboltUnmarshal(v, wan)
	})
	return wan, err
}

func BBolt_DeleteWANByID(id string) error {
	return BBoltDB.Update(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(WANS_BUCKET))
		return b.Delete([]byte(id))
	})
}

func BBolt_findWANs(limit, offset int64) ([]*types.WAN, error) {
	wl := make([]*types.WAN, 0)
	err := BBoltDB.View(func(tx *gobolt.Tx) error {
		b := tx.Bucket([]byte(WANS_BUCKET))
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
