package main

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/tunnels-is/monorepo/helpers"
	"github.com/tunnels-is/monorepo/structs"
)

func WriteErrorResponse(
	e echo.Context,
	code int,
	error string,
) error {
	return e.JSON(code, structs.ErrorResponse{Error: error})
}

func ValidateSubscription(c echo.Context, CR *structs.ConnectRequest) (user *User, code int, err error) {
	user, err = DB_findUserByID(CR.UserID)
	if err != nil {
		return nil, 500, errors.New("Unknown error, please try again in a moment")
	}

	if user == nil {
		return nil, 401, errors.New("User not found")
	}

	if user.Disabled {
		return nil, 401, errors.New("User disabled")
	}

	if !ValidToken(user, CR.UserToken) {
		return nil, 401, errors.New("Invalid device token")
	}

	if user.Trial {
		if time.Now().After(user.SubExpiration) {
			return nil, 401, errors.New("trial has ended")
		}
	} else {
		if time.Since(user.SubExpiration).Hours() > (24 * 4) {
			return nil, 401, errors.New("subscription has expired")
		}
	}

	return
}

func V3_userEnable(c echo.Context) error {
	defer helpers.BasicRecover()

	UF := new(USER_ENABLE_FORM)
	if err := c.Bind(UF); err != nil {
		return c.JSON(400, "The input is invalid")
	}
	if UF.Email == "" {
		return c.JSON(400, "The input is invalid")
	}

	var err error
	user, err := DB_findUserByEmail(UF.Email)
	if user == nil {
		return c.JSON(404, "User not found")
	}

	if err != nil {
		return c.JSON(500, "Database error, please try again in a moment")
	}

	if UF.Code != user.ConfirmCode {
		return c.JSON(401, "Invalid confirm code")
	}

	err = DB_WipeUserConfirmCode(&USER_ENABLE_QUERY{
		Email: user.Email,
	})
	if err != nil {
		return c.JSON(500, "Unable to update user, please try again in a moment")
	}

	return c.JSON(200, nil)
}

func KeyActivate(c echo.Context) error {
	defer helpers.BasicRecover()

	AF := new(KEY_ACTIVATE_FORM)
	if err := c.Bind(AF); err != nil {
		return c.JSON(400, "Invalid Inputs")
	}

	user, err := DB_findUserByEmail(AF.Email)
	if err != nil {
		return c.JSON(500, "Unexpected error, please try again in a moment")
	}
	if user == nil {
		return c.JSON(400, "user not found")
	}

	INFO(3, "KEY attempt:", AF.Key)

	key, resp, err := lemonClient.Licenses.Validate(context.Background(), AF.Key, "")
	if err != nil {
		if resp != nil && resp.Body != nil {
			ADMIN(3, "KEY: unable to validate", AF.Key, err)
			return c.String(resp.HTTPResponse.StatusCode, string(*resp.Body))
		} else {
			ADMIN(3, "KEY: unable to validate:", AF.Key, err)
			return c.JSON(500, "unexpected error, please try again")
		}
	}

	if key.LicenseKey.ActivationUsage > 0 {
		INFO(3, "KEY: already active:", AF.Key)
		return c.JSON(400, "key is already in use, please contact customer support")
	}

	randomizer := rand.New(helpers.RAND_SOURCE)
	if strings.Contains(strings.ToLower(key.LicenseAttributes.Meta.ProductName), "anonymous") {
		if user.SubExpiration.IsZero() {
			user.SubExpiration = time.Now()
		}
		if time.Until(user.SubExpiration).Seconds() > 1 {
			user.SubExpiration = time.Now()
		}
		user.SubExpiration = user.SubExpiration.AddDate(0, 1, 0).Add(time.Duration(randomizer.Intn(60)+60) * time.Minute)
		INFO(3, "KEY +1:", key.LicenseKey.Key, " - check activation in lemon")

		user.Key = &LicenseKey{
			Created: key.LicenseKey.CreatedAt,
			Months:  1,
			Key:     "unknown",
		}
	} else {
		ns := strings.Split(key.LicenseAttributes.Meta.ProductName, " ")
		months, err := strconv.Atoi(ns[0])
		if err != nil {
			ADMIN(3, "unable to parse license key name:", err)
			return c.JSON(500, "Something went wrong, please contact customer support")
		}
		if user.SubExpiration.IsZero() {
			user.SubExpiration = time.Now()
		}
		user.SubExpiration = time.Now().AddDate(0, months, 0).Add(time.Duration(randomizer.Intn(600)+60) * time.Minute)
		INFO(3, "KEY +", months, ":", key.LicenseKey.Key, " - check activate in lemon")

		user.Key = &LicenseKey{
			Created: key.LicenseKey.CreatedAt,
			Months:  months,
			Key:     key.LicenseKey.Key,
		}
	}

	user.Trial = false
	user.Disabled = false
	err = DB_UserActivateKey(user.SubExpiration, user.Key, user.ID)
	if err != nil {
		INFO(3, "KEY: unable to update user:", AF.Key, " exp:", user.SubExpiration, "err: ", err)
		return c.JSON(500, "unexpected error, please contact support")
	}

	activeKey, resp, err := lemonClient.Licenses.Activate(context.Background(), AF.Key, "tunnels")
	if err != nil {
		if resp != nil && resp.Body != nil {
			INFO(3, "KEY: activated:", key.LicenseKey.Key)
			return c.String(resp.HTTPResponse.StatusCode, string(*resp.Body))
		} else {
			ADMIN(3, "KEY: unable to verify:", AF.Key, err, resp)
			return c.JSON(500, "unexpected error, please try again")
		}
	}

	if activeKey.Error != "" {
		ADMIN(3, "KEY: activation error:", err)
		return c.JSON(400, "There was an error during license activation, please contact customer support or try again, error: "+activeKey.Error)
	}

	if key != nil {
		INFO(3, "KEY: Activated:", key.LicenseKey.Key)
	}
	return c.JSON(200, nil)
}
