// In auth_test.go
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// ... other imports
)

// --- Test Setup Helper (remains the same) ---
var testApp *fiber.App

func setupTestApp(t *testing.T) { /* ... exactly as before ... */
	if logger == nil { /* init logger */
	}
	setupTestDBGlobals(t) // MUST come first to clean DBs
	if testApp == nil {   // Create Fiber app instance and register routes
		testApp = fiber.New(fiber.Config{ /*...*/ })
		authGroup := testApp.Group("/auth")
		authGroup.Post("/login", handleLogin)
		authGroup.Post("/logout", handleLogout)
		userGroup := testApp.Group("/users")
		userGroup.Post("/", handleCreateUser)
		tokenGroup := authGroup.Group("/token")
		tokenGroup.Delete("/:token_uuid", handleTokenDelete)
		// Register other routes tested if needed...
	}
}

// --- Password Hashing Test (remains the same) ---
func TestPasswordHashing(t *testing.T) { /* ... as before ... */ }

// --- Create User with Password Test ---
func TestCreateUserWithPassword(t *testing.T) {
	setupTestApp(t)
	// Prerequisite admin/manager & token (use helper from previous answer)
	_, adminToken := createAndLoginTestUser(t, "adminCreateTest", "pass", true, true)

	newUsername := "newUserToCreate"
	newPassword := "p@ssword1"
	createUserBody, _ := json.Marshal(CreateUserRequest{Username: newUsername, Password: newPassword})
	reqCreate := httptest.NewRequest("POST", "/users", bytes.NewReader(createUserBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set(AuthHeader, adminToken)
	respCreate, _ := testApp.Test(reqCreate, -1)
	require.Equal(t, http.StatusCreated, respCreate.StatusCode)

	// Verify DB *does* contain the hash
	userUUID, err := getUserUUIDByUsername(newUsername)
	require.NoError(t, err)
	createdUser, err := getUser(userUUID)
	require.NoError(t, err)
	require.NotNil(t, createdUser)
	assert.True(t, checkPasswordHash(newPassword, createdUser.PasswordHash))
	assert.NotEmpty(t, createdUser.PasswordHash, "Hash should be saved in DB")

	// Verify Response body *does not* contain the hash
	var respBody map[string]any
	err = json.NewDecoder(respCreate.Body).Decode(&respBody)
	require.NoError(t, err)
	assert.Equal(t, newUsername, respBody["username"])
	assert.NotContains(t, respBody, "passwordHash", "Response body must not contain passwordHash")
	assert.NotContains(t, respBody, "otpSecret", "Response body must not contain otpSecret")
	assert.NotContains(t, respBody, "googleId", "Response body must not contain googleId")
}

// --- Login/Logout Handler Test ---
func TestHandleLoginLogout(t *testing.T) {
	setupTestApp(t)
	// Create user with password
	username := "testloginUser"
	password := "logmeinpass"
	hash, _ := hashPassword(password)
	user := &User{UUID: uuid.NewString(), Username: username, PasswordHash: hash}
	_ = saveUser(user)
	_ = setUsernameIndex(username, user.UUID)

	// --- Login Success ---
	loginBody, _ := json.Marshal(LoginRequest{Username: username, Password: password})
	reqLogin := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	respLogin, _ := testApp.Test(reqLogin, -1)
	require.Equal(t, http.StatusOK, respLogin.StatusCode)

	// Verify Response *body* filtering
	var loginResp map[string]interface{}
	err := json.NewDecoder(respLogin.Body).Decode(&loginResp)
	require.NoError(t, err)
	require.Contains(t, loginResp, "token")
	authToken, ok := loginResp["token"].(string)
	require.True(t, ok && authToken != "")
	require.Contains(t, loginResp, "user")
	userPart, ok := loginResp["user"].(map[string]interface{})
	require.True(t, ok)
	// **Assert absence** of sensitive fields in response map
	assert.NotContains(t, userPart, "passwordHash", "Login response 'user' must not contain passwordHash")
	assert.NotContains(t, userPart, "googleId", "Login response 'user' must not contain googleId")
	assert.NotContains(t, userPart, "otpSecret", "Login response 'user' must not contain otpSecret")
	assert.Equal(t, username, userPart["username"])

	// Verify token storage
	_, err = getToken(authToken)
	require.NoError(t, err, "Token should be saved")

	// --- Logout Success ---
	reqLogout := httptest.NewRequest("POST", "/auth/logout", nil)
	reqLogout.Header.Set(AuthHeader, authToken)
	respLogout, _ := testApp.Test(reqLogout, -1)
	require.Equal(t, http.StatusNoContent, respLogout.StatusCode)
	// Verify token deleted
	_, err = getToken(authToken)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))

	// ... (Other Login/Logout failure tests remain the same) ...
}

// ... include helper createAndLoginTestUser from previous answer ...
func createAndLoginTestUser(t *testing.T, username, password string, isAdmin, isManager bool) (*User, string) { /* ... */
	t.Helper()
	hash, _ := hashPassword(password)
	user := &User{UUID: uuid.NewString(), Username: username, PasswordHash: hash, IsAdmin: isAdmin, IsManager: isManager}
	require.NoError(t, saveUser(user))
	require.NoError(t, setUsernameIndex(username, user.UUID))
	if password == "" {
		return user, ""
	}
	loginBody, _ := json.Marshal(LoginRequest{Username: username, Password: password})
	reqLogin := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	respLogin, errLogin := testApp.Test(reqLogin, -1)
	require.NoError(t, errLogin)
	if respLogin.StatusCode != http.StatusOK {
		t.Logf("Login failed during test user setup %s: %d", username, respLogin.StatusCode)
		bodyBytes, _ := io.ReadAll(respLogin.Body)
		t.Logf("Body: %s", string(bodyBytes))
		require.FailNow(t, "Login failed")
	}
	var loginResp map[string]interface{}
	require.NoError(t, json.NewDecoder(respLogin.Body).Decode(&loginResp))
	authToken, ok := loginResp["token"].(string)
	require.True(t, ok)
	return user, authToken
}

// ... ensure 2FA tests are still okay, verifying fields manually ...
