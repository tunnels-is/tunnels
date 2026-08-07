package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	mrand "math/rand/v2"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	"github.com/xlzd/gotp"
	"golang.org/x/crypto/bcrypt"
)

func randomAuthDelay() {
	time.Sleep(time.Duration(50+mrand.IntN(100)) * time.Millisecond)
}

func API_AdminUILogin(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()

	defer BasicRecover()

	LF := new(LOGIN_FORM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := DB_findUserByEmail(LF.Email)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		senderr(w, 401, "Invalid login credentials")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(LF.Password))
	if err != nil {
		senderr(w, 401, "Invalid login credentials")
		return
	}

	err = validateUserTwoFactor(user, LF)
	if err != nil {
		senderr(w, 401, err.Error())
		return
	}

	if !user.IsAdmin {
		senderr(w, 401, "Admin or Manager access required")
		return
	}

	userLoginUpdate := handleUserDeviceToken(user, LF)
	err = DB_updateUserDeviceTokens(userLoginUpdate)
	if err != nil {
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	clearPasswordResetAttempts(LF.Email)

	cookieValue, err := encryptAdminCookie(user.ID.String(), user.DeviceToken.DT, clientIP(r))
	if err != nil {
		senderr(w, 500, "Failed to create session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    cookieValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400 * 7,
	})

	user.RemoveSensitiveInformation()
	sendObject(w, user)
}

func API_AdminUILogout(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})

	user := getUserFromContext(r.Context())
	if user != nil {
		LF := new(LOGOUT_FORM)
		_ = decodeBody(r, LF)

		user.Tokens = revokeUserDeviceTokens(user.Tokens, LF)

		update := new(UPDATE_USER_TOKENS)
		update.ID = user.ID
		update.Tokens = user.Tokens
		_ = DB_updateUserDeviceTokens(update)
	}

	w.WriteHeader(200)
}

func API_UserCreate(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()
	RF := new(REGISTER_FORM)
	err := decodeBody(r, RF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if RF.Password == "" {
		senderr(w, 400, "Missing Password")
		return
	}

	if len(RF.Password) > 72 {
		senderr(w, 400, "Password is too long, maximum 72 characters")
		return
	}

	if len(RF.Password) < 10 {
		senderr(w, 400, "Password is too short, minimum 10 characters")
		return
	}

	if len(RF.Email) > 320 {
		senderr(w, 400, "Email/Username is too long, maximum 320 characters")
		return
	}

	newUser, err := DB_findUserByEmail(RF.Email)
	if newUser != nil {
		senderr(w, 400, "User already registered")
		return
	}
	if err != nil {
		senderr(w, 500, "Unexpected error, please try again in a moment")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(RF.Password), 13)
	if err != nil {
		senderr(w, 500, "Unable to generate a secure password, please contact customer support")
		return
	}

	newUser = new(User)
	newUser.Password = string(hash)
	newUser.ID = uuid.New()
	newUser.Email = RF.Email
	newUser.Updated = time.Now()
	newUser.Trial = true
	newUser.SubExpiration = time.Now().AddDate(0, 0, 1)
	newUser.Groups = make([]uuid.UUID, 0)
	newUser.Tokens = make([]*DeviceToken, 0)

	T := new(DeviceToken)
	T.N = "registration"
	T.DT = uuid.NewString()
	T.Created = time.Now()

	newUser.DeviceToken = T
	newUser.Tokens = append(newUser.Tokens, T)
	err = DB_CreateUser(newUser)
	if err != nil {
		senderr(w, 500, "Unexpected error, please try again in a moment")
		return
	}

	sendObject(w, newUser)
}

func API_UserUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	UF := new(USER_UPDATE_FORM)
	err := decodeBody(r, UF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	if UF.APIKey != "" {
		UF.APIKey = uuid.NewString()
	}

	UF.UID = user.ID
	err = DB_updateUser(UF)
	if err != nil {
		senderr(w, 500, "Unable to update users, please try again in a moment")
		return
	}

	sendObject(w, map[string]string{"APIKey": UF.APIKey})
}

func API_UserAdminUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	UF := new(USER_ADMIN_UPDATE_FORM)
	err := decodeBody(r, UF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = DB_updateUserAdmin(UF)
	if err != nil {
		senderr(w, 500, "Unable to admin update user, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_UserLogin(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()

	LF := new(LOGIN_FORM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := DB_findUserByEmail(LF.Email)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		senderr(w, 401, "Invalid login credentials")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(LF.Password))
	if err != nil {
		senderr(w, 401, "Invalid login credentials")
		return
	}

	err = validateUserTwoFactor(user, LF)
	if err != nil {
		senderr(w, 401, err.Error())
		return
	}

	userLoginUpdate := handleUserDeviceToken(user, LF)
	err = DB_updateUserDeviceTokens(userLoginUpdate)
	if err != nil {
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	clearPasswordResetAttempts(LF.Email)

	user.RemoveSensitiveInformation()
	sendObject(w, user)
}

func API_UserLogout(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	LF := new(LOGOUT_FORM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 204, "User not found")
		return
	}

	user.Tokens = revokeUserDeviceTokens(user.Tokens, LF)

	userTokenUpdate := new(UPDATE_USER_TOKENS)
	userTokenUpdate.ID = user.ID
	userTokenUpdate.Tokens = user.Tokens

	err = DB_updateUserDeviceTokens(userTokenUpdate)
	if err != nil {
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_UserTwoFactorConfirm(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()

	LF := new(TWO_FACTOR_FORM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	if LF.Recovery != "" {
		recoveryFound := false
		recoveryUpper := strings.ToUpper(LF.Recovery)
		rc, err := Decrypt(user.RecoveryCodes, []byte(loadSecret("TwoFactorKey")))
		if err != nil {
			ADMIN(err)
			senderr(w, 500, "Encryption error")
			return
		}

		rcs := strings.SplitSeq(rc, " ")
		for v := range rcs {
			if v == recoveryUpper {
				recoveryFound = true
			}
		}

		if !recoveryFound {
			senderr(w, 401, "Invalid Recovery code")
			return
		}
	} else {
		if user.TwoFactorEnabled {
			senderr(w, 401, "This account already has two factor authentication enabled")
			return
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(LF.Password))
	if err != nil {
		senderr(w, 401, "Credentials missing or invalid")
		return
	}

	otp := gotp.NewDefaultTOTP(LF.Code).Now()
	if otp != LF.Digits {
		senderr(w, 400, "Authenticator code was incorrect")
		return
	}

	updatePackage := new(TWO_FACTOR_DB_PACKAGE)
	updatePackage.UID = user.ID
	updatePackage.Code, err = Encrypt(LF.Code, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		senderr(w, 500, "Encryption error")
		return
	}

	recoveryByte := strings.Join([]string{GENERATE_CODE(), GENERATE_CODE()}, " ")

	updatePackage.Recovery, err = Encrypt(recoveryByte, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		senderr(w, 500, "Encryption error")
		return
	}

	err = DB_userUpdateTwoFactorCodes(updatePackage)
	if err != nil {
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	out := make(map[string]any)
	out["Message"] = ""
	out["Data"] = recoveryByte

	sendObject(w, out)
}

func API_AdminUserList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_USERS)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	users, err := DB_getUsers(int64(clampListLimit(F.Limit)), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	if users == nil {
		w.WriteHeader(204)
		return
	}
	for i := range users {
		users[i].RemoveSensitiveInformation()
	}

	sendObject(w, users)
}


func API_AdminUserSearch(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_ADMIN_USER_SEARCH)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	email := strings.TrimSpace(F.Email)
	if email == "" {
		senderr(w, 400, "Email is required")
		return
	}

	user, err := DB_findUserByEmail(email)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		sendObject(w, []*User{})
		return
	}
	user.RemoveSensitiveInformation()
	sendObject(w, []*User{user})
}


func API_AdminUserGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_ADMIN_USER_GET)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if F.TargetUserID == uuid.Nil {
		senderr(w, 400, "TargetUserID is required")
		return
	}

	user, err := DB_findUserByID(F.TargetUserID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		senderr(w, 404, "user not found")
		return
	}
	user.RemoveSensitiveInformation()
	sendObject(w, user)
}


func API_AdminUserLatest(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	_ = decodeBody(r, new(FORM_LIST_USERS))

	const topN = 100
	const batchSize = 100
	users, total, trial, active, err := DB_getUsersLatest(topN, batchSize)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	for i := range users {
		users[i].RemoveSensitiveInformation()
	}
	sendObject(w, USER_LATEST_RESPONSE{
		Users:             users,
		Total:             total,
		Trial:             trial,
		ActiveSubscribers: active,
	})
}

func API_AdminDeviceUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = DB_UpdateDevice(F.Device)
	if err != nil {
		ERR(err)
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminDeviceDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = DB_DeleteDeviceByID(F.DID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_ClientDeviceDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(types.FORM_GET_DEVICE)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	device, err := DB_FindDeviceByID(F.DeviceID)
	if err != nil || device == nil {
		senderr(w, 404, "Device not found")
		return
	}

	if device.UserID != user.ID {
		senderr(w, 401, "You are not allowed to delete this device")
		return
	}

	if err := DB_DeleteDeviceByID(F.DeviceID); err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminDeviceList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	devices, err := DB_GetDevices(int64(clampListLimit(F.Limit)), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, devices)
}

const (
	maxListLimit      = 10000
	defaultListLimit  = 500
	maxDevicesPerUser = 50
)

func clampListLimit(n int) int {
	if n <= 0 {
		return defaultListLimit
	}
	if n > maxListLimit {
		return maxListLimit
	}
	return n
}

func API_ListDevicesByUser(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	devices, err := DB_GetDevicesByUserID(user.ID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, devices)
}

func API_DeviceCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	F := new(FORM_CREATE_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Device == nil || F.Device.Tag == "" {
		senderr(w, 400, "Invalid device format")
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	if !user.SubExpiration.IsZero() && time.Now().After(user.SubExpiration) {
		senderr(w, 403, "subscription expired")
		return
	}

	F.Device.UserID = user.ID
	F.Device.ID = uuid.New()
	F.Device.CreatedAt = time.Now()

	wgServer, srvErr := DB_FindServerByID(F.Device.ServerID)
	if srvErr != nil || wgServer == nil {
		senderr(w, 404, "Server not found")
		return
	}

	if !hasSharedOrNoGroup(user.Groups, wgServer.Groups) {
		senderr(w, 401, "Unauthorized")
		return
	}

	wgIPAllocMu.Lock()
	defer wgIPAllocMu.Unlock()

	existing, listErr := DB_GetDevicesByUserID(user.ID)
	if listErr != nil {
		senderr(w, 500, "Unable to create device, please try again later")
		return
	}
	if len(existing) >= maxDevicesPerUser {
		senderr(w, 400, "device limit reached for this account")
		return
	}

	if F.Device.ServerID != uuid.Nil {
		ip, assignErr := assignNextWireGuardIP(F.Device.ServerID)
		if assignErr != nil {
			senderr(w, 400, "WireGuard IP assignment failed", slog.Any("err", assignErr))
			return
		}
		F.Device.WireGuardIP = ip

		ipv6, assign6Err := assignNextWireGuardIPv6(F.Device.ServerID)
		if assign6Err != nil {
			senderr(w, 400, "WireGuard IPv6 assignment failed", slog.Any("err", assign6Err))
			return
		}
		F.Device.WireGuardIPv6 = ipv6

	}

	err = DB_CreateDevice(F.Device)
	if err != nil {
		ERR(err)
		senderr(w, 500, "Unable to create device, please try again later")
		return
	}

	if wgServer != nil {
		sendObject(w, map[string]any{
			"Device":        F.Device,
			"ServerPubKey":  wgServer.WireGuardPubKey,
			"ServerPort":    strconv.Itoa(wgServer.WireGuardPort),
			"ServerIP":      wgServer.IP,
			"ServerSubnet":  wgServer.WireGuardSubnet,
			"ServerSubnet6": wgServer.WireGuardSubnet6,
			"WANCIDR":       wanCIDRForServer(wgServer),
		})
	} else {
		sendObject(w, F.Device)
	}
}

func API_AdminDeviceCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	F := new(FORM_CREATE_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Device == nil {
		senderr(w, 400, "No device given")
		return
	}

	if F.Device.Tag == "" {
		senderr(w, 400, "Missing device tag")
		return
	}

	if F.Device.UserID == uuid.Nil {
		senderr(w, 400, "Device UserID is required")
		return
	}

	F.Device.ID = uuid.New()
	F.Device.CreatedAt = time.Now()

	wgIPAllocMu.Lock()
	defer wgIPAllocMu.Unlock()

	var wgServer *types.Server
	if F.Device.ServerID != uuid.Nil {
		ip, assignErr := assignNextWireGuardIP(F.Device.ServerID)
		if assignErr != nil {
			senderr(w, 400, "WireGuard IP assignment failed", slog.Any("err", assignErr))
			return
		}
		F.Device.WireGuardIP = ip

		ipv6, assign6Err := assignNextWireGuardIPv6(F.Device.ServerID)
		if assign6Err != nil {
			senderr(w, 400, "WireGuard IPv6 assignment failed", slog.Any("err", assign6Err))
			return
		}
		F.Device.WireGuardIPv6 = ipv6

		var srvErr error
		wgServer, srvErr = DB_FindServerByID(F.Device.ServerID)
		if srvErr != nil || wgServer == nil {
			senderr(w, 404, "Server not found")
			return
		}
	}

	err = DB_CreateDevice(F.Device)
	if err != nil {
		ERR(err)
		senderr(w, 500, "Unable to create device, please try again later")
		return
	}

	if wgServer != nil {
		sendObject(w, map[string]any{
			"Device":        F.Device,
			"ServerPubKey":  wgServer.WireGuardPubKey,
			"ServerPort":    strconv.Itoa(wgServer.WireGuardPort),
			"ServerIP":      wgServer.IP,
			"ServerSubnet":  wgServer.WireGuardSubnet,
			"ServerSubnet6": wgServer.WireGuardSubnet6,
			"WANCIDR":       wanCIDRForServer(wgServer),
		})
	} else {
		sendObject(w, F.Device)
	}
}

func API_AdminDeviceGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(types.FORM_GET_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	device, err := DB_FindDeviceByID(F.DeviceID)
	if err != nil {
		senderr(w, 400, "device not found", slog.Any("err", err))
		return
	}
	if device == nil {
		senderr(w, 400, "device not found")
		return
	}

	sendObject(w, device)
}

func API_AdminServerGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(types.FORM_GET_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	server, err := DB_FindServerByID(F.ServerID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment", slog.Any("error", err))
		return
	}
	if server == nil {
		senderr(w, 404, "server not found")
		return
	}

	attachWANs(server)
	sendObject(w, server)
}

func API_AdminServersList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_SERVERS)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	servers, err := DB_FindAllServers(100, int64(F.StartIndex))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	attachWANs(servers...)
	sendObject(w, servers)
}

func API_AdminGroupCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_CREATE_GROUP)
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

	err = DB_CreateGroup(F.Group)
	if err != nil {
		ERR(err)
		senderr(w, 500, "Unable to create group, please try again later")
		return
	}

	sendObject(w, F.Group)
}

func API_AdminGroupAdd(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GROUP_ADD)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	var u *User
	var s *types.Server

	switch F.Type {
	case "server":
		s, err = DB_FindServerByID(F.TypeID)
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
			u, err = DB_findUserByEmail(F.TypeTag)
		} else {
			u, err = DB_findUserByID(F.TypeID)
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

	err = DB_AddToGroup(F.GroupID, F.TypeID, F.Type)
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

func API_AdminGroupRemove(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GROUP_REMOVE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = DB_RemoveFromGroup(F.GroupID, F.TypeID, F.Type)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminGroupUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = DB_UpdateGroup(F.Group)
	if err != nil {
		ERR(err)
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminGroupDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = DB_DeleteGroupByID(F.GID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminUserDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_USER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	caller := getUserFromContext(r.Context())
	if caller != nil && caller.ID == F.TargetUserID {
		senderr(w, 400, "Cannot delete your own account")
		return
	}

	err = DB_DeleteUserByID(F.TargetUserID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminServerDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = DB_DeleteServerByID(F.ServerID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_DeviceGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(types.FORM_GET_DEVICE)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 400, "user not found")
		return
	}

	device, err := DB_FindDeviceByID(F.DeviceID)
	if err != nil || device == nil {
		if err != nil {
			senderr(w, 400, "device  not found", slog.Any("err", err))
		} else {
			senderr(w, 400, "device not found")
		}
		return
	}

	if device.UserID != user.ID {
		senderr(w, 400, "unauthorized")
		return
	}

	sendObject(w, device)
}

func API_AdminGroupGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	group, err := DB_findGroupByID(F.GID)
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

func API_AdminGroupGetEntities(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_GROUP_ENTITIES)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	entities, err := DB_FindEntitiesByGroupID(F.GID, F.Type, int64(F.Limit), int64(F.Offset))
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

func API_AdminGroupList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	limit := F.Limit
	if limit <= 0 {
		limit = 100
	}

	groups, err := DB_ListGroups(int64(limit), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, groups)
}

func sanitizeServerForClient(s *types.Server) *types.Server {
	c := *s
	c.APIKey = ""
	return &c
}

func findServersForUser(user *User, offset int64) ([]*types.Server, error) {
	servers, err := DB_FindServersWithoutGroups(10000, offset)
	if err != nil {
		return nil, err
	}
	if len(user.Groups) > 0 {
		grouped, err := DB_FindServersByGroups(user.Groups, 10000, offset)
		if err != nil {
			return nil, err
		}
		servers = append(servers, grouped...)
	}
	return servers, nil
}

func API_ServersForUserByCountry(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_SERVERS_BY_COUNTRY)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if F.Country == "" {
		senderr(w, 400, "Country is required")
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	all, err := findServersForUser(user, 0)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	matched := make([]*types.Server, 0)
	for _, s := range all {
		if strings.EqualFold(s.Country, F.Country) {
			matched = append(matched, s)
		}
	}

	mrand.Shuffle(len(matched), func(i, j int) {
		matched[i], matched[j] = matched[j], matched[i]
	})
	if len(matched) > 10 {
		matched = matched[:10]
	}

	for i, s := range matched {
		matched[i] = sanitizeServerForClient(s)
	}

	attachWANs(matched...)
	sendObject(w, matched)
}

func API_ServersForUser(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_SERVERS)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	servers := make([]*types.Server, 0)
	pservers, err := DB_FindServersWithoutGroups(100, int64(F.StartIndex))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	servers = append(servers, pservers...)

	if len(user.Groups) > 0 {
		puservers, err := DB_FindServersByGroups(user.Groups, 100, int64(F.StartIndex))
		if err != nil {
			senderr(w, 500, "Unknown error, please try again in a moment")
			return
		}
		servers = append(servers, puservers...)
	}

	for i, s := range servers {
		servers[i] = sanitizeServerForClient(s)
	}

	attachWANs(servers...)
	sendObject(w, servers)
}

func applyWGDefaults(s *types.Server) {
	if s.APIKey == "" {
		s.APIKey = uuid.NewString()
	}
	if s.WireGuardPort == 0 {
		s.WireGuardPort = 51820
	}
	if s.WireGuardMeshPort == 0 {
		s.WireGuardMeshPort = s.WireGuardPort + 1
	}
	if s.WireGuardIface == "" {
		s.WireGuardIface = "wg0"
	}
}

func validateServerWGFields(s *types.Server) error {
	if err := validateCIDR(s.WireGuardSubnet); err != nil {
		return fmt.Errorf("invalid WireGuardSubnet: %w", err)
	}
	if err := validateCIDR(s.WireGuardSubnet6); err != nil {
		return fmt.Errorf("invalid WireGuardSubnet6: %w", err)
	}
	return nil
}

func API_AdminServerUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	F := new(FORM_UPDATE_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Server == nil {
		senderr(w, 400, "Server is required")
		return
	}
	if err := validateServerWGFields(F.Server); err != nil {
		senderr(w, 400, err.Error())
		return
	}
	applyWGDefaults(F.Server)
	if err := validateServerMesh(F.Server); err != nil {
		senderr(w, 400, err.Error())
		return
	}

	_, err = DB_UpdateServer(F.Server)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_AdminServerCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_CREATE_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if F.Server == nil {
		senderr(w, 400, "Server is required")
		return
	}
	if err := validateServerWGFields(F.Server); err != nil {
		senderr(w, 400, err.Error())
		return
	}
	applyWGDefaults(F.Server)
	F.Server.ID = uuid.New()
	if err := validateServerMesh(F.Server); err != nil {
		senderr(w, 400, err.Error())
		return
	}

	F.Server.Groups = make([]uuid.UUID, 0)
	err = DB_CreateServer(F.Server)
	if err != nil {
		senderr(w, 500, "Uknown error, please try again in a moment", slog.Any("err", err))
		return
	}

	sendObject(w, F.Server)
}

func API_ServerGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(types.FORM_GET_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	server, err := DB_FindServerByID(F.ServerID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment", slog.Any("error", err))
		return
	}
	if server == nil {
		senderr(w, 404, "Server not found")
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	if !hasSharedOrNoGroup(user.Groups, server.Groups) {
		senderr(w, 401, "unauthorized")
		return
	}

	sc := sanitizeServerForClient(server)
	attachWANs(sc)
	sendObject(w, sc)
}

func API_UserResetPassword(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()

	var user *User
	RF := new(PASSWORD_RESET_FORM)
	err := decodeBody(r, RF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if len(RF.Password) < 10 {
		senderr(w, 400, "password smaller then 10 characters")
		return
	}
	if len(RF.Password) > 72 {
		senderr(w, 400, "Password is too long, maximum 72 characters")
		return
	}

	const genericAuthErr = "invalid email or reset code"
	const rateLimitErr = "too many attempts, try again later"

	if !passwordResetAllowed(RF.Email) {
		senderr(w, 429, rateLimitErr)
		return
	}

	user, err = DB_findUserByEmail(RF.Email)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		recordPasswordResetFailure(RF.Email)
		senderr(w, 401, genericAuthErr)
		return
	}

	code, err := Decrypt(user.TwoFactorCode, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		recordPasswordResetFailure(RF.Email)
		senderr(w, 401, genericAuthErr)
		return
	}

	otp := gotp.NewDefaultTOTP(code).Now()
	if otp != RF.ResetCode {
		recordPasswordResetFailure(RF.Email)
		senderr(w, 401, genericAuthErr)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(RF.Password), 13)
	if err != nil {
		senderr(w, 500, "Unable to generate a secure password, please contact customer support")
		return
	}
	user.Password = string(hash)

	err = DB_userResetPassword(user)
	if err != nil {
		senderr(w, 401, "Database error, please try again in a moment")
		return
	}

	clearPasswordResetAttempts(RF.Email)
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

	user := getUserFromContext(r.Context())
	if user == nil {
		senderr(w, 401, "Unauthorized")
		return
	}

	INFO("KEY attempt:", redactKey(AF.Key))

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

		base := user.SubExpiration
		if base.Before(time.Now()) {
			base = time.Now()
		}
		jitter, _ := rand.Int(rand.Reader, big.NewInt(60))
		user.SubExpiration = base.AddDate(0, 1, 0).Add(time.Duration(jitter.Int64()+60) * time.Minute)
		INFO("KEY +1:", redactKey(key.LicenseKey.Key), " - check activation in lemon")

		user.Key = &LicenseKey{
			Created: key.LicenseKey.CreatedAt,
			Months:  1,
			Key:     "unknown",
		}
	} else {
		ns := strings.Split(key.Meta.ProductName, " ")
		months, err := strconv.Atoi(ns[0])
		if err != nil {
			ADMIN("unable to parse license key name:", err)
			senderr(w, 500, "Something went wrong, please contact customer support")
			return
		}

		base := user.SubExpiration
		if base.Before(time.Now()) {
			base = time.Now()
		}
		jitter2, _ := rand.Int(rand.Reader, big.NewInt(600))
		user.SubExpiration = base.AddDate(0, months, 0).Add(time.Duration(jitter2.Int64()+60) * time.Minute)
		INFO("KEY +", months, ":", redactKey(key.LicenseKey.Key), " - check activate in lemon")

		user.Key = &LicenseKey{
			Created: key.LicenseKey.CreatedAt,
			Months:  months,
			Key:     key.LicenseKey.Key,
		}
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

	user.Trial = false
	user.Disabled = false
	err = DB_UserActivateKey(user.SubExpiration, user.Key, user.ID)
	if err != nil {
		senderr(w, 500, "unexpected error, please contact support")
		return
	}

	if key != nil {
		INFO("KEY: Activated:", redactKey(key.LicenseKey.Key))
	}

	w.WriteHeader(200)
}
