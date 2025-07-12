package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/signal"
	"github.com/tunnels-is/tunnels/types"
	"github.com/xlzd/gotp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

func makeErr(code int, msg string, slogArgs ...any) *ErrorResponse {
	logger.Error(msg, slogArgs...)
	return &ErrorResponse{Code: code, Error: msg}
}

func sendHTTPErrorResponse(w http.ResponseWriter, errResp *ErrorResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(errResp.Code)
	err := json.NewEncoder(w).Encode(errResp)
	if err != nil {
		logger.Error("unable to write JSON errResponse:", slog.Any("err", err))
	}
}

func sendHTTPOKResponse(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Error("unable to write JSON okResponse:", slog.Any("err", err))
	}
}

func cleanData(obj any) (data any) {
	u, ok := obj.(*User)
	if ok {
		u.RemoveSensitiveInformation()
		return u
	}
	return
}

func APIv2_UserLogin(LF *LOGIN_FORM) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := DB_findUserByEmail(LF.Email)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}
	if user == nil {
		return makeErr(401, "Invalid login credentials"), nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(LF.Password))
	if err != nil {
		return makeErr(401, "Invalid login credentials"), nil
	}

	err = validateUserTwoFactor(user, LF)
	if err != nil {
		return makeErr(401, err.Error()), nil
	}

	userLoginUpdate := handleUserDeviceToken(user, LF)
	err = DB_updateUserDeviceTokens(userLoginUpdate)
	if err != nil {
		return makeErr(500, "Database error, please try again in a moment"), nil
	}

	return nil, cleanData(user)
}

func APIv2_UserCreate(RF *REGISTER_FORM) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	if RF.Password == "" {
		return makeErr(400, "Missing Password"), nil
	}

	if len(RF.Password) > 200 {
		return makeErr(400, "Password is too long, maximum 255 characters"), nil
	}

	if len(RF.Password) < 10 {
		return makeErr(400, "Password is too short, minimum 10 characters"), nil
	}

	if len(RF.Email) > 320 {
		return makeErr(400, "Email/Username is too long, maximum 320 characters"), nil
	}

	newUser, err := DB_findUserByEmail(RF.Email)
	if newUser != nil {
		return makeErr(400, "User already registered"), nil
	}
	if err != nil {
		return makeErr(500, "Unexpected error, please try again in a moment"), nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(RF.Password), 13)
	if err != nil {
		return makeErr(500, "Unable to generate a secure password, please contact customer support"), nil
	}

	newUser = new(User)
	newUser.Password = string(hash)
	newUser.ID = primitive.NewObjectID()
	newUser.AdditionalInformation = RF.AdditionalInformation
	newUser.Email = RF.Email
	newUser.Updated = time.Now()
	newUser.Trial = true
	newUser.SubExpiration = time.Now().AddDate(0, 0, 1)
	newUser.Groups = make([]primitive.ObjectID, 0)
	newUser.Tokens = make([]*DeviceToken, 0)

	T := new(DeviceToken)
	T.N = "registration"
	T.DT = uuid.NewString()
	T.Created = time.Now()

	newUser.DeviceToken = T
	newUser.Tokens = append(newUser.Tokens, T)

	err = DB_CreateUser(newUser)
	if err != nil {
		return makeErr(500, "Unexpected error, please try again in a moment"), nil
	}

	return nil, cleanData(newUser)
}

func APIv2_UserUpdate(UF *USER_UPDATE_FORM) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	_, err := authenticateUserFromEmailOrIDAndToken("", UF.UID, UF.DeviceToken)
	if err != nil {
		return makeErr(401, err.Error()), nil
	}

	err = DB_updateUser(UF)
	if err != nil {
		return makeErr(500, "Unable to update users, please try again in a moment"), nil
	}

	return nil, map[string]string{"status": "user updated"}
}

func APIv2_UserLogout(LF *LOGOUT_FORM) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", LF.UID, LF.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}
	if user == nil {
		return makeErr(204, "User not found"), nil
	}

	if LF.All {
		user.Tokens = make([]*DeviceToken, 0)
	} else {
		user.Tokens = slices.DeleteFunc(user.Tokens, func(dt *DeviceToken) bool {
			return dt.DT == LF.LogoutToken
		})
	}

	userTokenUpdate := new(UPDATE_USER_TOKENS)
	userTokenUpdate.ID = user.ID
	userTokenUpdate.Tokens = user.Tokens

	err = DB_updateUserDeviceTokens(userTokenUpdate)
	if err != nil {
		return makeErr(500, "Database error, please try again in a moment"), nil
	}

	return nil, map[string]string{"status": "logged out"}
}

func APIv2_UserRequestPasswordCode(PRF *PASSWORD_RESET_FORM) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := DB_findUserByEmail(PRF.Email)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}
	if user == nil {
		return makeErr(401, "Invalid session token, please log in again"), nil
	}

	if !user.LastResetRequest.IsZero() && time.Since(user.LastResetRequest).Seconds() < 30 {
		return makeErr(401, "You need to wait at least 30 seconds between password reset attempts"), nil
	}

	user.ResetCode = uuid.NewString()
	user.LastResetRequest = time.Now()

	err = DB_userUpdateResetCode(user)
	if err != nil {
		return makeErr(500, "Database error, please try again in a moment"), nil
	}

	if loadSecret("EmailKey") != "" {
		err = SEND_PASSWORD_RESET(loadSecret("EmailKey"), user.Email, user.ResetCode)
		if err != nil {
			return makeErr(500, "Email system error, please try again in a moment"), nil
		}
	}

	return nil, map[string]string{"status": "password reset code sent"}
}

func APIv2_UserResetPassword(PRF *PASSWORD_RESET_FORM) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	if PRF.Password == "" {
		return makeErr(400, "Missing new password"), nil
	}

	if len(PRF.Password) < 10 {
		return makeErr(400, "password smaller then 10 characters"), nil
	}

	user, err := DB_findUserByEmail(PRF.Email)
	if user == nil {
		return makeErr(401, "Invalid user, please try again"), nil
	}
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	if PRF.UseTwoFactor {
		code, err := Decrypt(user.TwoFactorCode, []byte(loadSecret("TwoFactorKey")))
		if err != nil {
			return makeErr(500, "Two factor authentication error"), nil
		}
		otp := gotp.NewDefaultTOTP(string(code)).Now()
		if otp != PRF.ResetCode {
			return makeErr(401, "Invalid two factor code"), nil
		}
	} else {
		if PRF.ResetCode != user.ResetCode || user.ResetCode == "" {
			return makeErr(401, "Invalid reset code"), nil
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(PRF.Password), 13)
	if err != nil {
		return makeErr(500, "Unable to generate a secure password, please contact customer support"), nil
	}
	user.Password = string(hash)

	err = DB_userResetPassword(user)
	if err != nil {
		return makeErr(401, "Database error, please try again in a moment"), nil
	}

	return nil, map[string]string{"status": "password reset successful"}
}

func APIv2_UserTwoFactorConfirm(TF *TWO_FACTOR_FORM) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", TF.UID, TF.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}
	if user == nil {
		return makeErr(400, "User not found"), nil
	}

	if TF.Recovery != "" {
		recoveryFound := false
		recoveryUpper := strings.ToUpper(TF.Recovery)
		rc, err := Decrypt(user.RecoveryCodes, []byte(loadSecret("TwoFactorKey")))
		if err != nil {
			return makeErr(500, "Encryption error"), nil
		}

		rcs := strings.SplitSeq(string(rc), " ")
		for v := range rcs {
			if v == recoveryUpper {
				recoveryFound = true
			}
		}

		if !recoveryFound {
			return makeErr(401, "Invalid Recovery code"), nil
		}

	} else {
		if user.TwoFactorEnabled {
			return makeErr(401, "This account already has two factor authentication enabled"), nil
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(TF.Password))
	if err != nil {
		return makeErr(401, "Credentials missing or invalid"), nil
	}

	otp := gotp.NewDefaultTOTP(TF.Code).Now()
	if otp != TF.Digits {
		return makeErr(400, "Authenticator code was incorrect"), nil
	}

	updatePackage := new(TWO_FACTOR_DB_PACKAGE)
	updatePackage.UID = user.ID
	updatePackage.Code, err = Encrypt(TF.Code, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		return makeErr(500, "Encryption error"), nil
	}

	recoveryByte := strings.Join([]string{GENERATE_CODE(), GENERATE_CODE()}, " ")

	updatePackage.Recovery, err = Encrypt(recoveryByte, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		return makeErr(500, "Encryption error"), nil
	}

	err = DB_userUpdateTwoFactorCodes(updatePackage)
	if err != nil {
		return makeErr(500, "Database error, please try again in a moment"), nil
	}

	out := make(map[string]any)
	out["Message"] = ""
	out["Data"] = recoveryByte

	return nil, out
}

func APIv2_UserList(F *FORM_LIST_USERS) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to view groups"), nil
		}
	}

	users, err := DB_getUsers(int64(F.Limit), int64(F.Offset))
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	if users == nil {
		return nil, []any{
			// empty response
		}
	}

	for i := range users {
		users[i].RemoveSensitiveInformation()
	}

	return nil, users
}

func APIv2_DeviceDelete(F *FORM_DELETE_DEVICE) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to delete device"), nil
		}
	}

	err = DB_DeleteDeviceByID(F.DID)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	return nil, map[string]string{"status": "device deleted"}
}

func APIv2_DeviceList(F *FORM_LIST_DEVICE, hasAPIKey bool) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	if !hasAPIKey {
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			return makeErr(500, err.Error()), nil
		}

		if !user.IsAdmin {
			if !user.IsManager {
				return makeErr(401, "You are not allowed to view groups"), nil
			}
		}
	}

	devices, err := DB_GetDevices(int64(F.Limit), int64(F.Offset))
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	return nil, devices
}

func APIv2_DeviceCreate(F *FORM_CREATE_DEVICE, hasAPIKey bool) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	if !hasAPIKey {
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			return makeErr(500, err.Error()), nil
		}
		if !user.IsAdmin {
			if !user.IsManager {
				return makeErr(401, "You are not allowed to create devices"), nil
			}
		}
	}

	if F.Device == nil || F.Device.Tag == "" {
		return makeErr(400, "Invalid device format"), nil
	}

	F.Device.ID = primitive.NewObjectID()
	F.Device.CreatedAt = time.Now()
	if F.Device.Groups == nil {
		F.Device.Groups = make([]primitive.ObjectID, 0)
	}

	err := DB_CreateDevice(F.Device)
	if err != nil {
		return makeErr(500, "Unable to create group, please try again later"), nil
	}

	return nil, F.Device
}

func APIv2_DeviceUpdate(F *FORM_UPDATE_DEVICE) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to update devices"), nil
		}
	}

	err = DB_UpdateDevice(F.Device)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	return nil, map[string]string{"status": "device updated"}
}

func APIv2_DeviceGet(F *types.FORM_GET_DEVICE) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	device, err := DB_FindDeviceByID(F.DeviceID)
	if err != nil || device == nil {
		if err != nil {
			return makeErr(400, "device not found"), nil
		} else {
			return makeErr(400, "device not found"), nil
		}
	}

	return nil, device
}

func APIv2_GroupCreate(F *FORM_CREATE_GROUP) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	if F.Group == nil || F.Group.Tag == "" {
		return makeErr(400, "Invalid group format"), nil
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to create groups"), nil
		}
	}

	F.Group.ID = primitive.NewObjectID()
	F.Group.CreatedAt = time.Now()

	err = DB_CreateGroup(F.Group)
	if err != nil {
		return makeErr(500, "Unable to create group, please try again later"), nil
	}

	return nil, F.Group
}

func APIv2_GroupAdd(F *FORM_GROUP_ADD) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to update groups"), nil
		}
	}

	var u *User
	var s *types.Server
	var d *types.Device

	switch F.Type {
	case "device":
		d, err = DB_FindDeviceByID(F.TypeID)
		if err != nil {
			return makeErr(500, err.Error()), nil
		}
	case "server":
		s, err = DB_FindServerByID(F.TypeID)
		if err != nil {
			return makeErr(500, err.Error()), nil
		}
	case "user":
		if F.TypeTag == "email" {
			u, err = DB_findUserByEmail(F.TypeTag)
		} else {
			u, err = DB_findUserByID(F.TypeID)
		}
		if err != nil {
			return makeErr(500, err.Error()), nil
		}
		if u == nil {
			return makeErr(204, "user not found"), nil
		}
		F.TypeID = u.ID
	}

	err = DB_AddToGroup(F.GroupID, F.TypeID, F.Type)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	switch {
	case u != nil:
		return nil, u.ToMinifiedUser()
	case s != nil:
		return nil, s
	case d != nil:
		return nil, d
	default:
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}
}

func APIv2_GroupRemove(F *FORM_GROUP_REMOVE) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to update this entity"), nil
		}
	}

	err = DB_RemoveFromGroup(F.GroupID, F.TypeID, F.Type)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	return nil, map[string]string{"status": "removed from group"}
}

func APIv2_GroupUpdate(F *FORM_UPDATE_GROUP) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to update groups"), nil
		}
	}

	err = DB_UpdateGroup(F.Group)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	return nil, map[string]string{"status": "group updated"}
}

func APIv2_GroupDelete(F *FORM_DELETE_GROUP) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to view groups"), nil
		}
	}

	err = DB_DeleteGroupByID(F.GID)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	return nil, map[string]string{"status": "group deleted"}
}

func APIv2_GroupGet(F *FORM_GET_GROUP) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to view groups"), nil
		}
	}

	group, err := DB_findGroupByID(F.GID)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	if group == nil {
		return nil, map[string]string{"status": "group not found"}
	}

	return nil, group
}

func APIv2_GroupGetEntities(F *FORM_GET_GROUP_ENTITIES) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to view groups"), nil
		}
	}

	entities, err := DB_FindEntitiesByGroupID(F.GID, F.Type, int64(F.Limit), int64(F.Offset))
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	if F.Type == "user" {
		// Handle user entities specifically if needed
		// entities are already processed by the database function
	}

	return nil, entities
}

func APIv2_GroupList(F *FORM_LIST_GROUP) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to view groups"), nil
		}
	}

	groups, err := DB_findGroups()
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	return nil, groups
}

func APIv2_ServersForUser(F *FORM_GET_SERVERS) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}

	servers := make([]*types.Server, 0)
	pservers, err := DB_FindServersWithoutGroups(100, int64(F.StartIndex))
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}
	servers = append(servers, pservers...)

	if len(user.Groups) > 0 {
		puservers, err := DB_FindServersByGroups(user.Groups, 100, int64(F.StartIndex))
		if err != nil {
			return makeErr(500, "Unknown error, please try again in a moment"), nil
		}
		servers = append(servers, puservers...)
	}

	return nil, servers
}

func APIv2_ServerUpdate(F *FORM_UPDATE_SERVER) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(401, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to create servers"), nil
		}
	}

	_, err = DB_UpdateServer(F.Server)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	return nil, map[string]string{"status": "server updated"}
}

func APIv2_ServerCreate(F *FORM_CREATE_SERVER) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		return makeErr(401, err.Error()), nil
	}

	if !user.IsAdmin {
		if !user.IsManager {
			return makeErr(401, "You are not allowed to create servers"), nil
		}
	}

	F.Server.ID = primitive.NewObjectID()
	F.Server.Groups = make([]primitive.ObjectID, 0)
	err = DB_CreateServer(F.Server)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}

	return nil, F.Server
}

func APIv2_ServerGet(F *types.FORM_GET_SERVER) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	server, err := DB_FindServerByID(F.ServerID)
	if err != nil {
		return makeErr(500, err.Error()), nil
	}
	if server == nil {
		return makeErr(404, "unauthorized"), nil
	}

	allowed := false
	if F.DeviceKey != "" {
		deviceID, err := primitive.ObjectIDFromHex(F.DeviceKey)
		if err != nil {
			return makeErr(400, "invalid device key"), nil
		}
		device, err := DB_FindDeviceByID(deviceID)
		if err != nil {
			return makeErr(500, err.Error()), nil
		}
		if device == nil {
			return makeErr(401, "Unauthorized"), nil
		}
		for _, g := range server.Groups {
			for _, ug := range device.Groups {
				if g == ug {
					allowed = true
				}
			}
		}
	} else {
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			return makeErr(401, err.Error()), nil
		}
		for _, ug := range user.Groups {
			for _, sg := range server.Groups {
				if sg == ug {
					allowed = true
				}
			}
		}
	}

	if len(server.Groups) == 0 {
		allowed = true
	}

	if !allowed {
		return makeErr(401, "unauthorized"), nil
	}

	return nil, server
}

func APIv2_UserToggleSubStatus(UF *USER_UPDATE_SUB_FORM) (*ErrorResponse, any) {
	user, err := authenticateUserFromEmailOrIDAndToken(UF.Email, primitive.NilObjectID, UF.DeviceToken)
	if err != nil || user == nil {
		return makeErr(401, "Authentication failed", slog.Any("err", err)), nil
	}

	err = DB_toggleUserSubscriptionStatus(UF)
	if err != nil {
		return makeErr(500, "Failed to toggle subscription status", slog.Any("err", err)), nil
	}

	return nil, map[string]string{"status": "success"}
}

func APIv2_ActivateLicenseKey(AF *KEY_ACTIVATE_FORM) (*ErrorResponse, any) {
	user, err := authenticateUserFromEmailOrIDAndToken("", AF.UID, AF.DeviceToken)
	if err != nil {
		return makeErr(401, "Authentication failed", slog.Any("err", err)), nil
	}

	INFO(3, "KEY attempt:", AF.Key)

	lemonClient := lc.Load()
	key, resp, err := lemonClient.Licenses.Validate(context.Background(), AF.Key, "")
	if err != nil {
		if resp != nil && resp.Body != nil {
			return makeErr(500, "License validation failed", slog.Any("err", err)), nil
		}
		return makeErr(500, "License validation failed", slog.Any("err", err)), nil
	}

	if key.LicenseKey.ActivationUsage > 0 {
		return makeErr(400, "Key is already in use, please contact customer support"), nil
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
			return makeErr(500, "Something went wrong, please contact customer support", slog.Any("err", err)), nil
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
		return makeErr(500, "Failed to activate key", slog.Any("err", err)), nil
	}

	activeKey, resp, err := lemonClient.Licenses.Activate(context.Background(), AF.Key, "tunnels")
	if err != nil {
		if resp != nil && resp.Body != nil {
			return makeErr(500, "License activation failed", slog.Any("err", err)), nil
		}
		return makeErr(500, "License activation failed", slog.Any("err", err)), nil
	}

	if activeKey.Error != "" {
		return makeErr(400, activeKey.Error), nil
	}

	if key != nil {
		INFO(3, "KEY: Activated:", key.LicenseKey.Key)
	}

	return nil, map[string]string{"status": "success"}
}

func APIv2_SessionCreate(CR *types.ControllerConnectRequest) (*ErrorResponse, any) {
	server, err := DB_FindServerByID(CR.ServerID)
	if err != nil {
		return makeErr(500, "Server lookup failed", slog.Any("err", err)), nil
	}
	if server == nil {
		return makeErr(204, "Server not found"), nil
	}

	allowed := false
	if CR.DeviceKey != "" {
		deviceID, err := primitive.ObjectIDFromHex(CR.DeviceKey)
		if err != nil {
			return makeErr(400, "Invalid device key", slog.Any("err", err)), nil
		}
		device, err := DB_FindDeviceByID(deviceID)
		if err != nil {
			return makeErr(500, "Device lookup failed", slog.Any("err", err)), nil
		}
		if device == nil {
			return makeErr(401, "Unauthorized"), nil
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
			return makeErr(401, "Authentication failed", slog.Any("err", err)), nil
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

	if !allowed {
		return makeErr(400, "Unauthorized"), nil
	}

	SCR := new(types.SignedConnectRequest)
	SCR.Payload, err = json.Marshal(CR)
	if err != nil {
		return makeErr(500, "Unable to decode payload", slog.Any("err", err)), nil
	}
	SCR.Signature, err = crypt.SignData(SCR.Payload, PrivKey)
	if err != nil {
		return makeErr(500, "Unable to sign payload", slog.Any("err", err)), nil
	}

	return nil, SCR
}

func APIv2_AcceptUserConnections(SCR *types.SignedConnectRequest) (*ErrorResponse, *types.ServerConnectResponse) {
	err := crypt.VerifySignature(SCR.Payload, SCR.Signature, SignKey)
	if err != nil {
		return makeErr(401, "Invalid signature", slog.Any("err", err)), nil
	}

	CR := new(types.ControllerConnectRequest)
	err = json.Unmarshal(SCR.Payload, CR)
	if err != nil {
		return makeErr(400, "Unable to decode Payload", slog.Any("err", err)), nil
	}

	if time.Since(CR.Created).Seconds() > 30 {
		return makeErr(401, "Request not valid"), nil
	}
	if CR.UserID.IsZero() {
		return makeErr(401, "Invalid user identifier"), nil
	}

	totalC, totalUserC := countConnections(CR.UserID.Hex())
	if CR.RequestingPorts {
		if totalC >= slots {
			return makeErr(400, "Server is full"), nil
		}
	}

	Config := Config.Load()
	if totalUserC > Config.UserMaxConnections {
		return makeErr(400, "User has too many active connections"), nil
	}

	var EH *crypt.SocketWrapper
	EH, err = crypt.NewEncryptionHandler(CR.EncType, CR.CurveType)
	if err != nil {
		ERR("unable to create encryption handler", err)
		return makeErr(500, "Unable to create encryption handler", slog.Any("err", err)), nil
	}

	EH.SEAL.PublicKey, err = EH.SEAL.NewPublicKeyFromBytes(SCR.UserHandshake)
	if err != nil {
		ERR("Port allocation failed", err)
		return makeErr(500, "Port allocation failed", slog.Any("err", err)), nil
	}
	err = EH.SEAL.CreateAEAD()
	if err != nil {
		ERR("Port allocation failed", err)
		return makeErr(500, "Port allocation failed", slog.Any("err", err)), nil
	}

	CRR := types.CreateCRRFromServer(Config)
	index, err := CreateClientCoreMapping(CRR, CR, EH)
	if err != nil {
		ERR("Port allocation failed", err)
		return makeErr(500, "Port allocation failed", slog.Any("err", err)), nil
	}

	CRR.ServerHandshake = EH.GetPublicKey()
	CRR.ServerHandshakeSignature, err = crypt.SignData(CRR.ServerHandshake, PrivKey)
	if err != nil {
		ERR("Unable to sign server handshake", err)
		return makeErr(500, "Unable to sign server handshake", slog.Any("err", err)), nil
	}

	// Setup signal handling for this connection
	clientCoreMappings[index].ToSignal = signal.NewSignal(fmt.Sprintf("TO:%d", index), *CTX.Load(), *Cancel.Load(), time.Second, goroutineLogger, func() {
		toUserChannel(index)
	})

	clientCoreMappings[index].FromSignal = signal.NewSignal(fmt.Sprintf("FROM:%d", index), *CTX.Load(), *Cancel.Load(), time.Second, goroutineLogger, func() {
		fromUserChannel(index)
	})

	return nil, CRR
}

func APIv2_Firewall(fr *types.FirewallRequest) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	mapping := validateDHCPTokenAndIP(fr)
	if mapping == nil {
		return makeErr(401, "Unauthorized"), nil
	}

	syncFirewallState(fr, mapping)

	return nil, map[string]string{"status": "firewall updated"}
}

func APIv2_ListDevices(hasAPIKey bool, F *FORM_LIST_DEVICE) (errData *ErrorResponse, okData any) {
	defer BasicRecover()

	if !hasAPIKey {
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			return makeErr(500, err.Error()), nil
		}
		if !user.IsAdmin {
			if !user.IsManager {
				return makeErr(401, "You are not allowed to list devices"), nil
			}
		}
	}

	response := new(types.DeviceListResponse)
	response.Devices = make([]*types.ListDevice, 0)
outerloop:
	for i := range clientCoreMappings {
		if clientCoreMappings[i] == nil {
			continue
		}

		if clientCoreMappings[i].DHCP != nil {
			for _, v := range response.Devices {
				if v.DHCP.Token == clientCoreMappings[i].DHCP.Token {
					continue outerloop
				}
			}
		}

		d := new(types.ListDevice)
		d.AllowedIPs = make([]string, 0)
		for _, v := range clientCoreMappings[i].AllowedHosts {
			if v.Type == "auto" {
				continue
			}
			d.AllowedIPs = append(d.AllowedIPs,
				fmt.Sprintf("%d-%d-%d-%d",
					v.IP[0],
					v.IP[1],
					v.IP[2],
					v.IP[3],
				))
		}

		d.RAM = clientCoreMappings[i].RAM
		d.CPU = clientCoreMappings[i].CPU
		d.Disk = clientCoreMappings[i].Disk
		if clientCoreMappings[i].DHCP != nil {
			response.DHCPAssigned++
			d.DHCP = types.DHCPRecord{
				DeviceKey: clientCoreMappings[i].DHCP.DeviceKey,
				IP:        clientCoreMappings[i].DHCP.IP,
				Hostname:  clientCoreMappings[i].DHCP.Hostname,
				Token:     clientCoreMappings[i].DHCP.Token,
				Activity:  clientCoreMappings[i].DHCP.Activity,
			}
		}

		d.IngressQueue = len(clientCoreMappings[i].ToUser)
		d.EgressQueue = len(clientCoreMappings[i].FromUser)
		d.Created = clientCoreMappings[i].Created
		if clientCoreMappings[i].PortRange != nil {
			d.StartPort = clientCoreMappings[i].PortRange.StartPort
			d.EndPort = clientCoreMappings[i].PortRange.EndPort
		}
		response.Devices = append(response.Devices, d)
	}

	response.DHCPFree = len(DHCPMapping) - response.DHCPAssigned

	return nil, response
}

func APIv2_Health() (errData *ErrorResponse, okData any) {
	return nil, "OK"
}

// Additional APIv2 functions can be added here as needed for other routes
// For brevity, I'm implementing the most critical ones that demonstrate the pattern
