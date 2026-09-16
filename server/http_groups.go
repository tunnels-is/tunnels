package main

import (
	"log/slog"
	"net/http"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func handleAdminGroupCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(createGroupRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Group == nil || F.Group.Tag == "" {
		senderr(w, 400, "Invalid group format")
		return
	}

	F.Group.ID = uuid.New()
	F.Group.CreatedAt = time.Now()

	err = createGroup(F.Group)
	if err != nil {
		ERR(err)
		senderr(w, 500, "Unable to create group, please try again later")
		return
	}

	sendObject(w, F.Group)
}

func handleAdminGroupAdd(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(groupAddRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	var u *User
	var s *types.Server

	switch F.Type {
	case "server":
		s, err = findServerByID(F.TypeID)
		if err != nil {
			senderr(w, 400, err.Error())
			return
		}
		if s == nil {
			senderr(w, 404, "server not found")
			return
		}
	case "user":

		if F.TypeID == uuid.Nil && F.TypeTag != "" {
			u, err = findUserByEmail(F.TypeTag)
		} else {
			u, err = findUserByID(F.TypeID)
		}
		if err != nil {
			senderr(w, 400, err.Error())
			return
		}
		if u == nil {
			senderr(w, 204, "user not found")
			return
		}
		F.TypeID = u.ID
	}

	err = addToGroup(F.GroupID, F.TypeID, F.Type)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	switch {
	case u != nil:
		sendObject(w, u.ToMinifiedUser())
	case s != nil:
		sendObject(w, s)
	default:
		senderr(w, 500, "Unknown error, please try again in a moment")
	}
}

func handleAdminGroupRemove(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(groupRemoveRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = removeFromGroup(F.GroupID, F.TypeID, F.Type)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminGroupUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(updateGroupRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = updateGroup(F.Group)
	if err != nil {
		ERR(err)
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminGroupDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(deleteGroupRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = deleteGroupByID(F.GID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminGroupGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(getGroupRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	group, err := findGroupByID(F.GID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	if group == nil {
		w.WriteHeader(204)
		return
	}

	sendObject(w, group)
}

func handleAdminGroupGetEntities(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(getGroupEntitiesRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	entities, err := findEntitiesByGroupID(F.GID, F.Type, int64(F.Limit), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	if F.Type == "user" {
		ul := make([]MinifiedUser, 0)
		for _, v := range entities {
			us, ok := v.(*User)
			if !ok {
				ADMIN("unable to transform user:", reflect.TypeOf(v))
			}
			ul = append(ul, us.ToMinifiedUser())
		}
		sendObject(w, ul)
		return
	}

	sendObject(w, entities)
}

func handleAdminGroupList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(listGroupsRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	limit := F.Limit
	if limit <= 0 {
		limit = 100
	}

	groups, err := listGroups(int64(limit), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, groups)
}
