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
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xlzd/gotp"
)

var recoveryConsumeMu sync.Mutex

// deviceTokenMatchesLogout reports whether dt should be revoked for LF.
// Prefer LogoutToken (raw session secret). When Tokens[].DT is redacted in API
// responses, clients revoke other sessions by LogoutName + LogoutCreated.
func deviceTokenMatchesLogout(dt *DeviceToken, lf *logoutRequest) bool {
	if dt == nil || lf == nil {
		return false
	}
	if lf.LogoutToken != "" {
		return subtle.ConstantTimeCompare([]byte(dt.DT), []byte(lf.LogoutToken)) == 1
	}
	if lf.LogoutName == "" || lf.LogoutCreated.IsZero() {
		return false
	}
	if dt.N != lf.LogoutName {
		return false
	}
	// Compare at second resolution so JSON/RFC3339 round-trips match.
	return dt.Created.Unix() == lf.LogoutCreated.Unix()
}

func revokeUserDeviceTokens(tokens []*DeviceToken, lf *logoutRequest) []*DeviceToken {
	if lf == nil {
		return tokens
	}
	if lf.All {
		return make([]*DeviceToken, 0)
	}
	return slices.DeleteFunc(tokens, func(dt *DeviceToken) bool {
		return deviceTokenMatchesLogout(dt, lf)
	})
}

func handleUserDeviceToken(user *User, LF *loginRequest) (userTokenUpdate *userTokensUpdate) {
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

	if len(user.Tokens) > 20 {
		slices.SortFunc(user.Tokens, func(a, b *DeviceToken) int {
			return b.Created.Compare(a.Created)
		})
		user.Tokens = user.Tokens[:20]
	}

	userTokenUpdate = new(userTokensUpdate)
	userTokenUpdate.ID = user.ID
	userTokenUpdate.Tokens = user.Tokens
	userTokenUpdate.Version = LF.Version

	return userTokenUpdate
}

func consumeRecoveryCode(user *User, recovery string) error {
	fresh, ferr := findUserByID(user.ID)
	if ferr != nil || fresh == nil {
		return errors.New("unable to validate recovery code")
	}

	recoveryFound := false
	recoveryUpper := strings.ToUpper(recovery)
	rc, err := Decrypt(fresh.RecoveryCodes, []byte(loadSecret("TwoFactorKey")))
	if err != nil {
		ADMIN(err)
		return errors.New("encryption error")
	}

	remaining := make([]string, 0)
	for _, v := range strings.Fields(rc) {
		if v == recoveryUpper {
			recoveryFound = true
			continue
		}
		remaining = append(remaining, v)
	}

	if !recoveryFound {
		return errors.New("invalid Recovery code")
	}

	newBlob, encErr := Encrypt(strings.Join(remaining, " "), []byte(loadSecret("TwoFactorKey")))
	if encErr != nil {
		ADMIN(encErr)
		return errors.New("encryption error")
	}
	if dbErr := updateUserRecoveryCodes(user.ID, newBlob); dbErr != nil {
		ADMIN(dbErr)
		return errors.New("unable to consume recovery code, please try again")
	}
	return nil
}

func validateUserTwoFactor(user *User, LF *loginRequest) (err error) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()
	recoveryEnabled := false
	if user.TwoFactorEnabled {
		if LF.Recovery != "" {

			recoveryConsumeMu.Lock()
			defer recoveryConsumeMu.Unlock()

			if err := consumeRecoveryCode(user, LF.Recovery); err != nil {
				return err
			}
			recoveryEnabled = true
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
		user, err = findUserByEmail(normalizeEmail(email))
	} else if id != uuid.Nil {
		user, err = findUserByID(id)
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

	if token == "" {
		return nil, errors.New("authentication token missing")
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

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func cookieCipher() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(loadSecret("CookieSigningKey")))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

const adminSessionTTL = 7 * 24 * time.Hour

type adminCookiePayload struct {
	UID string `json:"u"`
	DT  string `json:"t"`
	IP  string `json:"i"`
	Exp int64  `json:"e"`
}

func encryptAdminCookie(userID, deviceToken, ip string) (string, error) {
	gcm, err := cookieCipher()
	if err != nil {
		return "", err
	}

	plain, err := json.Marshal(adminCookiePayload{
		UID: userID,
		DT:  deviceToken,
		IP:  ip,
		Exp: time.Now().Add(adminSessionTTL).Unix(),
	})
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plain, nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

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

	var payload adminCookiePayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return uuid.Nil, "", errors.New("invalid session")
	}
	if payload.UID == "" || payload.DT == "" || payload.Exp == 0 {
		return uuid.Nil, "", errors.New("invalid session")
	}
	if time.Now().Unix() > payload.Exp {
		return uuid.Nil, "", errors.New("invalid session")
	}

	if subtle.ConstantTimeCompare([]byte(remoteIP), []byte(payload.IP)) != 1 {
		return uuid.Nil, "", errors.New("invalid session")
	}

	uid, err = uuid.Parse(payload.UID)
	if err != nil {
		return uuid.Nil, "", errors.New("invalid session")
	}

	return uid, payload.DT, nil
}
