package main

import (
	"log/slog"
	mrand "math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/xlzd/gotp"
	"golang.org/x/crypto/bcrypt"
)

func authenticatePasswordLogin(w http.ResponseWriter, form *loginRequest) (user *User, email string) {
	email = normalizeEmail(form.Email)
	if email == "" || !passwordResetAllowed(email) {
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}

	user, err := findUserByEmail(email)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return nil, email
	}
	if user == nil {
		recordPasswordResetFailure(email)
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(form.Password))
	if err != nil {
		recordPasswordResetFailure(email)
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}

	err = validateUserTwoFactor(user, form)
	if err != nil {
		recordPasswordResetFailure(email)
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}

	if user.Disabled {
		sendError(w, 403, "This account has been disabled, please contact customer support")
		return nil, email
	}
	return user, email
}

func randomAuthDelay() {
	time.Sleep(time.Duration(50+mrand.IntN(100)) * time.Millisecond)
}

func handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()

	defer BasicRecover()

	form := new(loginRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, email := authenticatePasswordLogin(w, form)
	if user == nil {
		return
	}

	if !user.IsAdmin {
		sendError(w, 401, "Admin or Manager access required")
		return
	}

	userLoginUpdate := handleUserDeviceToken(user, form)
	err = updateUserDeviceTokens(userLoginUpdate)
	if err != nil {
		sendError(w, 500, "Database error, please try again in a moment")
		return
	}

	clearPasswordResetAttempts(email)

	cookieValue, err := encryptAdminCookie(user.ID.String(), user.DeviceToken.DT, clientIP(r))
	if err != nil {
		sendError(w, 500, "Failed to create session")
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

func handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	user := getUserFromContext(r.Context())
	if user != nil {
		form := new(logoutRequest)
		_ = decodeBody(r, form)
		if !form.All && form.LogoutToken == "" {
			form.LogoutToken = getDeviceTokenFromContext(r.Context())
		}

		user.Tokens = revokeUserDeviceTokens(user.Tokens, form)

		update := new(userTokensUpdate)
		update.ID = user.ID
		update.Tokens = user.Tokens
		if err := updateUserDeviceTokens(update); err != nil {
			sendError(w, 500, "Database error, please try again in a moment")
			return
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "admin_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(200)
}

func handleClientLogin(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()

	form := new(loginRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, email := authenticatePasswordLogin(w, form)
	if user == nil {
		return
	}

	userLoginUpdate := handleUserDeviceToken(user, form)
	err = updateUserDeviceTokens(userLoginUpdate)
	if err != nil {
		sendError(w, 500, "Database error, please try again in a moment")
		return
	}

	clearPasswordResetAttempts(email)

	user.RemoveSensitiveInformation()
	sendObject(w, user)
}

func handleClientLogout(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	form := new(logoutRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		sendError(w, 204, "User not found")
		return
	}

	if !form.All && form.LogoutToken == "" {
		if form.DeviceToken != "" {
			form.LogoutToken = form.DeviceToken
		} else {
			form.LogoutToken = getDeviceTokenFromContext(r.Context())
		}
	}

	user.Tokens = revokeUserDeviceTokens(user.Tokens, form)

	userTokenUpdate := new(userTokensUpdate)
	userTokenUpdate.ID = user.ID
	userTokenUpdate.Tokens = user.Tokens

	err = updateUserDeviceTokens(userTokenUpdate)
	if err != nil {
		sendError(w, 500, "Database error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleClientTwoFactorConfirm(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()

	form := new(twoFactorRequest)
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

	if form.Recovery != "" {
		ok, recErr := recoveryCodePresent(user.RecoveryCodes, form.Recovery)
		if recErr != nil {
			ADMIN(recErr)
			sendError(w, 500, "Encryption error")
			return
		}
		if !ok {
			sendError(w, 401, "Invalid Recovery code")
			return
		}
	} else {
		if user.TwoFactorEnabled {
			sendError(w, 401, "This account already has two factor authentication enabled")
			return
		}
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(form.Password))
	if err != nil {
		sendError(w, 401, "Credentials missing or invalid")
		return
	}

	otp := gotp.NewDefaultTOTP(form.Code).Now()
	if otp != form.Digits {
		sendError(w, 400, "Authenticator code was incorrect")
		return
	}

	updatePackage := new(twoFactorUpdate)
	updatePackage.UID = user.ID
	updatePackage.Code, err = Encrypt(form.Code, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		sendError(w, 500, "Encryption error")
		return
	}

	recoveryByte := strings.Join([]string{generateCode(), generateCode()}, " ")

	updatePackage.Recovery, err = Encrypt(recoveryByte, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		sendError(w, 500, "Encryption error")
		return
	}

	err = updateUserTwoFactorCodes(updatePackage)
	if err != nil {
		sendError(w, 500, "Database error, please try again in a moment")
		return
	}

	out := make(map[string]any)
	out["Message"] = ""
	out["Data"] = recoveryByte

	sendObject(w, out)
}

func recoveryCodePresent(blob []byte, recovery string) (bool, error) {
	recoveryUpper := strings.ToUpper(recovery)
	rc, err := Decrypt(blob, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		return false, err
	}
	for v := range strings.SplitSeq(rc, " ") {
		if v == recoveryUpper {
			return true, nil
		}
	}
	return false, nil
}
