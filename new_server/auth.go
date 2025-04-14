package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	googleOAuth "golang.org/x/oauth2/google"
)

var (
	googleOauthConfig *oauth2.Config
	oauthStateString  = "random-pseudo-state"

	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrOTPRequired  = errors.New("otp required")
	ErrInvalidOTP   = errors.New("invalid otp code")
)

const AuthHeader = "X-Auth-Token"
const GoogleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo?access_token="

var googleClientID = os.Getenv("GOOGLE_CLIENT_ID")
var googleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")

var googleRedirectURL = "http://localhost:3000/auth/google/callback"

var pendingTwoFactor = sync.Map{}

type TwoFAPending struct {
	AuthID  string
	UserID  string
	Expires time.Time
	Code    string
}

func mapUserForResponse(user *User) map[string]any {
	return map[string]any{
		"UUID":       user.UUID,
		"Username":   user.Username,
		"IsAdmin":    user.IsAdmin,
		"IsManager":  user.IsManager,
		"OTPEnabled": user.OTPEnabled,
		"Trial":      user.Trial,
		"SubExpires": user.SubExpires,
		"Disabled":   user.Disaled,
	}
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func checkPasswordHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func googleOAuthEnabled() bool {
	if googleClientID == "" || googleClientSecret == "" {
		logger.Warn("Failed to setup Google OAuth", slog.Any("warn", "No credentials provided"))
		return false
	}

	return true
}

func setupGoogleOAuth() error {
	if googleOAuthEnabled() {
		logger.Warn("Failed to setup Google OAuth", slog.Any("warn", "No credentials provided"))
		return nil
	}
	googleOauthConfig = &oauth2.Config{
		RedirectURL:  googleRedirectURL,
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     googleOAuth.Endpoint,
	}
	return nil
}

func generateAuthState(deviceName string, redirect string) (string, error) {
	stateData := GoogleCallbackState{
		DeviceName: deviceName,
		Redirect:   redirect,
	}
	stateBytes, err := json.Marshal(stateData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}
	encodedState := base64.URLEncoding.EncodeToString(stateBytes) + "." + oauthStateString
	return encodedState, nil
}

func parseAuthState(stateParam string) (*GoogleCallbackState, error) {
	parts := strings.Split(stateParam, ".")
	if len(parts) < 2 || parts[len(parts)-1] != oauthStateString {
		return nil, errors.New("invalid state format or marker mismatch")
	}
	encodedPayload := strings.Join(parts[:len(parts)-1], ".")

	stateBytes, err := base64.URLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode state payload: %w", err)
	}

	var stateData GoogleCallbackState
	if err := json.Unmarshal(stateBytes, &stateData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state data: %w", err)
	}

	if stateData.DeviceName == "" {
		stateData.DeviceName = "Unknown Device"
	}

	return &stateData, nil
}

func authenticateRequest(c *fiber.Ctx) (*User, *AuthToken, error) {
	tokenUUID := c.Get(AuthHeader)
	if tokenUUID == "" {
		logger.Warn("Authentication failed: Missing auth header", slog.String("path", c.Path()))
		return nil, nil, ErrUnauthorized
	}

	token, err := getToken(tokenUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			logger.Warn("Authentication failed: Token not found", slog.String("tokenUUID", tokenUUID), slog.String("path", c.Path()))
			return nil, nil, ErrUnauthorized
		}
		logger.Error("Authentication error: Failed to retrieve token", slog.Any("error", err), slog.String("tokenUUID", tokenUUID))
		return nil, nil, fmt.Errorf("failed to retrieve token: %w", err)
	}

	user, err := getUser(token.UserUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			logger.Warn("Authentication failed: User for token not found", slog.String("userUUID", token.UserUUID), slog.String("tokenUUID", tokenUUID), slog.String("path", c.Path()))
			return nil, nil, ErrUnauthorized
		}
		logger.Error("Authentication error: Failed to retrieve user for token", slog.Any("error", err), slog.String("userUUID", token.UserUUID))
		return nil, nil, fmt.Errorf("failed to retrieve user: %w", err)
	}

	return user, token, nil
}

func isAdmin(user *User) bool {
	return user.IsAdmin
}

func isManager(user *User) bool {
	return (user.IsManager || user.IsAdmin)
}

func canManageUser(requestUser *User, targetUserUUID string) (bool, error) {
	if requestUser.IsAdmin {
		return true, nil
	}
	if requestUser.IsManager {
		targetUser, err := getUser(targetUserUUID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return false, fmt.Errorf("target user %s not found", targetUserUUID)
			}
			return false, fmt.Errorf("failed to get target user %s: %w", targetUserUUID, err)
		}
		if targetUser.IsAdmin || targetUser.IsManager {
			return false, nil
		}
		return true, nil
	}
	if requestUser.UUID == targetUserUUID {
		return true, nil
	}

	return false, nil
}

func canManageGroup(requestUser *User) bool {
	return isManager(requestUser)
}

func canManageServer(requestUser *User) bool {
	return isAdmin(requestUser)
}

func generateAndSaveToken(userUUID string, deviceName string) (*AuthToken, error) {
	newTokenUUID, err := uuid.NewRandom()
	if err != nil {
		logger.Error("Failed to generate token UUID", slog.Any("error", err))
		return nil, fmt.Errorf("could not generate token id: %w", err)
	}

	token := &AuthToken{
		UserUUID:   userUUID,
		TokenUUID:  newTokenUUID.String(),
		CreatedAt:  time.Now(),
		DeviceName: deviceName,
	}

	err = saveToken(token)
	if err != nil {
		logger.Error("Failed to save auth token", slog.Any("error", err), slog.String("userUUID", userUUID))
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	logger.Info("Generated new auth token", slog.String("userUUID", userUUID), slog.String("tokenUUID", token.TokenUUID), slog.String("deviceName", deviceName))
	return token, nil
}

func handleGoogleLogin(c *fiber.Ctx) error {
	deviceName := c.Query("deviceName", "Google login")
	redirectAfter := c.Query("redirect", "/auth/google/callback")
	state, err := generateAuthState(deviceName, redirectAfter)
	if err != nil {
		logger.Error("Failed to generate OAuth state", slog.Any("error", err))
		return fiber.NewError(http.StatusInternalServerError, "Could not initiate login flow")
	}
	url := googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return c.Redirect(url, http.StatusTemporaryRedirect)
}

func handleGoogleCallback(c *fiber.Ctx) error {
	stateParam := c.Query("state")
	stateData, err := parseAuthState(stateParam)
	if err != nil {
		logger.Warn("Invalid OAuth state received", slog.String("state", stateParam), slog.Any("error", err))
		return fiber.NewError(http.StatusBadRequest, "Invalid state parameter")
	}

	code := c.Query("code")
	if code == "" {
		logger.Warn("OAuth callback missing code", slog.String("state", stateParam))
		return fiber.NewError(http.StatusBadRequest, "Authorization code not found")
	}

	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		logger.Error("Failed to exchange Google token", slog.Any("error", err))
		return fiber.NewError(http.StatusInternalServerError, "Could not exchange token")
	}

	if !token.Valid() {
		logger.Error("Received invalid token from Google", slog.Any("token", token))
		return fiber.NewError(http.StatusInternalServerError, "Received invalid token")
	}

	response, err := http.Get(GoogleUserInfoURL + token.AccessToken)
	if err != nil {
		logger.Error("Failed to get user info from Google", slog.Any("error", err))
		return fiber.NewError(http.StatusInternalServerError, "Could not get user info")
	}
	defer response.Body.Close()

	contents, err := io.ReadAll(response.Body)
	if err != nil {
		logger.Error("Failed to read user info response", slog.Any("error", err))
		return fiber.NewError(http.StatusInternalServerError, "Could not read user info")
	}

	var googleUser struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(contents, &googleUser); err != nil {
		logger.Error("Failed to parse Google user info", slog.Any("error", err), slog.String("response", string(contents)))
		return fiber.NewError(http.StatusInternalServerError, "Could not parse user info")
	}

	user, err := getUserByGoogleID(googleUser.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			logger.Warn("Google OAuth attempt for unknown user", slog.String("googleId", googleUser.ID), slog.String("email", googleUser.Email))
			return fiber.NewError(http.StatusForbidden, "User not registered or linked. Please contact an administrator.")
		} else {
			logger.Error("Failed to lookup user by Google ID", slog.Any("error", err), slog.String("googleId", googleUser.ID))
			return fiber.NewError(http.StatusInternalServerError, "Database error during login")
		}
	} else {
		if user.GoogleID == "" {
			user.GoogleID = googleUser.ID
			if err := saveUser(user); err != nil {
				logger.Error("Failed to link Google ID to existing user", slog.Any("error", err), slog.String("userUUID", user.UUID))
			} else {
				logger.Info("Successfully linked Google ID to user", slog.String("userUUID", user.UUID))
			}
		}
	}

	if user.OTPEnabled {
		logger.Info("OTP is enabled for user, requiring verification", slog.String("userUUID", user.UUID))
		pendingInfo := PendingOTPInfo{
			UserUUID:    user.UUID,
			OTPRequired: true,
		}
		return c.Status(fiber.StatusAccepted).JSON(pendingInfo)
	}

	authToken, err := generateAndSaveToken(user.UUID, stateData.DeviceName)
	if err != nil {
		logger.Error("Failed to generate internal auth token after Google OAuth", slog.Any("error", err), slog.String("userUUID", user.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not complete login")
	}

	logger.Info("Successful Google OAuth login", slog.String("userUUID", user.UUID), slog.String("tokenUUID", authToken.TokenUUID))

	if stateData.Redirect != "" && stateData.Redirect != "/" {
		return c.JSON(fiber.Map{
			"message":    "Login successful",
			"token":      authToken.TokenUUID,
			"user":       user,
			"redirectTo": stateData.Redirect,
		})
	}
	return c.JSON(fiber.Map{
		"message": "Login successful",
		"token":   authToken.TokenUUID,
		"user":    mapUserForResponse(user),
	})
}

func handleTokenValidate(c *fiber.Ctx) error {
	user, token, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or missing token"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error validating token")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message":    "Token valid",
		"userUuid":   user.UUID,
		"isAdmin":    user.IsAdmin,
		"isManager":  user.IsManager,
		"tokenUuid":  token.TokenUUID,
		"issuedAt":   token.CreatedAt,
		"deviceName": token.DeviceName,
	})
}

func handleTokenDelete(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	targetTokenUUID := c.Params("token_uuid")
	if targetTokenUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing token UUID in path"})
	}

	tokenToDelete, err := getToken(targetTokenUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Target token not found"})
		}
		logger.Error("Failed to get token for deletion check", slog.Any("error", err), slog.String("targetTokenUUID", targetTokenUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error checking token ownership")
	}

	if tokenToDelete.UserUUID != requestUser.UUID && !requestUser.IsAdmin {
		logger.Warn("Forbidden attempt to delete token", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", tokenToDelete.UserUUID), slog.String("targetToken", targetTokenUUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You can only delete your own tokens"})
	}

	err = deleteToken(targetTokenUUID)
	if err != nil {
		logger.Error("Failed to delete token", slog.Any("error", err), slog.String("tokenUUID", targetTokenUUID))
	} else {
		logger.Info("Token deleted", slog.String("tokenUUID", targetTokenUUID), slog.String("deletedBy", requestUser.UUID))
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func handleTokenDeleteAll(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	targetUserUUID := c.Params("user_uuid")
	isTargetingSelf := false

	if targetUserUUID == "" || targetUserUUID == requestUser.UUID {
		targetUserUUID = requestUser.UUID
		isTargetingSelf = true
	} else if !requestUser.IsAdmin {
		logger.Warn("Forbidden attempt to delete all tokens for another user", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required to delete another user's tokens"})
	}
	if !isTargetingSelf && requestUser.IsAdmin {
		_, err := getUser(targetUserUUID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Target user not found"})
			}
			logger.Error("Failed to get target user before token deletion", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
			return fiber.NewError(http.StatusInternalServerError, "Failed to verify target user")
		}
	}

	err = deleteAllUserTokens(targetUserUUID)
	if err != nil {
		logger.Error("Failed to delete all tokens for user", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Failed to delete user tokens")
	}

	logger.Info("Deleted all tokens for user", slog.String("targetUserUUID", targetUserUUID), slog.String("deletedBy", requestUser.UUID))
	return c.SendStatus(fiber.StatusNoContent)
}

func handle2FASetup(c *fiber.Ctx) error {
	user, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	issuer := "YourAppName"
	accountName := user.Username

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		SecretSize:  16,
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
	})
	if err != nil {
		logger.Error("Failed to generate TOTP key", slog.Any("error", err), slog.String("userUUID", user.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not generate OTP key")
	}

	user.OTPSecret = key.Secret()
	user.OTPEnabled = false

	if err := saveUser(user); err != nil {
		logger.Error("Failed to save user with new OTP secret", slog.Any("error", err), slog.String("userUUID", user.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not save OTP configuration")
	}

	logger.Info("Generated OTP setup details for user", slog.String("userUUID", user.UUID), slog.String("accountName", accountName))

	response := OTPSetupResponse{
		ProvisioningUrl: key.URL(),
	}

	return c.Status(fiber.StatusOK).JSON(response)
}
func handle2FAConfirm(c *fiber.Ctx) error {
	var req TwoFAPending
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}

	user, err := getUser(req.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "User not found")
		}
		return errResponse(c, fiber.StatusNotFound, "Internal server error", slog.Any("err", err))
	}

	val, ok := pendingTwoFactor.Load(req.AuthID)
	if !ok {
		return errResponse(c, fiber.StatusUnauthorized, "Pending auth request not found")
	}

	pendingAuth, ok := val.(*TwoFAPending)
	if !ok {
		return errResponse(c, fiber.StatusInternalServerError, "Malformed pending auth request")
	}

	if time.Since(pendingAuth.Expires).Seconds() > 1 {
		return errResponse(c, fiber.StatusBadRequest, "Malformed pending auth request")
	}

	valid, err := totp.ValidateCustom(req.Code, user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return errResponse(c, fiber.StatusNotFound, "Unable to validate two factor authentication", slog.Any("err", err))
	}

	if !valid {
		return errResponse(c, fiber.StatusNotFound, "Unable to validate two factor authentication")
	}

	return c.JSON(mapUserForResponse(user))
}

func handle2FAEnable(c *fiber.Ctx) error {
	requestUser, _, authErr := authenticateRequest(c)

	var req OTPVerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}

	if req.OTPCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing otpCode field"})
	}

	if authErr == nil && requestUser != nil {
		if requestUser.OTPEnabled {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "OTP is already enabled for this user"})
		}
		if requestUser.OTPSecret == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "OTP secret not set up for this user"})
		}

		valid, err := totp.ValidateCustom(req.OTPCode, requestUser.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
			Period:    30,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			logger.Error("Error during OTP validation (setup)", slog.Any("error", err), slog.String("userUUID", requestUser.UUID))
			return fiber.NewError(http.StatusInternalServerError, "Error validating OTP code")
		}

		if !valid {
			logger.Warn("Invalid OTP code during setup verification", slog.String("userUUID", requestUser.UUID))
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid OTP code"})
		}

		requestUser.OTPEnabled = true
		if err := saveUser(requestUser); err != nil {
			logger.Error("Failed to enable OTP flag for user", slog.Any("error", err), slog.String("userUUID", requestUser.UUID))
			return fiber.NewError(http.StatusInternalServerError, "Failed to update user OTP status")
		}

		logger.Info("OTP successfully verified and enabled for user", slog.String("userUUID", requestUser.UUID))
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "OTP enabled successfully"})
	}

	userUUID := c.Params("user_uuid")
	if userUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User UUID required in path for OTP login verification"})
	}

	user, err := getUser(userUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving user")
	}

	if !user.OTPEnabled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "OTP is not enabled for this user"})
	}
	if user.OTPSecret == "" {
		logger.Error("Data inconsistency: OTP enabled but no secret found", slog.String("userUUID", user.UUID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "User OTP configuration error"})
	}

	valid, err := totp.ValidateCustom(req.OTPCode, user.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})

	if err != nil {
		logger.Error("Error during OTP validation (login)", slog.Any("error", err), slog.String("userUUID", user.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Error validating OTP code")
	}

	if !valid {
		logger.Warn("Invalid OTP code during login verification", slog.String("userUUID", user.UUID))
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid OTP code"})
	}

	deviceName := "Verified via OTP"

	authToken, err := generateAndSaveToken(user.UUID, deviceName)
	if err != nil {
		logger.Error("Failed to generate internal auth token after OTP verification", slog.Any("error", err), slog.String("userUUID", user.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not complete login")
	}

	logger.Info("Successful OTP verification during login", slog.String("userUUID", user.UUID), slog.String("tokenUUID", authToken.TokenUUID))
	return c.JSON(fiber.Map{
		"message": "Login successful",
		"token":   authToken.TokenUUID,
		"user":    mapUserForResponse(user),
	})
}
