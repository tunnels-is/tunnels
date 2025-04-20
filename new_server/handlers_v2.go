package main

import (
	"errors"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/tunnels-is/monorepo/email"
	"github.com/tunnels-is/monorepo/encrypter"
	"github.com/tunnels-is/monorepo/helpers"
	"github.com/xlzd/gotp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

func handleUserDeviceToken(user *User, LF *LOGIN_FORM) (userTokenUpdate *UPDATE_USER_TOKENS) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

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
			var recoveryFound bool = false
			recoveryUpper := strings.ToUpper(LF.Recovery)
			rc, err := encrypter.Decrypt(user.RecoveryCodes, []byte(ENV.F2KEY))
			if err != nil {
				ADMIN(err)
				return errors.New("Encryption error")
			}

			rcs := strings.Split(string(rc), " ")
			for _, v := range rcs {
				if v == recoveryUpper {
					recoveryEnabled = true
					recoveryFound = true
				}
			}

			if !recoveryFound {
				return errors.New("Invalid Recovery code")
			}
		}

		if !recoveryEnabled {
			code, err := encrypter.Decrypt(user.TwoFactorCode, []byte(ENV.F2KEY))
			if err != nil {
				ADMIN(err)
				return errors.New("Encryption error")
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

	if user == nil {
		return nil, errors.New("user not found")
	}

	if err != nil {
		return nil, errors.New("Database error, please try again in a moment")
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

	if !allowed {
		return nil, errors.New("unauthorized")
	}

	return
}

func V3_userUpdateSubStatus(c echo.Context) error {
	defer helpers.BasicRecover()

	UF := new(USER_UPDATE_SUB_FORM)
	if err := c.Bind(UF); err != nil {
		return c.JSON(400, "The input is invalid, please verify your input information")
	}
	if UF.Email == "" {
		return c.JSON(400, "The input is invalid, please verify your input information")
	}

	user, err := authenticateUserFromEmailOrIDAndToken(UF.Email, primitive.NilObjectID, UF.DeviceToken)
	if err != nil || user == nil {
		return c.JSON(401, err.Error())
	}

	err = DB_toggleUserSubscriptionStatus(UF)
	if err != nil {
		return c.JSON(401, "Unable to update users, please try again in a moment")
	}

	return c.JSON(200, nil)
}

func V2_userResetPassword(c echo.Context) error {
	defer helpers.BasicRecover()
	start := time.Now()

	var user *User
	var err error
	RF := new(PASSWORD_RESET_FORM)
	if err := c.Bind(RF); err != nil {
		ADMIN(3, err)
		return c.JSON(400, "The input is invalid, please verify your password reset information")
	}

	if RF.NewPassword == "" {
		return c.JSON(400, 200)
	}

	if len(RF.NewPassword) > 200 {
		return c.JSON(400, 201)
	}

	if len(RF.NewPassword) < 10 {
		return c.JSON(400, 201)
	}

	user, err = DB_findUserByEmail(RF.Email)
	if user == nil {
		return c.JSON(401, "Invalid user, please try again")
	}

	if err != nil {
		return c.JSON(500, "Unknown error, please try again in a moment")
	}

	if user.Email == "" {
		return c.JSON(401, "Could not find the user")
	}

	if RF.ResetCode != user.ResetCode || user.ResetCode == "" {
		return c.JSON(401, "Invalid reset code")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(RF.NewPassword), 13)
	if err != nil {
		return c.JSON(500, "Unable to generate a secure password, please contact customer support")
	}
	user.Password = string(hash)

	err = DB_userResetPassword(user)
	if err != nil {
		return c.JSON(401, "Database error, please try again in a moment")
	}

	INFO(3, "PASSWORD RESET @ MS:", time.Since(start).Milliseconds())
	return c.JSON(200, nil)
}

func V2_RequestPasswordReset(c echo.Context) error {
	defer helpers.BasicRecover()
	start := time.Now()

	// user := new(structs.User)
	var user *User
	var err error
	RF := new(PASSWORD_RESET_FORM)
	if err := c.Bind(RF); err != nil {
		ADMIN(3, err)
		return c.JSON(400, "The input is invalid, please verify your password reset information")
	}

	user, err = DB_findUserByEmail(RF.Email)
	if user == nil {
		return c.JSON(401, "Invalid session token, please log in again")
	}
	if err != nil {
		return c.JSON(500, "Unknown error, please try again in a moment")
	}

	if user.Email == "" {
		return c.JSON(401, "Could not find the user")
	}

	if !user.LastResetRequest.IsZero() && time.Since(user.LastResetRequest).Seconds() < 30 {
		return c.JSON(401, "You need to wait at least 30 seconds between password reset attempts")
	}

	user.ResetCode = uuid.NewString()
	user.LastResetRequest = time.Now()

	err = DB_userUpdateResetCode(user)
	if err != nil {
		return c.JSON(500, "Database error, please try again in a moment")
	}

	err = email.SEND_PASSWORD_RESET(ENV.EMAILKEY, user.Email, user.ResetCode)
	if err != nil {
		ADMIN(3, "UNABLE TO SEND PASSWORD RESET CODE TO USER: ", user.ID)
		return c.JSON(500, "Email system  error, please try again in a moment")
	}

	// user.MutateForHTTPResponse()
	INFO(3, "PASSWORD RESET CODE @ MS:", time.Since(start).Milliseconds())
	return c.JSON(200, nil)
}
