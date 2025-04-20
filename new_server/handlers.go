package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/monorepo/email"
	"github.com/tunnels-is/tunnels/types"
	"github.com/xlzd/gotp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

func APICreateUser(w http.ResponseWriter, r *http.Request) {
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
		senderr(w, 400, "Password is too long, maximum 255 characters")
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
	}

	newUser = new(User)
	newUser.Password = string(hash)
	newUser.ID = primitive.NewObjectID()
	newUser.AdditionalInformation = RF.AdditionalInformation
	newUser.Email = RF.Email
	newUser.Updated = time.Now()

	newUser.Trial = true
	newUser.SubExpiration = time.Now().AddDate(0, 0, 1)

	splitEmail := strings.Split(RF.Email, "@")
	if len(splitEmail) > 1 {
		newUser.ConfirmCode = uuid.NewString()
		err = email.SEND_CONFIRMATION(loadSecret("EmailKey"), newUser.Email, newUser.ConfirmCode)
		if err != nil {
			INFO("unable to send confirm email on signup", err, nil)
			senderr(w, 500, "Email system error, please contact support")
			return
		}
	}

	err = DB_CreateUser(newUser)
	if err != nil {
		senderr(w, 500, "Unexpected error, please try again in a moment")
		return
	}

	sendObject(w, newUser)
}

func APIUpdateUser(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	UF := new(USER_UPDATE_FORM)
	err := decodeBody(r, UF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	_, err = authenticateUserFromEmailOrIDAndToken("", UF.UserID, UF.DeviceToken)
	if err != nil {
		senderr(w, 401, err.Error())
		return
	}

	err = DB_updateUser(UF)
	if err != nil {
		senderr(w, 500, "Unable to update users, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func APILoginUser(w http.ResponseWriter, r *http.Request) {
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
	if user.Email == "" {
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

	if user.Key != nil {
		ks := strings.Split(user.Key.Key, "-")
		if len(ks) < 1 {
			user.Key.Key = "redacted"
		} else {
			user.Key.Key = ks[len(ks)-1]
		}
	}

	user.RemoveSensitiveInformation()
	sendObject(w, user)
}

func APILogoutUser(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	LF := new(LOGOUT_FORM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	user, err := authenticateUserFromEmailOrIDAndToken("", LF.UserID, LF.DeviceToken)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		senderr(w, 204, "User not found")
		return
	}

	if LF.All {
		user.Tokens = make([]*DeviceToken, 0)
	} else {
		slices.DeleteFunc(user.Tokens, func(dt *DeviceToken) bool {
			if dt.DT == LF.DeviceToken {
				return true
			}
			return false
		})
	}

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

func APITwoFactorConfirm(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	var user *User
	LF := new(TWO_FACTOR_CONFIRM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err = DB_findUserByEmail(LF.Email)
	if user == nil {
		senderr(w, 401, "Invalid session token, please log in again")
		return
	}

	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	if LF.Recovery != "" {
		var recoveryFound bool = false
		recoveryUpper := strings.ToUpper(LF.Recovery)
		rc, err := Decrypt(user.RecoveryCodes, []byte(loadSecret("TwoFactory")))
		// rc, err := encrypter.Decrypt(user.RecoveryCodes, []byte(ENV.F2KEY))
		if err != nil {
			ADMIN(err)
			senderr(w, 500, "Encryption error")
			return
		}

		rcs := strings.Split(string(rc), " ")
		for _, v := range rcs {
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

func APICreateGroup(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_CREATE_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		senderr(w, 500, err.Error())
		return
	}

	if !user.IsAdmin {
		if !user.IsManager {
			senderr(w, 401, "You are not allowed to create groups")
			return
		}
	}

	F.Group.ID = primitive.NewObjectID()

	err = DB_CreateGroup(F.Group)
	if err != nil {
		ERR(3, err)
		senderr(w, 500, "Unable to create group, please try again later")
		return
	}

	sendObject(w, F.Group)
}
func APIAddToGroup(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GROUP_ADD)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UserID, F.DeviceToken)
	if err != nil {
		senderr(w, 500, err.Error())
		return
	}

	if !user.IsAdmin {
		if !user.IsManager {
			senderr(w, 401, "You are not allowed to update groups")
			return
		}
	}

	err = DB_AddToGroup(F.GroupID, F.TypeID, F.Type)
	if err != nil {
		ERR(3, err)
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func APIUpdateGroup(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UserID, F.DeviceToken)
	if err != nil {
		senderr(w, 500, err.Error())
		return
	}

	if !user.IsAdmin {
		if !user.IsManager {
			senderr(w, 401, "You are not allowed to update groups")
			return
		}
	}

	err = DB_UpdateGroup(F.Group)
	if err != nil {
		ERR(3, err)
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func APIGetGroup(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_GROUP)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		senderr(w, 500, err.Error())
		return
	}

	if !user.IsAdmin {
		if !user.IsManager {
			senderr(w, 401, "You are not allowed to view groups")
			return
		}
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

func APIGetServers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_SERVERS)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	servers := make([]*Server, 0)
	pservers, err := DB_FindServersWithoutGroups(100, int64(F.StartIndex))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	servers = append(servers, pservers...)

	puservers, err := DB_FindServersByGroups(user.Groups, 100, int64(F.StartIndex))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	servers = append(servers, puservers...)

	sendObject(w, servers)
}

func APIUpdateServer(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	F := new(FORM_UPDATE_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		senderr(w, 401, err.Error())
		return
	}

	if !user.IsAdmin {
		if !user.IsManager {
			senderr(w, 401, "You are not allowed to create servers")
			return
		}
	}

	_, err = DB_UpdateServer(F.Server)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func APICreateServer(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_CREATE_SERVER)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
	if err != nil {
		senderr(w, 401, err.Error())
		return
	}

	if !user.IsAdmin {
		if !user.IsManager {
			senderr(w, 401, "You are not allowed to create servers")
			return
		}
	}

	F.Server.ID = primitive.NewObjectID()
	err = DB_CreateServer(F.Server)
	if err != nil {
		senderr(w, 500, "Uknown error, please try again in a moment", slog.Any("err", err))
		return
	}

	sendObject(w, F.Server)
}

func APIConnectVPN(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	CR := new(types.ConnectRequest)
	err := decodeBody(r, CR)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken(CR.UserEmail, CR.UserID, CR.UserToken)
	if err != nil {
		senderr(w, 401, err.Error())
		return
	}

	// _, code, err := ValidateSubscription(c, CR)
	// if err != nil {
	// 	return WriteErrorResponse(c, code, err.Error())
	// }

	server, err := DB_FindServerByID(CR.SeverID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if server == nil {
		senderr(w, 204, "Server not found")
		return
	}

	allowed := false
	for _, g := range server.Groups {
		for _, ug := range user.Groups {
			if g == ug {
				allowed = true
			}
		}
	}

	if len(server.Groups) == 0 {
		allowed = true
	}

	if !allowed {
		senderr(w, 400, "Unauthorized")
		return
	}

	SCR := new(types.SignedConnectRequest)
	SCR.Payload, err = json.Marshal(CR)
	SCR.Signature, err = signData(SCR.Payload)
	if err != nil {
		senderr(w, 500, "Unable to sign payload")
		return
	}

	sendObject(w, SCR)
}
