package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xlzd/gotp"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func BasicRecover() {
	if r := recover(); r != nil {
		ERR(r, string(debug.Stack()))
	}
}

func CopySlice(in []byte) (out []byte) {
	out = make([]byte, len(in))
	_ = copy(out, in)
	return
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")

func GENERATE_CODE() string {
	defer BasicRecover()
	b := make([]rune, 16)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	return strings.ToUpper(string(b))
}

func decodeBody(r *http.Request, target any) (err error) {
	// ra, err := io.ReadAll(r.Body)
	// fmt.Println(string(ra))
	dec := json.NewDecoder(r.Body)
	// dec.DisallowUnknownFields()
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

	return
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

			rcs := strings.SplitSeq(string(rc), " ")
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

			otp := gotp.NewDefaultTOTP(string(code)).Now()
			if otp != LF.Digits {
				return errors.New("Authenticator code was incorrect")
			}
		}
	}
	return nil
}

func authenticateUserFromEmailOrIDAndToken(email string, id primitive.ObjectID, token string) (user *User, err error) {
	if email != "" {
		user, err = DB_findUserByEmail(email)
	} else if id != primitive.NilObjectID {
		user, err = DB_findUserByID(id)
	} else {
		return nil, errors.New("user not found")
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
		if d.DT == token {
			allowed = true
		}
	}

	if !allowed {
		if user.APIKey == token {
			allowed = true
		}
	}

	if allowed {
		return
	}

	return nil, errors.New("unauthorized")
}
