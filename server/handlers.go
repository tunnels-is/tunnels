package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/signal"
	"github.com/tunnels-is/tunnels/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func API_AcceptUserConnections(w http.ResponseWriter, r *http.Request) {

	SCR := new(types.SignedConnectRequest)
	err := decodeBody(r, SCR)
	if err != nil {
		senderr(w, 400, err.Error())
		return
	}

	err = crypt.VerifySignature(SCR.Payload, SCR.Signature, SignKey)
	if err != nil {
		senderr(w, 401, "Invalid signature", slog.Any("err", err))
		return
	}

	CR := new(types.ControllerConnectRequest)
	err = json.Unmarshal(SCR.Payload, CR)
	if err != nil {
		senderr(w, 400, "unable to decode Payload")
		return
	}

	if time.Since(CR.Created).Seconds() > 30 {
		senderr(w, 401, "request not valid")
		return
	}
	if CR.UserID.IsZero() {
		senderr(w, 401, "invalid user identifier")
		return
	}
	totalC, totalUserC := countConnections(CR.UserID.Hex())
	if CR.RequestingPorts {
		if totalC >= slots {
			senderr(w, 400, "server is full")
			return
		}
	}

	Config := Config.Load()
	if totalUserC > Config.UserMaxConnections {
		senderr(w, 400, "user has too many active connections")
		return
	}

	var EH *crypt.SocketWrapper
	EH, err = crypt.NewEncryptionHandler(CR.EncType, CR.CurveType)
	if err != nil {
		ERR("unable to create encryption handler", err)
		return
	}

	EH.SEAL.PublicKey, err = EH.SEAL.NewPublicKeyFromBytes(SCR.UserHandshake)
	if err != nil {
		ERR("Port allocation failed", err)
		return
	}
	err = EH.SEAL.CreateAEAD()
	if err != nil {
		ERR("Port allocation failed", err)
		return
	}

	CRR := types.CreateCRRFromServer(Config)
	index, err := CreateClientCoreMapping(CRR, CR, EH)
	if err != nil {
		ERR("Port allocation failed", err)
		return
	}

	CRR.ServerHandshake = EH.GetPublicKey()
	CRR.ServerHandshakeSignature, err = crypt.SignData(CRR.ServerHandshake, PrivKey)
	if err != nil {
		ERR("Unable to sign server handshake", err)
		return
	}
	CRRB, err := json.Marshal(CRR)
	if err != nil {
		ERR("Unable to marshal CCR", err)
		return
	}

	_, err = w.Write(CRRB)
	if err != nil {
		ERR("Unable to marshal CCR", err)
		return
	}

	clientCoreMappings[index].ToSignal = signal.NewSignal(fmt.Sprintf("TO:%d", index), *CTX.Load(), *Cancel.Load(), time.Second, goroutineLogger, func() {
		toUserChannel(index)
	})

	clientCoreMappings[index].FromSignal = signal.NewSignal(fmt.Sprintf("FROM:%d", index), *CTX.Load(), *Cancel.Load(), time.Second, goroutineLogger, func() {
		fromUserChannel(index)
	})
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

	server, err := DB_FindServerByID(CR.ServerID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if server == nil {
		senderr(w, 204, "Server not found")
		return
	}

	// _, code, err := ValidateSubscription(c, CR)
	// if err != nil {
	// 	return WriteErrorResponse(c, code, err.Error())
	// }

	allowed := false
	if CR.DeviceKey != "" {
		deviceID, err := primitive.ObjectIDFromHex(CR.DeviceKey)
		if err != nil {
			senderr(w, 400, "invalid device key")
			return
		}
		device, err := DB_FindDeviceByID(deviceID)
		if err != nil {
			senderr(w, 500, err.Error())
			return
		}
		if device == nil {
			senderr(w, 401, "Unauthorized")
			return
		}
		for _, g := range server.Groups {
			for _, ug := range device.Groups {
				if g == ug {
					allowed = true
				}
			}
		}
	} else {
		user, err := authenticateUserFromEmailOrIDAndToken("", CR.UserID, CR.DeviceToken)
		if err != nil {
			senderr(w, 401, err.Error())
			return
		}
		for _, g := range server.Groups {
			for _, ug := range user.Groups {
				if g == ug {
					allowed = true
				}
			}
		}
	}

	if len(server.Groups) == 0 {
		allowed = true
	}

	if allowed {
		SCR := new(types.SignedConnectRequest)
		SCR.Payload, err = json.Marshal(CR)
		if err != nil {
			senderr(w, 500, "Unable to decode payload")
			return
		}
		SCR.Signature, err = crypt.SignData(SCR.Payload, PrivKey)
		if err != nil {
			senderr(w, 500, "Unable to sign payload", slog.Any("err", err))
			return
		}

		sendObject(w, SCR)
		return
	}

	senderr(w, 400, "Unauthorized")
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

	user, err := authenticateUserFromEmailOrIDAndToken(UF.Email, primitive.NilObjectID, UF.DeviceToken)
	if err != nil || user == nil {
		senderr(w, 401, err.Error())
		return
	}

	err = DB_toggleUserSubscriptionStatus(UF)
	if err != nil {
		senderr(w, 500, "unexpected error, please try again later")
		return
	}

	w.WriteHeader(200)
}
func API_ActivateLicenseKey(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	AF := new(KEY_ACTIVATE_FORM)
	err := decodeBody(r, AF)
	if err != nil {
		senderr(w, 400, err.Error())
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", AF.UID, AF.DeviceToken)
	if err != nil {
		senderr(w, 401, err.Error())
		return
	}

	INFO(3, "KEY attempt:", AF.Key)

	lemonClient := lc.Load()
	key, resp, err := lemonClient.Licenses.Validate(context.Background(), AF.Key, "")
	if err != nil {
		if resp != nil && resp.Body != nil {
			senderr(w, 500, "unexpected error, please try again")
			return
		}
		senderr(w, 500, "unexpected error, please try again")
		return
	}

	if key.LicenseKey.ActivationUsage > 0 {
		senderr(w, 400, "key is already in use, please contact customer support")
		return
	}

	if strings.Contains(strings.ToLower(key.Meta.ProductName), "anonymous") {
		if user.SubExpiration.IsZero() {
			user.SubExpiration = time.Now()
		}
		if time.Until(user.SubExpiration).Seconds() > 1 {
			user.SubExpiration = time.Now()
		}
		user.SubExpiration = user.SubExpiration.AddDate(0, 1, 0).Add(time.Duration(rand.Intn(60)+60) * time.Minute)
		INFO(3, "KEY +1:", key.LicenseKey.Key, " - check activation in lemon")

		user.Key = &LicenseKey{
			Created: key.LicenseKey.CreatedAt,
			Months:  1,
			Key:     "unknown",
		}
	} else {
		ns := strings.Split(key.Meta.ProductName, " ")
		months, err := strconv.Atoi(ns[0])
		if err != nil {
			ADMIN(3, "unable to parse license key name:", err)
			senderr(w, 500, "Something went wrong, please contact customer support")
		}
		if user.SubExpiration.IsZero() {
			user.SubExpiration = time.Now()
		}
		user.SubExpiration = time.Now().AddDate(0, months, 0).Add(time.Duration(rand.Intn(600)+60) * time.Minute)
		INFO(3, "KEY +", months, ":", key.LicenseKey.Key, " - check activate in lemon")

		user.Key = &LicenseKey{
			Created: key.LicenseKey.CreatedAt,
			Months:  months,
			Key:     key.LicenseKey.Key,
		}
	}

	user.Trial = false
	user.Disabled = false
	err = DB_UserActivateKey(user.SubExpiration, user.Key, user.ID)
	if err != nil {
		senderr(w, 500, "unexpected error, please contact support")
		return
	}

	activeKey, resp, err := lemonClient.Licenses.Activate(context.Background(), AF.Key, "tunnels")
	if err != nil {
		if resp != nil && resp.Body != nil {
			senderr(w, 500, "unexpected error, please try again")
			return
		}
		senderr(w, 500, "unexpected error, please try again")
		return
	}

	if activeKey.Error != "" {
		senderr(w, 400, activeKey.Error)
		return
	}

	if key != nil {
		INFO(3, "KEY: Activated:", key.LicenseKey.Key)
	}

	w.WriteHeader(200)
}
