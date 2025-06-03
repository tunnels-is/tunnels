package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"reflect"
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

func API_AcceptUserConnections(w http.ResponseWriter, r *http.Request) {

	SCR := new(types.SignedConnectRequest)
	err := decodeBody(r, SCR)
	if err != nil {
		senderr(w, 400, err.Error())
		return
	}

	err = crypt.VerifySignature(SCR.Payload, SCR.Signature, PubKey)
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

	if RF.Password == "" {
		senderr(w, 400, "Missing Password")
		return
	}

	if len(RF.Password) > 200 {
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
	newUser.Groups = make([]primitive.ObjectID, 0)
	newUser.Tokens = make([]*DeviceToken, 0)

	// if loadSecret("EmailKey") != "" {
	// 	splitEmail := strings.Split(RF.Email, "@")
	// 	if len(splitEmail) > 1 {
	// 		newUser.ConfirmCode = uuid.NewString()
	// 		err = SEND_CONFIRMATION(loadSecret("EmailKey"), newUser.Email, newUser.ConfirmCode)
	// 		if err != nil {
	// 			INFO("unable to send confirm email on signup", err, nil)
	// 			senderr(w, 500, "Email system error, please contact support")
	// 			return
	// 		}
	// 	}
	// }

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

	_, err = authenticateUserFromEmailOrIDAndToken("", UF.UID, UF.DeviceToken)
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

func API_UserLogin(w http.ResponseWriter, r *http.Request) {
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

	user, err := authenticateUserFromEmailOrIDAndToken("", LF.UID, LF.DeviceToken)
	if err != nil {
		senderr(w, 500, err.Error())
		return
	}
	if user == nil {
		senderr(w, 204, "User not found")
		return
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
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_UserTwoFactorConfirm(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	LF := new(TWO_FACTOR_FORM)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err := authenticateUserFromEmailOrIDAndToken("", LF.UID, LF.DeviceToken)
	if err != nil {
		senderr(w, 500, err.Error())
		return
	}
	if user == nil {
		senderr(w, 400, "User not found")
		return
	}

	if LF.Recovery != "" {
		recoveryFound := false
		recoveryUpper := strings.ToUpper(LF.Recovery)
		rc, err := Decrypt(user.RecoveryCodes, []byte(loadSecret("TwoFactorKey")))
		// rc, err := encrypter.Decrypt(user.RecoveryCodes, []byte(ENV.F2KEY))
		if err != nil {
			ADMIN(err)
			senderr(w, 500, "Encryption error")
			return
		}

		rcs := strings.SplitSeq(string(rc), " ")
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

func API_UserList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_USERS)
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

	users, err := DB_getUsers(int64(F.Limit), int64(F.Offset))
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

func API_DeviceUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_DEVICE)
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
			senderr(w, 401, "You are not allowed to update devices")
			return
		}
	}

	err = DB_UpdateDevice(F.Device)
	if err != nil {
		ERR(3, err)
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_DeviceDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_DEVICE)
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
			senderr(w, 401, "You are not allowed to delete device")
			return
		}
	}

	err = DB_DeleteDeviceByID(F.DID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
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
	if !hasAPIKey {
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
	}

	devices, err := DB_GetDevices(int64(F.Limit), int64(F.Offset))
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	sendObject(w, devices)
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

	if !hasAPIKey {
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			senderr(w, 500, err.Error())
			return
		}
		if !user.IsAdmin {
			if !user.IsManager {
				senderr(w, 401, "You are not allowed to create devices")
				return
			}
		}

	}

	if F.Device == nil || F.Device.Tag == "" {
		senderr(w, 400, "Invalid device format")
		return
	}

	F.Device.ID = primitive.NewObjectID()
	F.Device.CreatedAt = time.Now()
	if F.Device.Groups == nil {
		F.Device.Groups = make([]primitive.ObjectID, 0)
	}

	err = DB_CreateDevice(F.Device)
	if err != nil {
		ERR(3, err)
		senderr(w, 500, "Unable to create group, please try again later")
		return
	}

	sendObject(w, F.Device)
}

func API_GroupCreate(w http.ResponseWriter, r *http.Request) {
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
	F.Group.CreatedAt = time.Now()

	err = DB_CreateGroup(F.Group)
	if err != nil {
		ERR(3, err)
		senderr(w, 500, "Unable to create group, please try again later")
		return
	}

	sendObject(w, F.Group)
}

func API_GroupAdd(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GROUP_ADD)
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
			senderr(w, 401, "You are not allowed to update groups")
			return
		}
	}

	var u *User
	var s *types.Server
	var d *types.Device

	switch F.Type {
	case "device":
		d, err = DB_FindDeviceByID(F.TypeID)
		if err != nil {
			senderr(w, 500, err.Error())
			return
		}
	case "server":
		s, err = DB_FindServerByID(F.TypeID)
		if err != nil {
			senderr(w, 500, err.Error())
			return
		}
	case "user":
		if F.TypeTag == "email" {
			u, err = DB_findUserByEmail(F.TypeTag)
		} else {
			u, err = DB_findUserByID(F.TypeID)
		}
		if err != nil {
			senderr(w, 500, err.Error())
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
	case d != nil:
		sendObject(w, d)
	default:
		senderr(w, 500, "Unknown error, please try again in a moment")
	}

}

func API_GroupRemove(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GROUP_REMOVE)
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
			senderr(w, 401, "You are not allowed to update this entity")
			return
		}
	}

	err = DB_RemoveFromGroup(F.GroupID, F.TypeID, F.Type)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}
func API_GroupUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_UPDATE_GROUP)
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

func API_GroupDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_DELETE_GROUP)
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

	err = DB_DeleteGroupByID(F.GID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	// TODO .. remove group from all users and servers

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

	device, err := DB_FindDeviceByID(F.DeviceID)
	if err != nil || device == nil {
		if err != nil {
			senderr(w, 400, "device  not found", slog.Any("err", err))
		} else {
			senderr(w, 400, "device not found")
		}
		return
	}

	sendObject(w, device)
}

func API_GroupGet(w http.ResponseWriter, r *http.Request) {
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

func API_GroupGetEntities(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_GROUP_ENTITIES)
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

func API_GroupList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_LIST_GROUP)
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

	groups, err := DB_findGroups()
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	if groups == nil {
		w.WriteHeader(204)
		return
	}

	sendObject(w, groups)
}

func API_ServersForUser(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(FORM_GET_SERVERS)
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

	sendObject(w, servers)
}

func API_ServerUpdate(w http.ResponseWriter, r *http.Request) {
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

func API_ServerCreate(w http.ResponseWriter, r *http.Request) {
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
	F.Server.Groups = make([]primitive.ObjectID, 0)
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
		senderr(w, 500, err.Error())
		return
	}
	if server == nil {
		senderr(w, 404, "unauthorized")
		return
	}

	allowed := false
	if F.DeviceKey != "" {
		deviceID, err := primitive.ObjectIDFromHex(F.DeviceKey)
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
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			senderr(w, 401, err.Error())
			return
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

	if allowed {
		sendObject(w, server)
		return
	}

	senderr(w, 401, "unauthorized")
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

	var user *User
	RF := new(PASSWORD_RESET_FORM)
	err := decodeBody(r, RF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, err = DB_findUserByEmail(RF.Email)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		senderr(w, 401, "Invalid session token, please log in again")
		return
	}

	if !user.LastResetRequest.IsZero() && time.Since(user.LastResetRequest).Seconds() < 30 {
		senderr(w, 401, "You need to wait at least 30 seconds between password reset attempts")
		return
	}

	user.ResetCode = uuid.NewString()
	user.LastResetRequest = time.Now()

	err = DB_userUpdateResetCode(user)
	if err != nil {
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	err = SEND_PASSWORD_RESET(loadSecret("EmailKey"), user.Email, user.ResetCode)
	if err != nil {
		senderr(w, 500, "Email system  error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func API_UserResetPassword(w http.ResponseWriter, r *http.Request) {
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

	user, err = DB_findUserByEmail(RF.Email)
	if user == nil {
		senderr(w, 401, "Invalid user, please try again")
		return
	}
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if RF.UseTwoFactor {
		code, err := Decrypt(user.TwoFactorCode, []byte(loadSecret("TwoFactorKey")))
		if err != nil {
			ADMIN(err)
			return
		}

		otp := gotp.NewDefaultTOTP(string(code)).Now()
		if otp != RF.ResetCode {
			return
		}
	} else {
		if RF.ResetCode != user.ResetCode || user.ResetCode == "" {
			senderr(w, 401, "Invalid reset code")
			return
		}

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

	w.WriteHeader(200)
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
