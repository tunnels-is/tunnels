package main

import (
	"context"
	"crypto/rand"
	"log/slog"
	"math/big"
	mrand "math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xlzd/gotp"
	"golang.org/x/crypto/bcrypt"
)

func authenticatePasswordLogin(w http.ResponseWriter, lf *loginRequest) (user *User, email string) {
	email = normalizeEmail(lf.Email)
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

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(lf.Password))
	if err != nil {
		recordPasswordResetFailure(email)
		sendError(w, 401, "Invalid login credentials")
		return nil, email
	}

	err = validateUserTwoFactor(user, lf)
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

func handleClientResetPassword(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()

	var user *User
	form := new(passwordResetRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if len(form.Password) < 10 {
		sendError(w, 400, "password smaller then 10 characters")
		return
	}
	if len(form.Password) > 72 {
		sendError(w, 400, "Password is too long, maximum 72 characters")
		return
	}

	const genericAuthErr = "invalid email or reset code"
	const rateLimitErr = "too many attempts, try again later"

	form.Email = normalizeEmail(form.Email)
	if form.Email == "" {
		sendError(w, 401, genericAuthErr)
		return
	}

	if !passwordResetAllowed(form.Email) {
		sendError(w, 429, rateLimitErr)
		return
	}

	user, err = findUserByEmail(form.Email)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		recordPasswordResetFailure(form.Email)
		sendError(w, 401, genericAuthErr)
		return
	}
	if user.Disabled {
		recordPasswordResetFailure(form.Email)
		sendError(w, 401, genericAuthErr)
		return
	}

	code, err := Decrypt(user.TwoFactorCode, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		recordPasswordResetFailure(form.Email)
		sendError(w, 401, genericAuthErr)
		return
	}

	otp := gotp.NewDefaultTOTP(code).Now()
	if otp != form.ResetCode {
		recordPasswordResetFailure(form.Email)
		sendError(w, 401, genericAuthErr)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(form.Password), 13)
	if err != nil {
		sendError(w, 500, "Unable to generate a secure password, please contact customer support")
		return
	}
	user.Password = string(hash)

	err = resetUserPassword(user)
	if err != nil {
		sendError(w, 401, "Database error, please try again in a moment")
		return
	}

	clearPasswordResetAttempts(form.Email)
	w.WriteHeader(200)
}

func handleClientActivateLicense(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	form := new(licenseActivateRequest)
	err := decodeBody(r, form)
	if err != nil {
		sendError(w, 400, err.Error())
		return
	}

	user := getUserFromContext(r.Context())
	if user == nil {
		sendError(w, 401, "Unauthorized")
		return
	}

	INFO("KEY attempt:", redactKey(form.Key))

	lemonClient := lc.Load()
	key, resp, err := lemonClient.Licenses.Validate(context.Background(), form.Key, "")
	if err != nil {
		if resp != nil && resp.Body != nil {
			sendError(w, 500, "unexpected error, please try again")
			return
		}
		sendError(w, 500, "unexpected error, please try again")
		return
	}

	if key.LicenseKey.ActivationUsage > 0 {
		sendError(w, 400, "key is already in use, please contact customer support")
		return
	}

	if err := applyLemonLicense(user, key.Meta.ProductName, key.LicenseKey.Key, key.LicenseKey.CreatedAt, key.LicenseKey.ExpiresAt); err != nil {
		ADMIN("unable to parse license key name:", err)
		sendError(w, 500, "Something went wrong, please contact customer support")
		return
	}

	activeKey, resp, err := lemonClient.Licenses.Activate(context.Background(), form.Key, "tunnels")
	if err != nil {
		if resp != nil && resp.Body != nil {
			sendError(w, 500, "unexpected error, please try again")
			return
		}
		sendError(w, 500, "unexpected error, please try again")
		return
	}

	if activeKey.Error != "" {
		sendError(w, 400, activeKey.Error)
		return
	}

	user.Trial = false
	user.Disabled = false
	err = activateUserKey(user.SubExpiration, user.Key, user.ID)
	if err != nil {
		sendError(w, 500, "unexpected error, please contact support")
		return
	}

	if key != nil {
		INFO("KEY: Activated:", redactKey(key.LicenseKey.Key))
	}

	w.WriteHeader(200)
}

func applyLemonLicense(user *User, productName, licenseKey string, created time.Time, expiresAt *time.Time) error {
	if strings.Contains(strings.ToLower(productName), "anonymous") {
		base := user.SubExpiration
		if base.Before(time.Now()) {
			base = time.Now()
		}
		jitter, _ := rand.Int(rand.Reader, big.NewInt(60))
		user.SubExpiration = base.AddDate(0, 1, 0).Add(time.Duration(jitter.Int64()+60) * time.Minute)
		INFO("KEY +1:", redactKey(licenseKey), " - check activation in lemon")

		user.Key = &LicenseKey{
			Created: created,
			Months:  1,
			Key:     "unknown",
		}
	} else {
		ns := strings.Split(productName, " ")
		months, err := strconv.Atoi(ns[0])
		if err != nil {
			return err
		}

		base := user.SubExpiration
		if base.Before(time.Now()) {
			base = time.Now()
		}
		jitter2, _ := rand.Int(rand.Reader, big.NewInt(600))
		user.SubExpiration = base.AddDate(0, months, 0).Add(time.Duration(jitter2.Int64()+60) * time.Minute)
		INFO("KEY +", months, ":", redactKey(licenseKey), " - check activate in lemon")

		user.Key = &LicenseKey{
			Created: created,
			Months:  months,
			Key:     licenseKey,
		}
	}
	if expiresAt != nil && !expiresAt.IsZero() {
		user.SubExpiration = expiresAt.UTC()
	}
	return nil
}
