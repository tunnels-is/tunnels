package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/tunnels-is/tunnels/types"
)

func API_AcceptUserConnections(w http.ResponseWriter, r *http.Request) {
	SCR := new(types.SignedConnectRequest)
	err := decodeBody(r, SCR)
	if err != nil {
		senderr(w, 400, err.Error())
		return
	}

	errData, okData := APIv2_AcceptUserConnections(SCR)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}

	CRRB, err := json.Marshal(okData)
	if err != nil {
		ERR("Unable to marshal CCR", err)
		return
	}

	_, err = w.Write(CRRB)
	if err != nil {
		ERR("Unable to write response", err)
		return
	}
}

func API_UserCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	RF := new(REGISTER_FORM)
	err := decodeBody(r, RF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_UserCreate(RF)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_UserUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	UF := new(USER_UPDATE_FORM)
	err := decodeBody(r, UF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_UserUpdate(UF)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_UserLogin(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	LF := new(LOGIN_FORM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_UserLogin(LF)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
	return
}

func API_UserLogout(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	LF := new(LOGOUT_FORM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_UserLogout(LF)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_UserTwoFactorConfirm(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	LF := new(TWO_FACTOR_FORM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_UserTwoFactorConfirm(LF)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_UserList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_USERS)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_UserList(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_DeviceUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_DeviceUpdate(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_DeviceDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_DeviceDelete(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_DeviceList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	hasAPIKey := HTTP_validateKey(r)
	errData, okData := APIv2_DeviceList(F, hasAPIKey)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_DeviceCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	hasAPIKey := HTTP_validateKey(r)

	F := new(FORM_CREATE_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_DeviceCreate(F, hasAPIKey)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_GroupCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_CREATE_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_GroupCreate(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_GroupAdd(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GROUP_ADD)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_GroupAdd(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_GroupRemove(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GROUP_REMOVE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_GroupRemove(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}
func API_GroupUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_GroupUpdate(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_GroupDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_GroupDelete(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_DeviceGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(types.FORM_GET_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_DeviceGet(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_GroupGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_GroupGet(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_GroupGetEntities(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_GROUP_ENTITIES)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_GroupGetEntities(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_GroupList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_GroupList(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_ServersForUser(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_SERVERS)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_ServersForUser(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_ServerUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	F := new(FORM_UPDATE_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_ServerUpdate(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_ServerCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_CREATE_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_ServerCreate(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_ServerGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(types.FORM_GET_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_ServerGet(F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_SessionCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	CR := new(types.ControllerConnectRequest)
	err := decodeBody(r, CR)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_SessionCreate(CR)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_UserRequestPasswordCode(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	RF := new(PASSWORD_RESET_FORM)
	err := decodeBody(r, RF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_UserRequestPasswordCode(RF)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_UserResetPassword(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	RF := new(PASSWORD_RESET_FORM)
	err := decodeBody(r, RF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_UserResetPassword(RF)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_UserToggleSubStatus(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	UF := new(USER_UPDATE_SUB_FORM)
	err := decodeBody(r, UF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_UserToggleSubStatus(UF)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}
func API_ActivateLicenseKey(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	AF := new(KEY_ACTIVATE_FORM)
	err := decodeBody(r, AF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_ActivateLicenseKey(AF)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_Firewall(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	fr := new(types.FirewallRequest)
	err := decodeBody(r, fr)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	errData, okData := APIv2_Firewall(fr)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}

func API_ListDevices(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	hasAPIKey := HTTP_validateKey(r)

	F := new(FORM_LIST_DEVICE)
	if !hasAPIKey {
		err := decodeBody(r, F)
		if err != nil {
			senderr(w, 400, "Invalid request body", slog.Any("error", err))
			return
		}
	}

	errData, okData := APIv2_ListDevices(hasAPIKey, F)
	if errData != nil {
		sendHTTPErrorResponse(w, errData)
		return
	}
	sendHTTPOKResponse(w, 200, okData)
}
