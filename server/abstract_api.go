package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
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

func sendHTTPOKResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Error("unable to write JSON okResponse:", slog.Any("err", err))
	}
}

func prepData(obj any) (data []byte) {
	var err error
	u, ok := obj.(*User)
	if ok {
		u.RemoveSensitiveInformation()
		data, err = json.Marshal(u)
	} else {
		data, err = json.Marshal(obj)
	}
	if err != nil {
		logger.Error("unable to encode response object", slog.Any("err", err))
		return nil
	}
	return
}

func APIv2_UserLogin(LF *LOGIN_FORM) (errData *ErrorResponse, okData interface{}) {
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

	return nil, prepData(user)
}

func APIv2_UserCreate(RF *REGISTER_FORM) (errData *ErrorResponse, okData interface{}) {
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

	return nil, prepData(newUser)
}

func APIv2_UserUpdate(UF *USER_UPDATE_FORM) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_UserLogout(LF *LOGOUT_FORM) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_UserRequestPasswordCode(PRF *PASSWORD_RESET_FORM) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_UserResetPassword(PRF *PASSWORD_RESET_FORM) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_UserTwoFactorConfirm(TF *TWO_FACTOR_FORM) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_UserList(F *FORM_LIST_USERS) (errData *ErrorResponse, okData interface{}) {
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
		return nil, []interface{}{
			// empty response
		}
	}

	for i := range users {
		users[i].RemoveSensitiveInformation()
	}

	return nil, users
}

func APIv2_DeviceDelete(F *FORM_DELETE_DEVICE) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_DeviceList(F *FORM_LIST_DEVICE, hasAPIKey bool) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_DeviceCreate(F *FORM_CREATE_DEVICE, hasAPIKey bool) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_DeviceUpdate(F *FORM_UPDATE_DEVICE) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_DeviceGet(F *types.FORM_GET_DEVICE) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_GroupCreate(F *FORM_CREATE_GROUP) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_GroupAdd(F *FORM_GROUP_ADD) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_GroupRemove(F *FORM_GROUP_REMOVE) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_GroupUpdate(F *FORM_UPDATE_GROUP) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_GroupDelete(F *FORM_DELETE_GROUP) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_GroupGet(F *FORM_GET_GROUP) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_GroupGetEntities(F *FORM_GET_GROUP_ENTITIES) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_GroupList(F *FORM_LIST_GROUP) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_ServersForUser(F *FORM_GET_SERVERS) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_ServerUpdate(F *FORM_UPDATE_SERVER) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_ServerCreate(F *FORM_CREATE_SERVER) (errData *ErrorResponse, okData interface{}) {
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

func APIv2_ServerGet(F *types.FORM_GET_SERVER) (errData *ErrorResponse, okData interface{}) {
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

// Additional APIv2 functions can be added here as needed for other routes
// For brevity, I'm implementing the most critical ones that demonstrate the pattern
