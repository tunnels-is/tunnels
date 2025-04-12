package main

import (
	"errors"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestGenerateAndSaveToken(t *testing.T) {
	setupTestDBGlobals(t)

	userUUID := uuid.NewString()
	deviceName := "BrowserXYZ"

	user := &User{UUID: userUUID, Username: "tokenTestUser"}
	err := saveUser(user)
	if err != nil {
		t.Fatalf("Failed to save prerequisite user: %v", err)
	}

	authToken, err := generateAndSaveToken(userUUID, deviceName)
	if err != nil {
		t.Fatalf("generateAndSaveToken failed: %v", err)
	}

	if authToken == nil {
		t.Fatalf("generateAndSaveToken returned nil token")
	}

	if authToken.UserUUID != userUUID {
		t.Errorf("Token UserUUID mismatch: expected %s, got %s", userUUID, authToken.UserUUID)
	}
	if authToken.DeviceName != deviceName {
		t.Errorf("Token DeviceName mismatch: expected %s, got %s", deviceName, authToken.DeviceName)
	}
	if authToken.TokenUUID == "" {
		t.Error("TokenUUID is empty")
	}
	if time.Since(authToken.CreatedAt) > time.Second*5 {
		t.Errorf("Token creation time is too old: %v", authToken.CreatedAt)
	}

	retrievedToken, err := getToken(authToken.TokenUUID)
	if err != nil {
		t.Fatalf("Failed to retrieve the generated token from DB: %v", err)
	}
	if retrievedToken.UserUUID != userUUID {
		t.Errorf("Retrieved token has wrong UserUUID in DB: expected %s, got %s", userUUID, retrievedToken.UserUUID)
	}
}

func TestAuthenticateRequestLogic(t *testing.T) {
	setupTestDBGlobals(t)

	userUUID := uuid.NewString()
	tokenUUID := uuid.NewString()
	user := &User{UUID: userUUID, Username: "authLogicUser"}
	token := &AuthToken{UserUUID: userUUID, TokenUUID: tokenUUID, CreatedAt: time.Now(), DeviceName: "TestAuth"}

	err := saveUser(user)
	if err != nil {
		t.Fatalf("Failed saving user: %v", err)
	}
	err = saveToken(token)
	if err != nil {
		t.Fatalf("Failed saving token: %v", err)
	}

	retrievedToken, err := getToken(tokenUUID)
	if err != nil {
		t.Fatalf("getToken failed for valid token: %v", err)
	}
	retrievedUser, err := getUser(retrievedToken.UserUUID)
	if err != nil {
		t.Fatalf("getUser failed for valid user: %v", err)
	}

	if retrievedUser.UUID != userUUID {
		t.Errorf("Valid scenario: Retrieved wrong user UUID %s", retrievedUser.UUID)
	}
	if retrievedToken.TokenUUID != tokenUUID {
		t.Errorf("Valid scenario: Retrieved wrong token UUID %s", retrievedToken.TokenUUID)
	}

	newUserUUID := uuid.NewString()
	newTokenUUID := uuid.NewString()
	newUser := &User{UUID: newUserUUID, Username: "tempUser"}
	newToken := &AuthToken{UserUUID: newUserUUID, TokenUUID: newTokenUUID, CreatedAt: time.Now()}
	err = saveUser(newUser)
	if err != nil {
		t.Fatalf("Failed saving temp user: %v", err)
	}
	err = saveToken(newToken)
	if err != nil {
		t.Fatalf("Failed saving temp token: %v", err)
	}

	err = deleteUser(newUserUUID)
	if err != nil {
		t.Fatalf("Failed deleting temp user: %v", err)
	}

	retrievedTokenOrphan, errToken := getToken(newTokenUUID)
	if errToken != nil {
		t.Fatalf("getToken should succeed for orphaned token: %v", errToken)
	}
	_, errUser := getUser(retrievedTokenOrphan.UserUUID)
	if errUser == nil {
		t.Errorf("Orphaned token scenario: getUser should have failed, but got nil error")
	} else if !errors.Is(errUser, ErrNotFound) && !errors.Is(errUser, badger.ErrKeyNotFound) {
		t.Errorf("Orphaned token scenario: getUser failed with unexpected error: %v", errUser)
	}

	_, err = getToken("invalid-token-uuid")
	if err == nil {
		t.Errorf("Invalid token scenario: getToken should have failed, but got nil error")
	} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("Invalid token scenario: getToken failed with unexpected error: %v", err)
	}
}

func Test2FASetupLogic(t *testing.T) {
	setupTestDBGlobals(t)

	userUUID := uuid.NewString()
	username := "2faSetupUser"
	user := &User{UUID: userUUID, Username: username, OTPEnabled: false}
	err := saveUser(user)
	if err != nil {
		t.Fatalf("Failed to save prerequisite user: %v", err)
	}

	issuer := "YourAppName"
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: username,
		SecretSize:  16,
	})
	if err != nil {
		t.Fatalf("totp.Generate failed: %v", err)
	}

	user.OTPSecret = key.Secret()
	user.OTPEnabled = false

	err = saveUser(user)
	if err != nil {
		t.Fatalf("Failed to save user with OTP secret: %v", err)
	}

	retrievedUser, err := getUser(userUUID)
	if err != nil {
		t.Fatalf("Failed to retrieve user after setting secret: %v", err)
	}

	if retrievedUser.OTPSecret == "" {
		t.Errorf("User OTPSecret was not saved in DB")
	}
	if retrievedUser.OTPSecret != key.Secret() {
		t.Errorf("Saved OTPSecret %s doesn't match generated %s", retrievedUser.OTPSecret, key.Secret())
	}
	if retrievedUser.OTPEnabled != false {
		t.Errorf("User OTPEnabled should be false after setup, but was true")
	}

	if key.URL() == "" {
		t.Error("Generated key URL is empty")
	}
}

func Test2FAVerifyEnableLogic(t *testing.T) {
	setupTestDBGlobals(t)

	userUUID := uuid.NewString()
	username := "2faVerifyUser"

	key, err := totp.Generate(totp.GenerateOpts{Issuer: "TestApp", AccountName: username, SecretSize: 16})
	if err != nil {
		t.Fatalf("Failed to generate OTP key: %v", err)
	}

	user := &User{UUID: userUUID, Username: username, OTPSecret: key.Secret(), OTPEnabled: false}
	err = saveUser(user)
	if err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	validCode, err := totp.GenerateCodeCustom(user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("Failed to generate valid OTP code for test: %v", err)
	}

	valid, err := totp.ValidateCustom(validCode, user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("Error during valid OTP validation check: %v", err)
	}
	if !valid {
		t.Errorf("Valid OTP code %s failed validation", validCode)
	}

	if valid {
		user.OTPEnabled = true
		err = saveUser(user)
		if err != nil {
			t.Fatalf("Failed to save user after enabling OTP: %v", err)
		}
	}

	retrievedUser, err := getUser(userUUID)
	if err != nil {
		t.Fatalf("Failed retrieving user post-enablement: %v", err)
	}
	if !retrievedUser.OTPEnabled {
		t.Errorf("OTPEnabled should be true in DB after valid verification, but is false")
	}

	invalidCode := "000000"
	if invalidCode == validCode {
		invalidCode = "111111"
	}

	user.OTPEnabled = false
	err = saveUser(user)
	if err != nil {
		t.Fatalf("Failed resetting user state: %v", err)
	}

	valid, err = totp.ValidateCustom(invalidCode, user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Logf("Validation check returned error (potentially expected for invalid code format): %v", err)
	}
	if valid {
		t.Errorf("Invalid OTP code %s passed validation", invalidCode)
	}

	retrievedUser, err = getUser(userUUID)
	if err != nil {
		t.Fatalf("Failed retrieving user after invalid verification: %v", err)
	}
	if retrievedUser.OTPEnabled {
		t.Errorf("OTPEnabled should be false in DB after invalid verification, but is true")
	}
}

func Test2FAVerifyLoginLogic(t *testing.T) {
	setupTestDBGlobals(t)

	userUUID := uuid.NewString()
	username := "2faLoginUser"

	key, err := totp.Generate(totp.GenerateOpts{Issuer: "TestApp", AccountName: username, SecretSize: 16})
	if err != nil {
		t.Fatalf("Failed to generate OTP key: %v", err)
	}

	user := &User{UUID: userUUID, Username: username, OTPSecret: key.Secret(), OTPEnabled: true}
	err = saveUser(user)
	if err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	validCode, err := totp.GenerateCodeCustom(user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("Failed to generate valid OTP code for test: %v", err)
	}

	loginUser, err := getUser(userUUID)
	if err != nil {
		t.Fatalf("Login scenario: Failed getting user %s: %v", userUUID, err)
	}
	if !loginUser.OTPEnabled {
		t.Fatalf("Login scenario: User %s OTPEnabled flag is false", userUUID)
	}
	if loginUser.OTPSecret == "" {
		t.Fatalf("Login scenario: User %s OTPSecret is empty", userUUID)
	}

	valid, err := totp.ValidateCustom(validCode, loginUser.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("Login: Error during valid OTP validation check: %v", err)
	}
	if !valid {
		t.Errorf("Login: Valid OTP code %s failed validation", validCode)
	}

	if valid {
		t.Logf("Login scenario validation passed for valid code %s, token generation would proceed.", validCode)
	}

	invalidCode := "999999"
	if invalidCode == validCode {
		invalidCode = "888888"
	}

	valid, err = totp.ValidateCustom(invalidCode, loginUser.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Logf("Login: Validation check returned error (potentially expected for invalid code): %v", err)
	}
	if valid {
		t.Errorf("Login: Invalid OTP code %s passed validation", invalidCode)
	}

	if !valid {
		t.Logf("Login scenario validation failed for invalid code %s as expected.", invalidCode)
	}
}
