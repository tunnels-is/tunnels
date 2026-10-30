package main

import (
	"errors"
	"log/slog"
	mrand "math/rand/v2"
	"net/http"
	"strings"
	"time"

	"github.com/xlzd/gotp"
	"golang.org/x/crypto/bcrypt"
)

// dummyPasswordHash is a cost-13 bcrypt hash. A missing-user login and a
// registration that returns "Unable to complete registration" compare against
// it and ignore the result, so those answers take about as long as a real
// password check. The plaintext is not a credential. A match still fails.
const dummyPasswordHash = "$2a$13$a7TIGconfU4iJBKBkJlQhOHyKAgbGOKmibCU2B88En4cM2hVFQOmO"

func equalizePasswordCost(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
}

func authenticatePasswordLogin(w http.ResponseWriter, r *http.Request, form *loginRequest) (user *User, email string) {
	email = normalizeEmail(form.Email)
	if email == "" {
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}
	ip := clientIP(r)
	if !takeAuthSlot(authLimitLogin, email, ip) {
		sendError(w, 429, "too many attempts, try again later")
		return nil, email
	}

	user, err := findUserByEmail(email)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return nil, email
	}
	if user == nil {
		equalizePasswordCost(form.Password)
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(form.Password))
	if err != nil {
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}

	// A disabled account pays the same bcrypt cost and returns the same 401
	// as a wrong password. The check is before 2FA so a correct password does
	// not add a decrypt on this path.
	if user.Disabled {
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}

	err = validateUserTwoFactor(user, form)
	if err != nil {
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}
	clearAuthSlot(authLimitLogin, email, ip)
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

	user, _ := authenticatePasswordLogin(w, r, form)
	if user == nil {
		return
	}

	// A valid password for a non-admin gets the same 401 as a wrong password.
	// There is no manager role on this server.
	if !user.IsAdmin {
		sendError(w, 401, "Invalid login credentials")
		return
	}

	if err := issueLoginDeviceToken(user, form); err != nil {
		sendError(w, 500, "Database error, please try again in a moment")
		return
	}

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
	// The session cookie already holds this token. The admin page does not
	// read it, and the raw value authenticates to the client API from any IP.
	user.DeviceToken = nil
	for _, t := range user.Tokens {
		if t != nil {
			t.DT = ""
		}
	}
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

		if err := saveLogoutDeviceTokens(user.ID, form); err != nil {
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

	user, _ := authenticatePasswordLogin(w, r, form)
	if user == nil {
		return
	}

	if err := issueLoginDeviceToken(user, form); err != nil {
		sendError(w, 500, "Database error, please try again in a moment")
		return
	}

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

	if err := saveLogoutDeviceTokens(user.ID, form); err != nil {
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

	secret := strings.TrimSpace(form.Code)
	if !validTOTPSecret(secret) {
		sendError(w, 400, "invalid two-factor secret")
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(form.Password))
	if err != nil {
		sendError(w, 401, "Credentials missing or invalid")
		return
	}

	otp := gotp.NewDefaultTOTP(secret).Now()
	if otp != form.Digits {
		sendError(w, 400, "Authenticator code was incorrect")
		return
	}

	updatePackage := new(twoFactorUpdate)
	updatePackage.UID = user.ID
	updatePackage.ConsumeRecovery = strings.TrimSpace(form.Recovery)
	updatePackage.AcceptedTOTP = otp
	updatePackage.Code, err = Encrypt(secret, []byte(loadSecret("TwoFactorKey")))
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

	recoveryConsumeMu.Lock()
	err = updateUserTwoFactorCodes(updatePackage)
	recoveryConsumeMu.Unlock()
	if err != nil {
		if errors.Is(err, errRecoveryNotFound) {
			sendError(w, 401, "Invalid Recovery code")
			return
		}
		if errors.Is(err, errTwoFactorAlreadyEnabled) {
			sendError(w, 401, "This account already has two factor authentication enabled")
			return
		}
		if errors.Is(err, errRecoveryDecrypt) {
			ADMIN(err)
			sendError(w, 500, "Encryption error")
			return
		}
		sendError(w, 500, "Database error, please try again in a moment")
		return
	}

	out := make(map[string]any)
	out["Message"] = ""
	out["Data"] = recoveryByte

	sendObject(w, out)
}

// validTOTPSecret reports whether secret is base32 that gotp can decode.
// An empty secret decodes, and NewDefaultTOTP would still panic later, so
// empty is rejected here. The check matches secrets the client already issues.
func validTOTPSecret(secret string) bool {
	if secret == "" {
		return false
	}
	return gotp.IsSecretValid(secret)
}
