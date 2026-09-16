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
		senderr(w, 401, "Invalid login credentials")
		return nil, email
	}

	user, err := findUserByEmail(email)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return nil, email
	}
	if user == nil {
		recordPasswordResetFailure(email)
		senderr(w, 401, "Invalid login credentials")
		return nil, email
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(lf.Password))
	if err != nil {
		recordPasswordResetFailure(email)
		senderr(w, 401, "Invalid login credentials")
		return nil, email
	}

	err = validateUserTwoFactor(user, lf)
	if err != nil {
		recordPasswordResetFailure(email)
		senderr(w, 401, "Invalid login credentials")
		return nil, email
	}

	if user.Disabled {
		senderr(w, 403, "This account has been disabled, please contact customer support")
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

	LF := new(loginRequest)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, email := authenticatePasswordLogin(w, LF)
	if user == nil {
		return
	}

	if !user.IsAdmin {
		senderr(w, 401, "Admin or Manager access required")
		return
	}

	userLoginUpdate := handleUserDeviceToken(user, LF)
	err = updateUserDeviceTokens(userLoginUpdate)
	if err != nil {
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	clearPasswordResetAttempts(email)

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

func handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	user := getUserFromContext(r.Context())
	if user != nil {
		LF := new(logoutRequest)
		_ = decodeBody(r, LF)
		if !LF.All && LF.LogoutToken == "" {
			LF.LogoutToken = getDeviceTokenFromContext(r.Context())
		}

		user.Tokens = revokeUserDeviceTokens(user.Tokens, LF)

		update := new(userTokensUpdate)
		update.ID = user.ID
		update.Tokens = user.Tokens
		if err := updateUserDeviceTokens(update); err != nil {
			senderr(w, 500, "Database error, please try again in a moment")
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

	LF := new(loginRequest)
	err := decodeBody(r, LF)
	if err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	user, email := authenticatePasswordLogin(w, LF)
	if user == nil {
		return
	}

	userLoginUpdate := handleUserDeviceToken(user, LF)
	err = updateUserDeviceTokens(userLoginUpdate)
	if err != nil {
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	clearPasswordResetAttempts(email)

	user.RemoveSensitiveInformation()
	sendObject(w, user)
}

func handleClientLogout(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	LF := new(logoutRequest)
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

	if !LF.All && LF.LogoutToken == "" {
		if LF.DeviceToken != "" {
			LF.LogoutToken = LF.DeviceToken
		} else {
			LF.LogoutToken = getDeviceTokenFromContext(r.Context())
		}
	}

	user.Tokens = revokeUserDeviceTokens(user.Tokens, LF)

	userTokenUpdate := new(userTokensUpdate)
	userTokenUpdate.ID = user.ID
	userTokenUpdate.Tokens = user.Tokens

	err = updateUserDeviceTokens(userTokenUpdate)
	if err != nil {
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	w.WriteHeader(200)
}

func handleClientTwoFactorConfirm(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()

	LF := new(twoFactorRequest)
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

	updatePackage := new(twoFactorUpdate)
	updatePackage.UID = user.ID
	updatePackage.Code, err = Encrypt(LF.Code, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		senderr(w, 500, "Encryption error")
		return
	}

	recoveryByte := strings.Join([]string{generateCode(), generateCode()}, " ")

	updatePackage.Recovery, err = Encrypt(recoveryByte, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		senderr(w, 500, "Encryption error")
		return
	}

	err = updateUserTwoFactorCodes(updatePackage)
	if err != nil {
		senderr(w, 500, "Database error, please try again in a moment")
		return
	}

	out := make(map[string]any)
	out["Message"] = ""
	out["Data"] = recoveryByte

	sendObject(w, out)
}

func handleClientResetPassword(w http.ResponseWriter, r *http.Request) {
	defer randomAuthDelay()
	defer BasicRecover()

	var user *User
	RF := new(passwordResetRequest)
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

	RF.Email = normalizeEmail(RF.Email)
	if RF.Email == "" {
		senderr(w, 401, genericAuthErr)
		return
	}

	if !passwordResetAllowed(RF.Email) {
		senderr(w, 429, rateLimitErr)
		return
	}

	user, err = findUserByEmail(RF.Email)
	if err != nil {
		senderr(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		recordPasswordResetFailure(RF.Email)
		senderr(w, 401, genericAuthErr)
		return
	}
	if user.Disabled {
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

	err = resetUserPassword(user)
	if err != nil {
		senderr(w, 401, "Database error, please try again in a moment")
		return
	}

	clearPasswordResetAttempts(RF.Email)
	w.WriteHeader(200)
}

func handleClientActivateLicense(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	AF := new(licenseActivateRequest)
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
	if key.LicenseKey.ExpiresAt != nil && !key.LicenseKey.ExpiresAt.IsZero() {
		user.SubExpiration = key.LicenseKey.ExpiresAt.UTC()
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
	err = activateUserKey(user.SubExpiration, user.Key, user.ID)
	if err != nil {
		senderr(w, 500, "unexpected error, please contact support")
		return
	}

	if key != nil {
		INFO("KEY: Activated:", redactKey(key.LicenseKey.Key))
	}

	w.WriteHeader(200)
}
