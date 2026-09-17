package main

import (
	"context"
	"crypto/rand"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xlzd/gotp"
	"golang.org/x/crypto/bcrypt"
)

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

	if msg := passwordLengthError(form.Password); msg != "" {
		sendError(w, 400, msg)
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

func passwordLengthError(pw string) string {
	if len(pw) < 10 {
		return "password smaller then 10 characters"
	}
	if len(pw) > 72 {
		return "Password is too long, maximum 72 characters"
	}
	return ""
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
