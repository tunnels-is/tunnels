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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xlzd/gotp"
)

var recoveryConsumeMu sync.Mutex

func BasicRecover() {
	if r := recover(); r != nil {
		ERR(r, string(debug.Stack()))
	}
}

func redactKey(k string) string {
	const show = 5
	if len(k) <= show {
		return "…"
	}
	return k[:show] + "…"
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
	r.Body = http.MaxBytesReader(nil, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	err = dec.Decode(target)
	if err != nil {
		return fmt.Errorf("Invalid request body: %s", err)
	}
	return nil
}

// deviceTokenMatchesLogout reports whether dt should be revoked for LF.
// Prefer LogoutToken (raw session secret). When Tokens[].DT is redacted in API
// responses, clients revoke other sessions by LogoutName + LogoutCreated.
func deviceTokenMatchesLogout(dt *DeviceToken, lf *LOGOUT_FORM) bool {
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

func revokeUserDeviceTokens(tokens []*DeviceToken, lf *LOGOUT_FORM) []*DeviceToken {
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

	if len(user.Tokens) > 20 {
		slices.SortFunc(user.Tokens, func(a, b *DeviceToken) int {
			return b.Created.Compare(a.Created)
		})
		user.Tokens = user.Tokens[:20]
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

			recoveryConsumeMu.Lock()
			defer recoveryConsumeMu.Unlock()

			fresh, ferr := DB_findUserByID(user.ID)
			if ferr != nil || fresh == nil {
				return errors.New("unable to validate recovery code")
			}

			recoveryFound := false
			recoveryUpper := strings.ToUpper(LF.Recovery)
			rc, err := Decrypt(fresh.RecoveryCodes, []byte(loadSecret("TwoFactorKey")))
			if err != nil {
				ADMIN(err)
				return errors.New("encryption error")
			}

			remaining := make([]string, 0)
			for _, v := range strings.Fields(rc) {
				if v == recoveryUpper {
					recoveryEnabled = true
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
			if dbErr := DB_updateUserRecoveryCodes(user.ID, newBlob); dbErr != nil {
				ADMIN(dbErr)
				return errors.New("unable to consume recovery code, please try again")
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
		user, err = DB_findUserByEmail(normalizeEmail(email))
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

func hasSharedOrNoGroup(actorGroups []uuid.UUID, serverGroups []uuid.UUID) (yes bool) {
	if len(serverGroups) == 0 {
		return true
	}
	for _, g := range actorGroups {
		for _, dg := range serverGroups {
			if subtle.ConstantTimeCompare(g[:], dg[:]) == 1 {
				return true
			}
		}
	}

	return false
}
