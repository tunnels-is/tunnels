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
	form := new(createGroupRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if form.Group == nil || form.Group.Tag == "" {
		sendError(w, 400, "Invalid group format")
		return
	}

	form.Group.ID = uuid.New()
	form.Group.CreatedAt = time.Now()

	err = createGroup(form.Group)
	if err != nil {
		ERR(err)
		sendError(w, 500, "Unable to create group, please try again later")
		return
	}

	sendObject(w, form.Group)
}

func handleAdminGroupAdd(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(groupAddRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	var u *User
	var s *types.Server

	switch form.Type {
	case "server":
		s, err = findServerByID(form.TypeID)
		if err != nil {
			sendError(w, 400, err.Error())
			return
		}
		if s == nil {
			sendError(w, 404, "server not found")
			return
		}
	case "user":

		if form.TypeID == uuid.Nil && form.TypeTag != "" {
			u, err = findUserByEmail(form.TypeTag)
		} else {
			u, err = findUserByID(form.TypeID)
		}
		if err != nil {
			sendError(w, 400, err.Error())
			return
		}
		if u == nil {
			sendError(w, 204, "user not found")
			return
		}
		form.TypeID = u.ID
	}

	err = addToGroup(form.GroupID, form.TypeID, form.Type)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	switch {
	case u != nil:
		sendObject(w, u.ToMinifiedUser())
	case s != nil:
		sendObject(w, s)
	default:
		sendError(w, 500, "Unknown error, please try again in a moment")
	}
}

func handleAdminGroupRemove(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(groupRemoveRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = removeFromGroup(form.GroupID, form.TypeID, form.Type)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminGroupUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(updateGroupRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = updateGroup(form.Group)
	if err != nil {
		ERR(err)
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminGroupDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(deleteGroupRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = deleteGroupByID(form.GID)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminGroupGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(getGroupRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	group, err := findGroupByID(form.GID)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
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
	form := new(getGroupEntitiesRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	entities, err := findEntitiesByGroupID(form.GID, form.Type, int64(form.Limit), int64(form.Offset))
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	if form.Type == "user" {
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
	form := new(listGroupsRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	limit := form.Limit
	if limit <= 0 {
		limit = 100
	}

	groups, err := listGroups(int64(limit), int64(form.Offset))
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, groups)
}
