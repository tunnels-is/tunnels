package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xlzd/gotp"
)

func BasicRecover() {
	if r := recover(); r != nil {
		ERR(r, string(debug.Stack()))
	}
}

func CopySlice(in []byte) (out []byte) {
	out = make([]byte, len(in))
	_ = copy(out, in)
	return out
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")

func GENERATE_CODE() string {
	defer BasicRecover()
	b := make([]rune, 16)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letterRunes))))
		if err != nil {
			panic(err)
		}
		b[i] = letterRunes[n.Int64()]
	}

	return strings.ToUpper(string(b))
}

func decodeBody(r *http.Request, target any) (err error) {
	dec := json.NewDecoder(r.Body)
	err = dec.Decode(target)
	if err != nil {
		return fmt.Errorf("Invalid request body: %s", err)
	}
	return nil
}

func sendObject(w http.ResponseWriter, obj any) {
	w.WriteHeader(200)
	var err error
	enc := json.NewEncoder(w)
	u, ok := obj.(*User)
	if ok {
		u.RemoveSensitiveInformation()
		err = enc.Encode(u)
	} else {
		err = enc.Encode(obj)
	}
	if err != nil {
		senderr(w, 500, "unable to encode response object")
		return
	}
}

func handleUserDeviceToken(user *User, LF *LOGIN_FORM) (userTokenUpdate *UPDATE_USER_TOKENS) {
	defer BasicRecover()

	tokenExists := false
	if LF.DeviceToken != "" {
		for i, v := range user.Tokens {
			if v.DT == LF.DeviceToken {
				tokenExists = true
				user.Tokens[i].DT = uuid.NewString()
				user.Tokens[i].N = LF.DeviceName
				user.Tokens[i].Created = time.Now()
				user.DeviceToken = user.Tokens[i]
			}
		}
	}

	if !tokenExists {
		T := new(DeviceToken)
		T.N = LF.DeviceName
		T.DT = uuid.NewString()
		T.Created = time.Now()

		user.DeviceToken = T
		user.Tokens = append(user.Tokens, T)
	}

	userTokenUpdate = new(UPDATE_USER_TOKENS)
	userTokenUpdate.ID = user.ID
	userTokenUpdate.Tokens = user.Tokens
	userTokenUpdate.Version = LF.Version

	return userTokenUpdate
}

func validateUserTwoFactor(user *User, LF *LOGIN_FORM) (err error) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()
	recoveryEnabled := false
	if user.TwoFactorEnabled {
		if LF.Recovery != "" {
			recoveryFound := false
			recoveryUpper := strings.ToUpper(LF.Recovery)
			rc, err := Decrypt(user.RecoveryCodes, []byte(loadSecret("TwoFactorKey")))
			if err != nil {
				ADMIN(err)
				return errors.New("encryption error")
			}

			rcs := strings.SplitSeq(rc, " ")
			for v := range rcs {
				if v == recoveryUpper {
					recoveryEnabled = true
					recoveryFound = true
				}
			}

			if !recoveryFound {
				return errors.New("invalid Recovery code")
			}
		}

		if !recoveryEnabled {
			code, err := Decrypt(user.TwoFactorCode, []byte(loadSecret("TwoFactorKey")))
			if err != nil {
				ADMIN(err)
				return errors.New("encryption error")
			}

			otp := gotp.NewDefaultTOTP(code).Now()
			if otp != LF.Digits {
				return errors.New("Authenticator code was incorrect")
			}
		}
	}
	return nil
}

func authenticateUserFromEmailOrIDAndToken(email string, id uuid.UUID, token string) (user *User, err error) {
	if email != "" {
		user, err = DB_findUserByEmail(email)
	} else if id != uuid.Nil {
		user, err = DB_findUserByID(id)
	} else {
		return nil, errors.New("user identifier missing")
	}
	if err != nil {
		return nil, errors.New("Database error, please try again in a moment")
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	if user.Disabled {
		return nil, errors.New("This account has been disabled, please contact customer support")
	}

	allowed := false
	for _, d := range user.Tokens {
		if subtle.ConstantTimeCompare([]byte(d.DT), []byte(token)) == 1 {
			allowed = true
		}
	}

	if !allowed {
		if subtle.ConstantTimeCompare([]byte(user.APIKey), []byte(token)) == 1 {
			allowed = true
		}
	}

	if allowed {
		return user, err
	}

	return nil, errors.New("unauthorized")
}

// clientIP extracts the remote IP address from the request, stripping the port.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// cookieCipher returns an AES-256-GCM cipher derived from CookieSigningKey.
func cookieCipher() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(loadSecret("CookieSigningKey")))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// encryptAdminCookie encrypts userID, deviceToken and the client IP into an
// opaque base64 cookie value. AES-256-GCM provides both confidentiality and
// authenticity, so a separate HMAC is not needed.
func encryptAdminCookie(userID, deviceToken, ip string) (string, error) {
	gcm, err := cookieCipher()
	if err != nil {
		return "", err
	}

	plaintext := []byte(userID + ":" + deviceToken + ":" + ip)

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// decryptAdminCookie decrypts and authenticates the cookie, then verifies
// that the embedded IP matches the current request's remote IP.
func decryptAdminCookie(cookieValue, remoteIP string) (uid uuid.UUID, deviceToken string, err error) {
	gcm, err := cookieCipher()
	if err != nil {
		return uuid.Nil, "", errors.New("internal encryption error")
	}

	data, err := base64.RawURLEncoding.DecodeString(cookieValue)
	if err != nil {
		return uuid.Nil, "", errors.New("invalid session")
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return uuid.Nil, "", errors.New("invalid session")
	}

	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return uuid.Nil, "", errors.New("invalid session")
	}

	parts := strings.SplitN(string(plaintext), ":", 3)
	if len(parts) != 3 {
		return uuid.Nil, "", errors.New("invalid session format")
	}

	userIDStr, deviceToken, cookieIP := parts[0], parts[1], parts[2]

	if subtle.ConstantTimeCompare([]byte(remoteIP), []byte(cookieIP)) != 1 {
		return uuid.Nil, "", errors.New("session IP mismatch")
	}

	uid, err = uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, "", errors.New("invalid session")
	}

	return uid, deviceToken, nil
}
