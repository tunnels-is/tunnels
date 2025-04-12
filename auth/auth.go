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
	// State strings should be securely generated and validated. Using simple ones for example.
	oauthStateString = "random-pseudo-state" // Replace with secure random string generation + storage/validation

	// Custom error types for auth flow
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found") // Re-use this from db possibly
	ErrOTPRequired  = errors.New("otp required")
	ErrInvalidOTP   = errors.New("invalid otp code")
)

const AuthHeader = "X-Auth-Token"
const GoogleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo?access_token="

// You MUST set these via environment variables
var googleClientID = os.Getenv("GOOGLE_CLIENT_ID")
var googleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")

// IMPORTANT: Update this to your actual callback URL registered with Google
var googleRedirectURL = "http://localhost:3000/auth/google/callback"

// Helper to create response map from User, clearing sensitive fields
func mapUserForResponse(user *User) map[string]interface{} {
	if user == nil {
		return nil // Or empty map? {}
	}
	// Create map manually, excluding sensitive fields
	return map[string]interface{}{
		"uuid":       user.UUID,
		"username":   user.Username,
		"isAdmin":    user.IsAdmin,
		"isManager":  user.IsManager,
		"otpEnabled": user.OTPEnabled,
		// passwordHash, googleId, otpSecret ARE NOT included
	}
}

func hashPassword(password string) (string, error) { /* ... */
	if password == "" {
		return "", fmt.Errorf("pwd empty")
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
func checkPasswordHash(password, hash string) bool { /* ... */
	if password == "" || hash == "" {
		return false
	}
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
	// Using simple base64 encoding for the state value, could use JWT or other methods
	// NOTE: This state *value* should also be random and validated on callback for security.
	// This example combines payload and a pseudo-random state marker.
	// A better approach involves generating random state, storing it server-side mapped to the payload,
	// and verifying the returned state matches a stored one.
	encodedState := base64.URLEncoding.EncodeToString(stateBytes) + "." + oauthStateString
	return encodedState, nil
}

func parseAuthState(stateParam string) (*GoogleCallbackState, error) {
	parts := strings.Split(stateParam, ".")
	// Basic validation - could be more robust
	if len(parts) < 2 || parts[len(parts)-1] != oauthStateString {
		return nil, errors.New("invalid state format or marker mismatch")
	}
	encodedPayload := strings.Join(parts[:len(parts)-1], ".") // Rejoin if base64 includes '.'

	stateBytes, err := base64.URLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode state payload: %w", err)
	}

	var stateData GoogleCallbackState
	if err := json.Unmarshal(stateBytes, &stateData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state data: %w", err)
	}

	// Validate device name somewhat?
	if stateData.DeviceName == "" {
		stateData.DeviceName = "Unknown Device" // Default if missing
	}

	return &stateData, nil
}

// authenticateRequest extracts token, finds user, returns user and token struct or error
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
		return nil, nil, fmt.Errorf("failed to retrieve token: %w", err) // Internal error
	}

	// Conceptual: Check token expiry if needed
	// if time.Since(token.CreatedAt) > SomeExpiryDuration { ... }

	user, err := getUser(token.UserUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			logger.Warn("Authentication failed: User for token not found", slog.String("userUUID", token.UserUUID), slog.String("tokenUUID", tokenUUID), slog.String("path", c.Path()))
			// Optionally delete the orphaned token
			_ = deleteToken(tokenUUID)
			return nil, nil, ErrUnauthorized
		}
		logger.Error("Authentication error: Failed to retrieve user for token", slog.Any("error", err), slog.String("userUUID", token.UserUUID))
		return nil, nil, fmt.Errorf("failed to retrieve user: %w", err) // Internal error
	}

	return user, token, nil
}

// Authorization checks
func isAdmin(user *User) bool {
	return user != nil && user.IsAdmin
}

func isManager(user *User) bool {
	return user != nil && (user.IsManager || user.IsAdmin)
}

func canManageUser(requestUser *User, targetUserUUID string) (bool, error) {
	if requestUser == nil {
		return false, ErrUnauthorized
	}
	// Admins can manage anyone
	if requestUser.IsAdmin {
		return true, nil
	}
	// Managers can manage regular users (not other managers or admins)
	if requestUser.IsManager {
		targetUser, err := getUser(targetUserUUID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return false, fmt.Errorf("target user %s not found", targetUserUUID)
			}
			return false, fmt.Errorf("failed to get target user %s: %w", targetUserUUID, err)
		}
		if targetUser.IsAdmin || targetUser.IsManager {
			return false, nil // Managers cannot manage admins or other managers
		}
		return true, nil
	}
	// Regular users can manage themselves (for profile updates, maybe not roles)
	if requestUser.UUID == targetUserUUID {
		// Decide based on context if this is allowed, handlers should specify.
		// For simplicity, let's say generally no role changes by self.
		return true, nil // Allow self-view/basic profile update maybe
	}

	return false, nil // No rights by default
}

func canManageGroup(requestUser *User) bool {
	return isManager(requestUser) // Includes Admins
}

func canManageServer(requestUser *User) bool {
	return isAdmin(requestUser) // Only Admins
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

// --- Google OAuth Handlers ---

func handleGoogleLogin(c *fiber.Ctx) error {
	deviceName := c.Query("deviceName", "Unknown Browser")
	redirectAfter := c.Query("redirect", "/") // Optional redirect path after login
	state, err := generateAuthState(deviceName, redirectAfter)
	if err != nil {
		logger.Error("Failed to generate OAuth state", slog.Any("error", err))
		return fiber.NewError(http.StatusInternalServerError, "Could not initiate login flow")
	}
	url := googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline) // Request refresh token
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

	// Get user info from Google
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

	// Find or potentially create user in our DB
	user, err := getUserByGoogleID(googleUser.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// User not found by Google ID.
			// Decision: Automatically create user? Or require existing account?
			// For this example, let's *require* an existing account that needs linking or was created by an admin.
			// You could uncomment the creation logic if needed.
			logger.Warn("Google OAuth attempt for unknown user", slog.String("googleId", googleUser.ID), slog.String("email", googleUser.Email))

			/* // ---- Example User Creation Logic (if desired) ----
			   newUserUUID, _ := uuid.NewRandom()
			   newUser := &User{
			       UUID:      newUserUUID.String(),
			       Username:  googleUser.Name, // Or use email? Username needs careful handling (duplicates!)
			       GoogleID:  googleUser.ID,
			       IsAdmin:   false, // Never auto-admin!
			       IsManager: false, // Never auto-manager!
			       OTPSecret: "",
			       OTPEnabled:false,
			   }
			   if err := saveUser(newUser); err != nil {
			       logger.Error("Failed to automatically create user from Google OAuth", slog.Any("error", err), slog.String("googleId", googleUser.ID))
			       return fiber.NewError(http.StatusInternalServerError, "Failed to provision user account")
			   }
			   // Index the new user by username (careful with duplicates if username not unique)
			   if err := setUsernameIndex(newUser.Username, newUser.UUID); err != nil {
			        logger.Error("Failed to set username index for new OAuth user", slog.Any("error", err), slog.String("username", newUser.Username), slog.String("uuid", newUser.UUID))
			        // Handle this? Maybe requires manual fix. Continue login for now.
			   }
			   user = newUser // Continue with the newly created user
			   logger.Info("New user created via Google OAuth", slog.String("userUUID", user.UUID), slog.String("username", user.Username))
			    // ---- End Example User Creation ---- */

			return fiber.NewError(http.StatusForbidden, "User not registered or linked. Please contact an administrator.")

		} else {
			// Database error looking up user
			logger.Error("Failed to lookup user by Google ID", slog.Any("error", err), slog.String("googleId", googleUser.ID))
			return fiber.NewError(http.StatusInternalServerError, "Database error during login")
		}
	} else {
		// User found, update GoogleID if it wasn't set before (linking)
		if user.GoogleID == "" {
			user.GoogleID = googleUser.ID
			if err := saveUser(user); err != nil {
				logger.Error("Failed to link Google ID to existing user", slog.Any("error", err), slog.String("userUUID", user.UUID))
				// Continue login, but linking failed
			} else {
				logger.Info("Successfully linked Google ID to user", slog.String("userUUID", user.UUID))
			}
		}
	}

	// --- OTP Check ---
	if user.OTPEnabled {
		logger.Info("OTP is enabled for user, requiring verification", slog.String("userUUID", user.UUID))
		// Do not issue the full token yet. Return information needed for the OTP step.
		// The client must make a separate call to /auth/2fa/verify.
		// Store temporary state? Or just return the user UUID?
		// Simple approach: return user UUID, client calls verify endpoint.
		pendingInfo := PendingOTPInfo{
			UserUUID:    user.UUID,
			OTPRequired: true,
		}
		// Don't redirect here, return JSON indicating next step.
		return c.Status(fiber.StatusAccepted).JSON(pendingInfo) // 202 Accepted suggests processing isn't complete
	}

	// --- Generate and save our internal token ---
	authToken, err := generateAndSaveToken(user.UUID, stateData.DeviceName)
	if err != nil {
		logger.Error("Failed to generate internal auth token after Google OAuth", slog.Any("error", err), slog.String("userUUID", user.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not complete login")
	}

	// Optional: Redirect to a frontend page with the token, or return JSON
	logger.Info("Successful Google OAuth login", slog.String("userUUID", user.UUID), slog.String("tokenUUID", authToken.TokenUUID))

	if stateData.Redirect != "" && stateData.Redirect != "/" {
		// Example redirecting with token in query param (consider security implications)
		// Or store in localStorage/sessionStorage client-side after JSON response.
		// return c.Redirect(fmt.Sprintf("%s?token=%s", stateData.Redirect, authToken.TokenUUID), http.StatusFound)
		// Returning JSON is generally preferred for APIs
		return c.JSON(fiber.Map{
			"message":    "Login successful",
			"token":      authToken.TokenUUID,
			"user":       user, // Maybe omit sensitive fields like OTPSecret
			"redirectTo": stateData.Redirect,
		})

	}
	// Default: Return token info as JSON
	return c.JSON(fiber.Map{
		"message": "Login successful",
		"token":   authToken.TokenUUID,
		"user":    mapUserForResponse(user), // Omit sensitive fields here too
	})
}

// --- Token Management ---

func handleTokenValidate(c *fiber.Ctx) error {
	user, token, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or missing token"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error validating token")
	}

	// Token is valid, user exists
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
	requestUser, _, err := authenticateRequest(c) // Need auth to delete tokens
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

	// Validation: Allow users to delete their OWN tokens, or Admins to delete any token
	tokenToDelete, err := getToken(targetTokenUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Target token not found"})
		}
		logger.Error("Failed to get token for deletion check", slog.Any("error", err), slog.String("targetTokenUUID", targetTokenUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error checking token ownership")
	}

	// Authorization check
	if tokenToDelete.UserUUID != requestUser.UUID && !requestUser.IsAdmin {
		logger.Warn("Forbidden attempt to delete token", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", tokenToDelete.UserUUID), slog.String("targetToken", targetTokenUUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You can only delete your own tokens"})
	}

	err = deleteToken(targetTokenUUID)
	if err != nil {
		// Log error, but maybe don't expose details
		logger.Error("Failed to delete token", slog.Any("error", err), slog.String("tokenUUID", targetTokenUUID))
		// Badger's Delete might not error if key not found, could return 204 anyway
		// return fiber.NewError(http.StatusInternalServerError, "Failed to delete token")
	} else {
		logger.Info("Token deleted", slog.String("tokenUUID", targetTokenUUID), slog.String("deletedBy", requestUser.UUID))
	}

	return c.SendStatus(fiber.StatusNoContent) // Success, no content to return
}

func handleTokenDeleteAll(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c) // Need auth
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Parameter allows targeting a specific user (Admin only) or defaults to self
	targetUserUUID := c.Params("user_uuid")
	isTargetingSelf := false

	if targetUserUUID == "" || targetUserUUID == requestUser.UUID {
		targetUserUUID = requestUser.UUID // Deleting own tokens
		isTargetingSelf = true
	} else if !requestUser.IsAdmin {
		// User is trying to delete someone else's tokens without admin rights
		logger.Warn("Forbidden attempt to delete all tokens for another user", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required to delete another user's tokens"})
	}
	// If admin targets specific user, ensure targetUserUUID is valid
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

// --- 2FA Handlers ---

func handle2FASetup(c *fiber.Ctx) error {
	user, _, err := authenticateRequest(c)
	if err != nil {
		// Authentication required to set up 2FA for oneself
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Generate OTP secret
	// Issuer and AccountName should probably come from config or constants
	issuer := "YourAppName"
	accountName := user.Username // Or email, ensure uniqueness

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		SecretSize:  16,                // ~128 bits, recommended minimum
		Algorithm:   otp.AlgorithmSHA1, // Standard default
		Digits:      otp.DigitsSix,     // Standard default
	})
	if err != nil {
		logger.Error("Failed to generate TOTP key", slog.Any("error", err), slog.String("userUUID", user.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not generate OTP key")
	}

	// Store the *secret*, not the full key URL, associated with the user
	user.OTPSecret = key.Secret() // This is the base32 encoded secret
	user.OTPEnabled = false       // IMPORTANT: OTP is not enabled until verified by the user

	if err := saveUser(user); err != nil {
		logger.Error("Failed to save user with new OTP secret", slog.Any("error", err), slog.String("userUUID", user.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not save OTP configuration")
	}

	// Base64 encode the image bytes for JSON embedding

	logger.Info("Generated OTP setup details for user", slog.String("userUUID", user.UUID), slog.String("accountName", accountName))

	// Return provisioning URL and QR code image data

	response := OTPSetupResponse{
		ProvisioningUrl: key.URL(),
	}

	return c.Status(fiber.StatusOK).JSON(response)
}

func handle2FAVerify(c *fiber.Ctx) error {
	// This endpoint might be hit in two scenarios:
	// 1. Immediately after setup (user needs auth token + OTP code to enable).
	// 2. During login for an already-enabled user (user has no auth token yet, provides user UUID + OTP code).

	// Determine context: Check for Auth header first.
	requestUser, _, authErr := authenticateRequest(c) // Ignore token, just need user

	var req OTPVerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}

	if req.OTPCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing otpCode field"})
	}

	// --- Scenario 1: Enabling OTP after setup ---
	if authErr == nil && requestUser != nil {
		if requestUser.OTPEnabled {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "OTP is already enabled for this user"})
		}
		if requestUser.OTPSecret == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "OTP secret not set up for this user"})
		}

		// Validate the provided code against the stored secret
		valid, err := totp.ValidateCustom(req.OTPCode, requestUser.OTPSecret, time.Now().UTC(), totp.ValidateOpts{
			Period:    30, // Standard
			Skew:      1,  // Allow 1 period (30 seconds) clock skew
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

		// Code is valid, enable OTP for the user
		requestUser.OTPEnabled = true
		if err := saveUser(requestUser); err != nil {
			logger.Error("Failed to enable OTP flag for user", slog.Any("error", err), slog.String("userUUID", requestUser.UUID))
			return fiber.NewError(http.StatusInternalServerError, "Failed to update user OTP status")
		}

		logger.Info("OTP successfully verified and enabled for user", slog.String("userUUID", requestUser.UUID))
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "OTP enabled successfully"})
	}

	// --- Scenario 2: Verifying OTP during login (no initial auth token) ---
	// Requires User UUID to be passed, perhaps from the Google callback response
	userUUID := c.Params("user_uuid") // Expecting UUID in the path: /auth/2fa/verify/{user_uuid}
	if userUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User UUID required in path for OTP login verification"})
	}

	user, err := getUser(userUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		logger.Error("Failed to get user for OTP login verification", slog.Any("error", err), slog.String("userUUID", userUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving user")
	}

	if !user.OTPEnabled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "OTP is not enabled for this user"})
	}
	if user.OTPSecret == "" {
		// This shouldn't happen if OTPEnabled is true, indicates data inconsistency
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
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid OTP code"}) // Use 401 Unauthorized
	}

	// OTP is valid! Generate the actual auth token now.
	// Need device name - how to get it? Maybe passed in request? Or defaulted?
	// For now, use a generic device name.
	deviceName := "Verified via OTP" // Improve this if possible

	authToken, err := generateAndSaveToken(user.UUID, deviceName)
	if err != nil {
		logger.Error("Failed to generate internal auth token after OTP verification", slog.Any("error", err), slog.String("userUUID", user.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not complete login")
	}

	logger.Info("Successful OTP verification during login", slog.String("userUUID", user.UUID), slog.String("tokenUUID", authToken.TokenUUID))
	// Return the token
	return c.JSON(fiber.Map{
		"message": "Login successful",
		"token":   authToken.TokenUUID,
		"user":    mapUserForResponse(user), // Omit sensitive fields
	})
}
