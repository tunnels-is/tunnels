package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

func makeErr(code int, msg string, slogArgs ...any) *ErrorResponse {
	logger.Error(msg, slogArgs...)
	return &ErrorResponse{Code: code, Error: msg}
}

func sendHTTPErrorResponse(w http.ResponseWriter, errResp *ErrorResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(errResp.Code)
	err := json.NewEncoder(w).Encode(errResp)
	if err != nil {
		logger.Error("unable to write JSON errResponse:", slog.Any("err", err))
	}
}

func sendHTTPOKResponse(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Error("unable to write JSON okResponse:", slog.Any("err", err))
	}
}

func prepData(obj any) (data []byte) {
	var err error
	u, ok := obj.(*User)
	if ok {
		u.RemoveSensitiveInformation()
		data, err = json.Marshal(u)
	} else {
		data, err = json.Marshal(obj)
	}
	if err != nil {
		logger.Error("unable to encode response object", slog.Any("err", err))
		return nil
	}
	return
}

func APIv2_UserLogin(LF *LOGIN_FORM) (errData *ErrorResponse, okData interface{}) {
	defer BasicRecover()

	user, err := DB_findUserByEmail(LF.Email)
	if err != nil {
		return makeErr(500, "Unknown error, please try again in a moment"), nil
	}
	if user == nil {
		return makeErr(401, "Invalid login credentials"), nil
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(LF.Password))
	if err != nil {
		return makeErr(401, "Invalid login credentials"), nil
	}

	err = validateUserTwoFactor(user, LF)
	if err != nil {
		return makeErr(401, err.Error()), nil
	}

	userLoginUpdate := handleUserDeviceToken(user, LF)
	err = DB_updateUserDeviceTokens(userLoginUpdate)
	if err != nil {
		return makeErr(500, "Database error, please try again in a moment"), nil
	}

	return nil, prepData(user)
}
