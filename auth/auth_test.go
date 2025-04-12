package main

import (
	"errors"
	"testing"
	"time"

	// No Fiber context mocking needed if we test core logic directly
	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"github.com/pquerna/otp" // Required for key generation and validation
	"github.com/pquerna/otp/totp"
)

// --- Auth Token Tests ---

func TestGenerateAndSaveToken(t *testing.T) {
	setupTestDBGlobals(t) // Need DBs for token storage

	userUUID := uuid.NewString()
	deviceName := "BrowserXYZ"

	// Pre-create user to satisfy potential checks (if any)
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

	// Verify it was actually saved
	retrievedToken, err := getToken(authToken.TokenUUID)
	if err != nil {
		t.Fatalf("Failed to retrieve the generated token from DB: %v", err)
	}
	if retrievedToken.UserUUID != userUUID {
		t.Errorf("Retrieved token has wrong UserUUID in DB: expected %s, got %s", userUUID, retrievedToken.UserUUID)
	}
}

// Test the core logic authenticateRequest relies on
func TestAuthenticateRequestLogic(t *testing.T) {
	setupTestDBGlobals(t)

	userUUID := uuid.NewString()
	tokenUUID := uuid.NewString()
	user := &User{UUID: userUUID, Username: "authLogicUser"}
	token := &AuthToken{UserUUID: userUUID, TokenUUID: tokenUUID, CreatedAt: time.Now(), DeviceName: "TestAuth"}

	// Scenario 1: Valid token and user
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

	// Scenario 2: Token exists, but user deleted
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
	_, errUser := getUser(retrievedTokenOrphan.UserUUID) // Expect failure here
	if errUser == nil {
		t.Errorf("Orphaned token scenario: getUser should have failed, but got nil error")
	} else if !errors.Is(errUser, ErrNotFound) && !errors.Is(errUser, badger.ErrKeyNotFound) {
		t.Errorf("Orphaned token scenario: getUser failed with unexpected error: %v", errUser)
	}

	// Scenario 3: Invalid Token UUID
	_, err = getToken("invalid-token-uuid")
	if err == nil {
		t.Errorf("Invalid token scenario: getToken should have failed, but got nil error")
	} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("Invalid token scenario: getToken failed with unexpected error: %v", err)
	}

}

// --- 2FA Tests ---

func Test2FASetupLogic(t *testing.T) {
	setupTestDBGlobals(t)

	userUUID := uuid.NewString()
	username := "2faSetupUser"
	user := &User{UUID: userUUID, Username: username, OTPEnabled: false}
	err := saveUser(user)
	if err != nil {
		t.Fatalf("Failed to save prerequisite user: %v", err)
	}

	// Mimic core generation logic from handle2FASetup
	issuer := "YourAppName"
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: username,
		SecretSize:  16,
	})
	if err != nil {
		t.Fatalf("totp.Generate failed: %v", err)
	}

	// Save secret to the user in DB (as the handler would)
	user.OTPSecret = key.Secret()
	user.OTPEnabled = false // Ensure it's false before verification

	err = saveUser(user) // Use saveUser to update
	if err != nil {
		t.Fatalf("Failed to save user with OTP secret: %v", err)
	}

	// Verify DB state
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

	// Check if the key URL is generated (part of the response structure)
	if key.URL() == "" {
		t.Error("Generated key URL is empty")
	}
}

func Test2FAVerifyEnableLogic(t *testing.T) {
	setupTestDBGlobals(t)

	userUUID := uuid.NewString()
	username := "2faVerifyUser"
	// Start with generated secret but OTPEnabled = false
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "TestApp", AccountName: username, SecretSize: 16})
	if err != nil {
		t.Fatalf("Failed to generate OTP key: %v", err)
	}

	user := &User{UUID: userUUID, Username: username, OTPSecret: key.Secret(), OTPEnabled: false}
	err = saveUser(user)
	if err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	// Scenario 1: Valid OTP Code
	validCode, err := totp.GenerateCodeCustom(user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("Failed to generate valid OTP code for test: %v", err)
	}

	// Mimic validation check from handle2FAVerify (part 1: Enabling)
	valid, err := totp.ValidateCustom(validCode, user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("Error during valid OTP validation check: %v", err)
	}
	if !valid {
		t.Errorf("Valid OTP code %s failed validation", validCode)
	}

	// If valid, mimic the enabling step
	if valid {
		user.OTPEnabled = true
		err = saveUser(user) // Update user in DB
		if err != nil {
			t.Fatalf("Failed to save user after enabling OTP: %v", err)
		}
	}

	// Verify DB state
	retrievedUser, err := getUser(userUUID)
	if err != nil {
		t.Fatalf("Failed retrieving user post-enablement: %v", err)
	}
	if !retrievedUser.OTPEnabled {
		t.Errorf("OTPEnabled should be true in DB after valid verification, but is false")
	}

	// Scenario 2: Invalid OTP Code
	invalidCode := "000000" // Highly unlikely to be valid
	if invalidCode == validCode {
		invalidCode = "111111"
	} // Ensure difference

	user.OTPEnabled = false // Reset state for this scenario
	err = saveUser(user)
	if err != nil {
		t.Fatalf("Failed resetting user state: %v", err)
	}

	valid, err = totp.ValidateCustom(invalidCode, user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		// Some validation errors are expected for formatting etc, but check unexpected ones
		// Let's assume invalid format/length could error, that's ok test implicitly
		t.Logf("Validation check returned error (potentially expected for invalid code format): %v", err)
	}
	if valid {
		t.Errorf("Invalid OTP code %s passed validation", invalidCode)
	}

	// Verify OTPEnabled remains false
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
	// Start with generated secret AND OTPEnabled = true
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "TestApp", AccountName: username, SecretSize: 16})
	if err != nil {
		t.Fatalf("Failed to generate OTP key: %v", err)
	}

	user := &User{UUID: userUUID, Username: username, OTPSecret: key.Secret(), OTPEnabled: true}
	err = saveUser(user)
	if err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	// Scenario 1: Valid OTP code during login
	validCode, err := totp.GenerateCodeCustom(user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		t.Fatalf("Failed to generate valid OTP code for test: %v", err)
	}

	// Mimic validation from handle2FAVerify (part 2: Login)
	// Need to retrieve the user first based on UUID (as the handler would)
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

	// If valid, the handler would proceed to generateAndSaveToken(...)
	// We can test that function call separately (already done in TestGenerateAndSaveToken)
	if valid {
		t.Logf("Login scenario validation passed for valid code %s, token generation would proceed.", validCode)
		// Conceptual check: next step would be generateAndSaveToken(loginUser.UUID, "someDevice")
	}

	// Scenario 2: Invalid OTP code during login
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

	// If invalid, token generation should NOT proceed.
	if !valid {
		t.Logf("Login scenario validation failed for invalid code %s as expected.", invalidCode)
	}

}
