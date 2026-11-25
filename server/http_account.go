package main

import (
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"

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

	ip := clientIP(r)
	if !takeAuthSlot(authLimitReset, form.Email, ip) {
		sendError(w, 429, rateLimitErr)
		return
	}

	user, err = findUserByEmail(form.Email)
	if err != nil {
		sendError(w, 500, "Unknown error, please try again in a moment")
		return
	}
	if user == nil {
		sendError(w, 401, genericAuthErr)
		return
	}
	if user.Disabled {
		sendError(w, 401, genericAuthErr)
		return
	}

	code, err := Decrypt(user.TwoFactorCode, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		sendError(w, 401, genericAuthErr)
		return
	}

	otp := gotp.NewDefaultTOTP(code).Now()
	if subtle.ConstantTimeCompare([]byte(otp), []byte(form.ResetCode)) != 1 {
		sendError(w, 401, genericAuthErr)
		return
	}

	if err := previewResetAuth(user.ID, otp, form.Recovery); err != nil {
		code, msg := resetAuthStatus(err)
		sendError(w, code, msg)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(form.Password), 13)
	if err != nil {
		sendError(w, 500, "Unable to generate a secure password, please contact customer support")
		return
	}

	recoveryConsumeMu.Lock()
	recErr := acceptResetAuth(user.ID, otp, form.Recovery, string(hash))
	recoveryConsumeMu.Unlock()
	if recErr != nil {
		code, msg := resetAuthStatus(recErr)
		sendError(w, code, msg)
		return
	}

	clearAuthSlot(authLimitReset, form.Email, ip)
	w.WriteHeader(200)
}

func resetAuthStatus(err error) (int, string) {
	if errors.Is(err, errTOTPReused) || errors.Is(err, errRecoveryNotFound) || errors.Is(err, errRecoveryDecrypt) {
		return 401, "invalid email or reset code"
	}
	return 401, "Database error, please try again in a moment"
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
