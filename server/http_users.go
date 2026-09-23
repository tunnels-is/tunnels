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
		sendError(w, 403, "public registration is disabled")
		return
	}
	createUserFromRequest(w, r)
}

func handleAdminUserCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	createUserFromRequest(w, r)
}

func validateRegisterForm(form *registerRequest) (int, string) {
	if form.Password == "" {
		return 400, "Missing Password"
	}

	if len(form.Password) > 72 {
		return 400, "Password is too long, maximum 72 characters"
	}

	if len(form.Password) < 10 {
		return 400, "Password is too short, minimum 10 characters"
	}

	form.Email = normalizeEmail(form.Email)
	if form.Email == "" {
		return 400, "Email/Username is required"
	}
	if len(form.Email) > 320 {
		return 400, "Email/Username is too long, maximum 320 characters"
	}
	if isReservedAccountEmail(form.Email) {
		return 400, "Unable to complete registration"
	}
	return 0, ""
}

func newRegisteredUser(form *registerRequest) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(form.Password), 13)
	if err != nil {
		return nil, err
	}

	newUser := new(User)
	newUser.Password = string(hash)
	newUser.ID = uuid.New()
	newUser.Email = form.Email
	newUser.Updated = time.Now()
	newUser.Trial = true
	newUser.SubExpiration = time.Now().AddDate(0, 0, 1)
	newUser.Groups = make([]uuid.UUID, 0)
	newUser.Tokens = make([]*DeviceToken, 0)

	token := &DeviceToken{
		N:       "registration",
		DT:      uuid.NewString(),
		Created: time.Now(),
	}

	newUser.DeviceToken = token
	newUser.Tokens = append(newUser.Tokens, token)
	return newUser, nil
}

func createUserFromRequest(w http.ResponseWriter, r *http.Request) {
	form := new(registerRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if code, msg := validateRegisterForm(form); code != 0 {
		sendError(w, code, msg)
		return
	}

	newUser, err := findUserByEmail(form.Email)
	if newUser != nil {
		sendError(w, 400, "Unable to complete registration")
		return
	}
	if err != nil {
		sendError(w, 500, "Unexpected error, please try again in a moment")
		return
	}

	newUser, err = newRegisteredUser(form)
	if err != nil {
		sendError(w, 500, "Unable to generate a secure password, please contact customer support")
		return
	}
	err = createUser(newUser)
	if err != nil {
		if errors.Is(err, errEmailRegistered) {
			sendError(w, 400, "Unable to complete registration")
			return
		}
		sendError(w, 500, "Unexpected error, please try again in a moment")
		return
	}

	sendObject(w, newUser)
}

func handleClientUserUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	form := new(userUpdateRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		sendError(w, 401, "Unauthorized")
		return
	}

	if form.APIKey != "" {
		form.APIKey = uuid.NewString()
	}

	form.UID = user.ID
	err = updateUser(form)
	if err != nil {
		sendError(w, 500, "Unable to update users, please try again in a moment")
		return
	}

	sendObject(w, map[string]string{"APIKey": form.APIKey})
}

func handleAdminUserUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	form := new(adminUserUpdateRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	err = updateUserAdmin(form)
	if err != nil {
		sendError(w, 500, "Unable to admin update user, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleAdminUserList(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(listUsersRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	users, err := getUsers(int64(clampListLimit(form.Limit)), int64(form.Offset))
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
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
	form := new(adminUserSearchRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	email := strings.TrimSpace(form.Email)
	if email == "" {
		sendError(w, 400, "Email is required")
		return
	}

	user, err := findUserByEmail(email)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
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
	form := new(adminUserGetRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}
	if form.TargetUserID == uuid.Nil {
		sendError(w, 400, "TargetUserID is required")
		return
	}

	user, err := findUserByID(form.TargetUserID)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		sendError(w, 404, "user not found")
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
		sendError(w, 500, "Unknown error, please try again in a moment")
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
	form := new(deleteUserRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	caller := getUserFromContext(r.Context())
	if caller != nil && caller.ID == form.TargetUserID {
		sendError(w, 400, "Cannot delete your own account")
		return
	}

	err = deleteUserByID(form.TargetUserID)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}
