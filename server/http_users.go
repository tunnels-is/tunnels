package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Set by -disablePublicRegistration. Also honored from config.json
// DisablePublicRegistration (config reload applies without restart).
var disablePublicRegistrationCLI bool

func publicRegistrationDisabled() bool {
	if disablePublicRegistrationCLI {
		return true
	}
	cfg := Config.Load()
	return cfg != nil && cfg.DisablePublicRegistration
}

func handleClientUserCreate(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()
	if publicRegistrationDisabled() {
		senderr(w, 403, "public registration is disabled")
		return
	}
	createUserFromRequest(w, r)
}

func handleAdminUserCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	createUserFromRequest(w, r)
}

func validateRegisterForm(rf *registerRequest) (int, string) {
	if rf.Password == "" {
		return 400, "Missing Password"
	}

	if len(rf.Password) > 72 {
		return 400, "Password is too long, maximum 72 characters"
	}

	if len(rf.Password) < 10 {
		return 400, "Password is too short, minimum 10 characters"
	}

	rf.Email = normalizeEmail(rf.Email)
	if rf.Email == "" {
		return 400, "Email/Username is required"
	}
	if len(rf.Email) > 320 {
		return 400, "Email/Username is too long, maximum 320 characters"
	}
	if isReservedAccountEmail(rf.Email) {
		return 400, "Unable to complete registration"
	}
	return 0, ""
}

func newRegisteredUser(rf *registerRequest) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(rf.Password), 13)
	if err != nil {
		return nil, err
	}

	newUser := new(User)
	newUser.Password = string(hash)
	newUser.ID = uuid.New()
	newUser.Email = rf.Email
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
	return newUser, nil
}

func createUserFromRequest(w http.ResponseWriter, r *http.Request) {
	RF := new(registerRequest)
	err := decodeBody(r, RF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if code, msg := validateRegisterForm(RF); code != 0 {
		senderr(w, code, msg)
		return
	}

	newUser, err := findUserByEmail(RF.Email)
	if newUser != nil {
		senderr(w, 400, "Unable to complete registration")
		return
	}
	if err != nil {
		senderr(w, 500, "Unexpected error, please try again in a moment")
		return
	}

	newUser, err = newRegisteredUser(RF)
	if err != nil {
		senderr(w, 500, "Unable to generate a secure password, please contact customer support")
		return
	}
	err = createUser(newUser)
	if err != nil {
		if errors.Is(err, errEmailRegistered) {
			senderr(w, 400, "Unable to complete registration")
			return
		}
		senderr(w, 500, "Unexpected error, please try again in a moment")
		return
	}

	sendObject(w, newUser)
}

func handleClientUserUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	UF := new(userUpdateRequest)
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
	err = updateUser(UF)
	if err != nil {
		senderr(w, 500, "Unable to update users, please try again in a moment")
		return
	}

	sendObject(w, map[string]string{"APIKey": UF.APIKey})
}

func handleAdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	UF := new(adminUserUpdateRequest)
	err := decodeBody(r, UF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = updateUserAdmin(UF)
	if err != nil {
		senderr(w, 500, "Unable to admin update user, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminUserList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(listUsersRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	users, err := getUsers(int64(clampListLimit(F.Limit)), int64(F.Offset))
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

func handleAdminUserSearch(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(adminUserSearchRequest)
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

	user, err := findUserByEmail(email)
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

func handleAdminUserGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(adminUserGetRequest)
	err := decodeBody(r, F)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if F.TargetUserID == uuid.Nil {
		senderr(w, 400, "TargetUserID is required")
		return
	}

	user, err := findUserByID(F.TargetUserID)
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

func handleAdminUserLatest(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	_ = decodeBody(r, new(listUsersRequest))

	const topN = 100
	const batchSize = 100
	users, total, trial, active, err := getUsersLatest(topN, batchSize)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	for i := range users {
		users[i].RemoveSensitiveInformation()
	}
	sendObject(w, userLatestResponse{
		Users:             users,
		Total:             total,
		Trial:             trial,
		ActiveSubscribers: active,
	})
}

func handleAdminUserDelete(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	F := new(deleteUserRequest)
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

	err = deleteUserByID(F.TargetUserID)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}
