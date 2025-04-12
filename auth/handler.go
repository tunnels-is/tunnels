package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices" // Requires Go 1.21+

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func errBadRequest(c *fiber.Ctx, err error) error { /* ... */
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": err.Error()})
}
func errUnauthorized(c *fiber.Ctx, msg string) error { /* ... */
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": msg})
}

// --- Login/Logout Handlers ---
func handleLogin(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return errBadRequest(c, err)
	}
	if req.Username == "" || req.Password == "" {
		return errBadRequest(c, errors.New("usr/pwd needed"))
	}

	userUUID, err := getUserUUIDByUsername(req.Username)
	if err != nil {
		return errUnauthorized(c, "invalid credentials")
	} // Hide user existence
	user, err := getUser(userUUID)
	if err != nil {
		return errUnauthorized(c, "invalid credentials")
	} // Internal inconsistency?

	if user.PasswordHash == "" || !checkPasswordHash(req.Password, user.PasswordHash) {
		return errUnauthorized(c, "invalid credentials")
	}

	if user.OTPEnabled { // Check OTP
		return c.Status(fiber.StatusAccepted).JSON(PendingOTPInfo{UserUUID: user.UUID, OTPRequired: true})
	}

	// Issue Token
	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "Unknown"
	}
	authToken, err := generateAndSaveToken(user.UUID, deviceName)
	if err != nil {
		logger.Error("Failed to generate token", slog.Any("error", "failed to generate token"), slog.String("username", req.Username))
		return fiber.NewError(http.StatusInternalServerError, "failed generating token")
	}

	logger.Info("Password login success", slog.String("user", user.UUID))
	return c.JSON(fiber.Map{
		"message": "Login successful",
		"token":   authToken.TokenUUID,
		"user":    mapUserForResponse(user), // Use helper map function
	})
}

func handleLogout(c *fiber.Ctx) error {
	_, token, err := authenticateRequest(c)
	if err != nil {
		return errUnauthorized(c, "invalid credentials")
	}
	if err := deleteToken(token.TokenUUID); err != nil {
		logger.Warn("Failed token delete logout", slog.Any("err", err))
	}
	logger.Info("User logged out", slog.String("user", token.UserUUID))
	return c.SendStatus(fiber.StatusNoContent)
}

// --- User Handlers ---

func handleCreateUser(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Only Admins or Managers can create users
	if !isManager(requestUser) { // isManager includes Admins
		logger.Warn("Forbidden attempt to create user", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}

	if req.Username == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Username cannot be empty"})
	}

	// Check if username is already taken
	_, err = getUserUUIDByUsername(req.Username)
	if err == nil {
		// Username exists
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Username already taken"})
	}
	if !errors.Is(err, ErrNotFound) {
		// DB error looking up username
		logger.Error("Failed to check username existence", slog.Any("error", err), slog.String("username", req.Username))
		return fiber.NewError(http.StatusInternalServerError, "Error checking username")
	}
	// If we get here, username is available (or DB error occurred, handled above)

	newUserUUID, _ := uuid.NewRandom()
	newUser := &User{
		UUID:     newUserUUID.String(),
		Username: req.Username,
		// Role assignment MUST be restricted
		IsAdmin:    false,
		IsManager:  false,
		OTPSecret:  "",
		OTPEnabled: false,
	}

	if req.Password != "" {
		hashedPassword, hashErr := hashPassword(req.Password)
		if hashErr != nil {
			logger.Error("Failed to create password", slog.Any("error", "unable to hash password"), slog.String("username", req.Username))
			return fiber.NewError(http.StatusInternalServerError, "Error creating password")
		}
		newUser.PasswordHash = hashedPassword
	}

	// Only Admins can set IsAdmin or IsManager flags on creation
	if requestUser.IsAdmin {
		// newUser.IsAdmin = req.IsAdmin                    // Admin can create other admins
		newUser.IsManager = req.IsManager || req.IsAdmin // Admin can create managers, and Admin implies Manager
	} else if requestUser.IsManager {
		// Managers cannot create Admins or other Managers
		if req.IsAdmin || req.IsManager {
			logger.Warn("Manager attempted to create user with elevated roles", slog.String("requestUser", requestUser.UUID), slog.String("targetUsername", req.Username))
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Managers can only create regular users"})
		}
		// IsAdmin and IsManager remain false (default)
	}

	if err := saveUser(newUser); err != nil {
		logger.Error("Failed to save new user", slog.Any("error", err), slog.String("username", req.Username))
		return fiber.NewError(http.StatusInternalServerError, "Could not save user")
	}
	// Add to username index
	if err := setUsernameIndex(newUser.Username, newUser.UUID); err != nil {
		logger.Error("Failed to set username index for new user", slog.Any("error", err), slog.String("username", newUser.Username), slog.String("uuid", newUser.UUID))
		// Consider what to do here - user exists but is unfindable by username? Critical error?
		// For now, log it and return success for user creation itself.
	}

	logger.Info("User created", slog.String("newUserUUID", newUser.UUID), slog.String("username", newUser.Username), slog.String("createdBy", requestUser.UUID))
	// Return the created user object (excluding sensitive fields ideally)
	return c.Status(fiber.StatusCreated).JSON(mapUserForResponse(newUser))
}

func handleGetUser(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	targetUserUUID := c.Params("uuid")
	if targetUserUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user UUID in path"})
	}

	// Authorization: Admin, Manager, or self
	allowed, authErr := canManageUser(requestUser, targetUserUUID) // Using this for general 'access' check
	if authErr != nil {
		logger.Error("Error checking user access permission", slog.Any("error", authErr), slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error checking permissions")
	}
	if !allowed {
		logger.Warn("Forbidden attempt to get user", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Access denied"})
	}

	user, err := getUser(targetUserUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		logger.Error("Failed to get user", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving user")
	}

	return c.JSON(mapUserForResponse(user))
}

func handleUpdateUser(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	targetUserUUID := c.Params("uuid")
	if targetUserUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user UUID in path"})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}

	// Get the existing user
	targetUser, err := getUser(targetUserUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		logger.Error("Failed to get user for update", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving user")
	}

	// Authorization Checks:
	originalUsername := targetUser.Username
	needsIndexUpdate := false

	// 1. Update Username? (Any authenticated user for self, Admin/Manager for others within rules)
	if req.Username != nil && *req.Username != targetUser.Username {
		canModify, authErr := canManageUser(requestUser, targetUserUUID)
		if authErr != nil {
			logger.Error("Error checking user access permission for username update", slog.Any("error", authErr), slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
			return fiber.NewError(http.StatusInternalServerError, "Error checking permissions")
		}
		if !canModify {
			logger.Warn("Forbidden attempt to update username", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Permission denied to update username"})
		}

		// Check if new username is taken
		existingUUID, err := getUserUUIDByUsername(*req.Username)
		if err == nil && existingUUID != targetUserUUID {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "New username is already taken"})
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			logger.Error("Failed to check new username existence", slog.Any("error", err), slog.String("username", *req.Username))
			return fiber.NewError(http.StatusInternalServerError, "Error checking username availability")
		}

		targetUser.Username = *req.Username
		needsIndexUpdate = true
	}

	// 2. Update Roles? (Admin/Manager privileges required, following specific rules)
	if req.IsAdmin != nil || req.IsManager != nil {
		if !isManager(requestUser) { // Need at least Manager rights to modify roles
			logger.Warn("Non-manager attempting role modification", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required to modify roles"})
		}

		// Only Admins can grant/revoke Admin
		if req.IsAdmin != nil && *req.IsAdmin != targetUser.IsAdmin && !requestUser.IsAdmin {
			logger.Warn("Manager attempting admin role modification", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only Admins can modify the Admin role"})
		}
		if req.IsAdmin != nil && requestUser.IsAdmin {
			// Prevent last admin from removing their own admin status? (Optional Safeguard)
			if targetUser.IsAdmin && !(*req.IsAdmin) && targetUserUUID == requestUser.UUID {
				// Count other admins? Complicates things. For now, allow self-removal. Consider adding a check.
				logger.Warn("Admin removing their own admin status", slog.String("userUUID", requestUser.UUID))
			}
			targetUser.IsAdmin = *req.IsAdmin
			// If granting Admin, automatically grant Manager too
			if targetUser.IsAdmin {
				targetUser.IsManager = true
			}
		}

		// Admin or Manager can modify Manager role (but Manager cannot make self/others Admin)
		if req.IsManager != nil && *req.IsManager != targetUser.IsManager {
			// Manager cannot grant manager role to an existing Admin (redundant) or themselves if already admin.
			// Admin can freely modify Manager role.
			if requestUser.IsManager && !requestUser.IsAdmin { // If requestor is exactly a Manager
				if targetUser.IsAdmin { // Cannot change Manager role of an Admin
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot modify Manager role for an Admin"})
				}
			}
			// Apply change if allowed by above logic (or if requestor is Admin)
			targetUser.IsManager = *req.IsManager
			// Ensure consistency: if Admin role is removed, Manager might need downgrade too unless explicit `isManager: true` is passed.
			if req.IsAdmin != nil && !(*req.IsAdmin) && targetUser.IsAdmin { // If admin was just revoked
				// And IsManager wasn't explicitly set to true in *this same request*
				if req.IsManager == nil || !(*req.IsManager) {
					targetUser.IsManager = false // Revoke manager if admin is revoked, unless kept explicitly
				}
			}
		}
		// Ensure final consistency: Admin always implies Manager
		if targetUser.IsAdmin {
			targetUser.IsManager = true
		}
	}

	// Save updated user
	if err := saveUser(targetUser); err != nil {
		logger.Error("Failed to save updated user", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not save user")
	}

	// Update username index if changed
	if needsIndexUpdate {
		// Delete old index entry
		if err := deleteUsernameIndex(originalUsername); err != nil {
			logger.Error("Failed to delete old username index entry", slog.Any("error", err), slog.String("username", originalUsername), slog.String("uuid", targetUserUUID))
			// Continue, but log error
		}
		// Set new index entry
		if err := setUsernameIndex(targetUser.Username, targetUser.UUID); err != nil {
			logger.Error("Failed to set new username index entry", slog.Any("error", err), slog.String("username", targetUser.Username), slog.String("uuid", targetUserUUID))
			// Continue, but log error
		}
	}

	logger.Info("User updated", slog.String("targetUserUUID", targetUser.UUID), slog.String("updatedBy", requestUser.UUID))
	return c.JSON(mapUserForResponse(targetUser))
}

func handleDeleteUser(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	targetUserUUID := c.Params("uuid")
	if targetUserUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing user UUID in path"})
	}

	// Authorization: Admins can delete anyone (except maybe themselves?). Managers can delete non-admins/non-managers.
	// Let's simplify: Only Admins can delete users for now.
	// Need the target user to check roles / username
	targetUser, err := getUser(targetUserUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		logger.Error("Failed to get user for delete check", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving user")
	}

	if !isAdmin(requestUser) {
		logger.Warn("Forbidden attempt to delete user", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required to delete users"})
	}
	// Optional: Prevent self-deletion?
	if requestUser.UUID == targetUserUUID {
		logger.Warn("Admin attempted self-deletion", slog.String("adminUUID", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot delete your own account"})
	}
	// Optional: Prevent deletion of the *last* admin? Requires counting admins. Skip for simplicity.

	// --- Clean up related data before deleting user ---

	// 1. Delete username index
	if targetUser.Username != "" {
		if err := deleteUsernameIndex(targetUser.Username); err != nil {
			logger.Error("Failed to delete username index before user deletion", slog.Any("error", err), slog.String("username", targetUser.Username), slog.String("uuid", targetUserUUID))
			// Log and continue with deletion attempt
		}
	}

	// 2. Remove user from all groups they belong to
	allGroups, err := listGroups()
	if err != nil {
		logger.Error("Failed to list groups for user cleanup", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		// Proceed with user deletion, but group membership might remain orphaned
	} else {
		for _, group := range allGroups {
			needsUpdate := false
			originalLen := len(group.UserUUIDs)
			// Filter out the user UUID
			group.UserUUIDs = slices.DeleteFunc(group.UserUUIDs, func(uuid string) bool {
				return uuid == targetUserUUID
			})
			if len(group.UserUUIDs) < originalLen {
				needsUpdate = true
			}

			if needsUpdate {
				if err := saveGroup(&group); err != nil {
					logger.Error("Failed to update group after removing user", slog.Any("error", err), slog.String("groupUUID", group.UUID), slog.String("userUUID", targetUserUUID))
					// Log and continue
				} else {
					logger.Debug("Removed user from group", slog.String("groupUUID", group.UUID), slog.String("userUUID", targetUserUUID))
				}
			}
		}
	}

	// 3. Delete all auth tokens for the user
	if err := deleteAllUserTokens(targetUserUUID); err != nil {
		logger.Error("Failed to delete auth tokens before user deletion", slog.Any("error", err), slog.String("userUUID", targetUserUUID))
		// Log and continue
	}

	// --- Delete the user ---
	if err := deleteUser(targetUserUUID); err != nil {
		// Log detailed error, but maybe return a generic server error
		logger.Error("Failed to delete user", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Failed to delete user")
		// Note: deleteItem doesn't typically error on not found, but underlying DB ops could fail.
	}

	logger.Info("User deleted", slog.String("targetUserUUID", targetUserUUID), slog.String("deletedBy", requestUser.UUID))
	return c.SendStatus(fiber.StatusNoContent)
}

func handleListUsers(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Admins or Managers can list users
	if !isManager(requestUser) {
		logger.Warn("Forbidden attempt to list users", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	users, err := listUsers()
	if err != nil {
		logger.Error("Failed to list users", slog.Any("error", err))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving user list")
	}

	responseList := make([]map[string]any, len(users))
	for i, u := range users {
		// MUST manually clear sensitive fields from 'u' before adding to response
		responseList[i] = mapUserForResponse(&u) // Use helper map function
	}
	return c.JSON(responseList)
}

// --- Group Handlers ---

func handleCreateGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Only Admins or Managers can create groups
	if !canManageGroup(requestUser) {
		logger.Warn("Forbidden attempt to create group", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	var req CreateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Group name cannot be empty"})
	}
	// TODO: Check if group name is unique? Requires listing/scanning. Skipped for simplicity.

	newGroupUUID, _ := uuid.NewRandom()
	newGroup := &Group{
		UUID:        newGroupUUID.String(),
		Name:        req.Name,
		UserUUIDs:   []string{},
		ServerUUIDs: []string{},
	}

	if err := saveGroup(newGroup); err != nil {
		logger.Error("Failed to save new group", slog.Any("error", err), slog.String("groupName", req.Name))
		return fiber.NewError(http.StatusInternalServerError, "Could not save group")
	}

	logger.Info("Group created", slog.String("newGroupUUID", newGroup.UUID), slog.String("groupName", newGroup.Name), slog.String("createdBy", requestUser.UUID))
	return c.Status(fiber.StatusCreated).JSON(newGroup)
}

func handleGetGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Any authenticated user can view group details? Let's restrict to Admins/Managers.
	if !canManageGroup(requestUser) { // Reuse canManageGroup for viewing too
		logger.Warn("Forbidden attempt to get group details", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	targetGroupUUID := c.Params("uuid")
	if targetGroupUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing group UUID in path"})
	}

	group, err := getGroup(targetGroupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Group not found"})
		}
		logger.Error("Failed to get group", slog.Any("error", err), slog.String("targetGroupUUID", targetGroupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving group")
	}

	return c.JSON(group)
}

func handleUpdateGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Admins/Managers can update group properties (like name)
	if !canManageGroup(requestUser) {
		logger.Warn("Forbidden attempt to update group", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	targetGroupUUID := c.Params("uuid")
	if targetGroupUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing group UUID in path"})
	}

	var req UpdateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}

	group, err := getGroup(targetGroupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Group not found"})
		}
		logger.Error("Failed to get group for update", slog.Any("error", err), slog.String("targetGroupUUID", targetGroupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving group")
	}

	if req.Name != nil && *req.Name != group.Name {
		if *req.Name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Group name cannot be empty"})
		}
		// Check for name uniqueness? (Skipped)
		group.Name = *req.Name
	} else {
		// No actual change requested in this simple example
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "No changes provided"})
	}

	if err := saveGroup(group); err != nil {
		logger.Error("Failed to save updated group", slog.Any("error", err), slog.String("groupUUID", group.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not save group")
	}

	logger.Info("Group updated", slog.String("groupUUID", group.UUID), slog.String("updatedBy", requestUser.UUID))
	return c.JSON(group)
}

func handleDeleteGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Admins/Managers can delete groups
	if !canManageGroup(requestUser) {
		logger.Warn("Forbidden attempt to delete group", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	targetGroupUUID := c.Params("uuid")
	if targetGroupUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing group UUID in path"})
	}

	// Optionally: Check if group exists before attempting delete
	_, err = getGroup(targetGroupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Group not found"})
		}
		logger.Error("Failed to get group for delete check", slog.Any("error", err), slog.String("targetGroupUUID", targetGroupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error checking group existence")
	}

	// Group deletion doesn't automatically remove users/servers from it (they just lose association).
	// The group object itself is deleted.
	if err := deleteGroup(targetGroupUUID); err != nil {
		logger.Error("Failed to delete group", slog.Any("error", err), slog.String("targetGroupUUID", targetGroupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Failed to delete group")
	}

	logger.Info("Group deleted", slog.String("groupUUID", targetGroupUUID), slog.String("deletedBy", requestUser.UUID))
	return c.SendStatus(fiber.StatusNoContent)
}

func handleListGroups(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Admins/Managers can list groups
	if !canManageGroup(requestUser) {
		logger.Warn("Forbidden attempt to list groups", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	groups, err := listGroups()
	if err != nil {
		logger.Error("Failed to list groups", slog.Any("error", err))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving group list")
	}

	return c.JSON(groups)
}

// --- Group Membership Handlers ---

func handleAddUserToGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Admins/Managers manage group membership
	if !canManageGroup(requestUser) {
		logger.Warn("Forbidden attempt to add user to group", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	groupUUID := c.Params("group_uuid")
	userUUID := c.Params("user_uuid")
	if groupUUID == "" || userUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing group or user UUID in path"})
	}

	// Verify group exists
	group, err := getGroup(groupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Group not found"})
		}
		logger.Error("Failed to get group for adding user", slog.Any("error", err), slog.String("groupUUID", groupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving group")
	}

	// Verify user exists
	_, err = getUser(userUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		logger.Error("Failed to get user for adding to group", slog.Any("error", err), slog.String("userUUID", userUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving user")
	}

	// Add user UUID if not already present (using standard library 'slices')
	if !slices.Contains(group.UserUUIDs, userUUID) {
		group.UserUUIDs = append(group.UserUUIDs, userUUID)
		if err := saveGroup(group); err != nil {
			logger.Error("Failed to save group after adding user", slog.Any("error", err), slog.String("groupUUID", groupUUID), slog.String("userUUID", userUUID))
			return fiber.NewError(http.StatusInternalServerError, "Could not update group membership")
		}
		logger.Info("User added to group", slog.String("userUUID", userUUID), slog.String("groupUUID", groupUUID), slog.String("addedBy", requestUser.UUID))
	} else {
		// User already in group, return success (idempotent) or a specific message
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User already in group"})
	}

	return c.Status(fiber.StatusOK).JSON(group) // Return updated group
}

func handleRemoveUserFromGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Admins/Managers manage group membership
	if !canManageGroup(requestUser) {
		logger.Warn("Forbidden attempt to remove user from group", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	groupUUID := c.Params("group_uuid")
	userUUID := c.Params("user_uuid")
	if groupUUID == "" || userUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing group or user UUID in path"})
	}

	group, err := getGroup(groupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Group not found"})
		}
		logger.Error("Failed to get group for removing user", slog.Any("error", err), slog.String("groupUUID", groupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving group")
	}

	// No need to check if user exists, just remove the UUID if present

	originalLen := len(group.UserUUIDs)
	group.UserUUIDs = slices.DeleteFunc(group.UserUUIDs, func(uuid string) bool {
		return uuid == userUUID
	})

	if len(group.UserUUIDs) < originalLen {
		// UUID was found and removed, save the group
		if err := saveGroup(group); err != nil {
			logger.Error("Failed to save group after removing user", slog.Any("error", err), slog.String("groupUUID", groupUUID), slog.String("userUUID", userUUID))
			return fiber.NewError(http.StatusInternalServerError, "Could not update group membership")
		}
		logger.Info("User removed from group", slog.String("userUUID", userUUID), slog.String("groupUUID", groupUUID), slog.String("removedBy", requestUser.UUID))
		return c.Status(fiber.StatusOK).JSON(group) // Return updated group
	} else {
		// User was not in the group, return success (idempotent) or not found?
		// Let's return success indicating the state is achieved.
		return c.SendStatus(fiber.StatusNoContent)
		// Alternative: return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found in this group"})
	}
}

func handleAddServerToGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Only Admins manage server group assignments
	if !isAdmin(requestUser) {
		logger.Warn("Forbidden attempt to add server to group by non-admin", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required"})
	}

	groupUUID := c.Params("group_uuid")
	serverUUID := c.Params("server_uuid")
	if groupUUID == "" || serverUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing group or server UUID in path"})
	}

	// Verify group exists
	group, err := getGroup(groupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Group not found"})
		}
		logger.Error("Failed to get group for adding server", slog.Any("error", err), slog.String("groupUUID", groupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving group")
	}

	// Verify server exists
	_, err = getServer(serverUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Server not found"})
		}
		logger.Error("Failed to get server for adding to group", slog.Any("error", err), slog.String("serverUUID", serverUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving server")
	}

	if !slices.Contains(group.ServerUUIDs, serverUUID) {
		group.ServerUUIDs = append(group.ServerUUIDs, serverUUID)
		if err := saveGroup(group); err != nil {
			logger.Error("Failed to save group after adding server", slog.Any("error", err), slog.String("groupUUID", groupUUID), slog.String("serverUUID", serverUUID))
			return fiber.NewError(http.StatusInternalServerError, "Could not update group membership")
		}
		logger.Info("Server added to group", slog.String("serverUUID", serverUUID), slog.String("groupUUID", groupUUID), slog.String("addedBy", requestUser.UUID))
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "Server already in group"})
	}

	return c.Status(fiber.StatusOK).JSON(group)
}

func handleRemoveServerFromGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Only Admins manage server group assignments
	if !isAdmin(requestUser) {
		logger.Warn("Forbidden attempt to remove server from group by non-admin", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required"})
	}

	groupUUID := c.Params("group_uuid")
	serverUUID := c.Params("server_uuid")
	if groupUUID == "" || serverUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing group or server UUID in path"})
	}

	group, err := getGroup(groupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Group not found"})
		}
		logger.Error("Failed to get group for removing server", slog.Any("error", err), slog.String("groupUUID", groupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving group")
	}

	originalLen := len(group.ServerUUIDs)
	group.ServerUUIDs = slices.DeleteFunc(group.ServerUUIDs, func(uuid string) bool {
		return uuid == serverUUID
	})

	if len(group.ServerUUIDs) < originalLen {
		if err := saveGroup(group); err != nil {
			logger.Error("Failed to save group after removing server", slog.Any("error", err), slog.String("groupUUID", groupUUID), slog.String("serverUUID", serverUUID))
			return fiber.NewError(http.StatusInternalServerError, "Could not update group membership")
		}
		logger.Info("Server removed from group", slog.String("serverUUID", serverUUID), slog.String("groupUUID", groupUUID), slog.String("removedBy", requestUser.UUID))
		return c.Status(fiber.StatusOK).JSON(group)
	} else {
		return c.SendStatus(fiber.StatusNoContent) // State achieved
	}
}

// --- Server Handlers ---

func handleCreateServer(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Only Admins can create servers
	if !canManageServer(requestUser) {
		logger.Warn("Forbidden attempt to create server by non-admin", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required"})
	}

	var req CreateServerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}
	if req.Name == "" || req.Hostname == "" { // IP is optional? Assume yes for now.
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Server name and hostname cannot be empty"})
	}
	// TODO: Validate Hostname/IP format? Unique checks?

	newServerUUID, _ := uuid.NewRandom()
	newServer := &Server{
		UUID:      newServerUUID.String(),
		Name:      req.Name,
		Hostname:  req.Hostname,
		IPAddress: req.IPAddress,
	}

	if err := saveServer(newServer); err != nil {
		logger.Error("Failed to save new server", slog.Any("error", err), slog.String("serverName", req.Name))
		return fiber.NewError(http.StatusInternalServerError, "Could not save server")
	}

	logger.Info("Server created", slog.String("newServerUUID", newServer.UUID), slog.String("serverName", newServer.Name), slog.String("createdBy", requestUser.UUID))
	return c.Status(fiber.StatusCreated).JSON(newServer)
}

func handleGetServer(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Only Admins can view server details (or maybe managers too? Stick to Admin for now)
	if !canManageServer(requestUser) { // Or maybe a broader view permission?
		logger.Warn("Forbidden attempt to get server details by non-admin", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required"})
	}

	targetServerUUID := c.Params("uuid")
	if targetServerUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing server UUID in path"})
	}

	server, err := getServer(targetServerUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Server not found"})
		}
		logger.Error("Failed to get server", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving server")
	}

	return c.JSON(server)
}

func handleUpdateServer(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Only Admins can update servers
	if !canManageServer(requestUser) {
		logger.Warn("Forbidden attempt to update server by non-admin", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required"})
	}

	targetServerUUID := c.Params("uuid")
	if targetServerUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing server UUID in path"})
	}

	var req UpdateServerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}

	server, err := getServer(targetServerUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Server not found"})
		}
		logger.Error("Failed to get server for update", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving server")
	}

	changed := false
	if req.Name != nil && *req.Name != server.Name {
		if *req.Name == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Server name cannot be empty"})
		}
		server.Name = *req.Name
		changed = true
	}
	if req.Hostname != nil && *req.Hostname != server.Hostname {
		if *req.Hostname == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Server hostname cannot be empty"})
		}
		// Validate format?
		server.Hostname = *req.Hostname
		changed = true
	}
	if req.IPAddress != nil && *req.IPAddress != server.IPAddress {
		// Validate format? Allow empty to clear?
		server.IPAddress = *req.IPAddress
		changed = true
	}

	if !changed {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"message": "No changes provided"})
	}

	if err := saveServer(server); err != nil {
		logger.Error("Failed to save updated server", slog.Any("error", err), slog.String("serverUUID", server.UUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not save server")
	}

	logger.Info("Server updated", slog.String("serverUUID", server.UUID), slog.String("updatedBy", requestUser.UUID))
	return c.JSON(server)
}

func handleDeleteServer(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Only Admins can delete servers
	if !canManageServer(requestUser) {
		logger.Warn("Forbidden attempt to delete server by non-admin", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required"})
	}

	targetServerUUID := c.Params("uuid")
	if targetServerUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing server UUID in path"})
	}

	// Optional: Check existence
	_, err = getServer(targetServerUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Server not found"})
		}
		logger.Error("Failed to get server for delete check", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving server")
	}

	// Clean up group associations
	allGroups, err := listGroups()
	if err != nil {
		logger.Error("Failed to list groups for server cleanup", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
		// Proceed with server deletion, but group membership might remain orphaned
	} else {
		for _, group := range allGroups {
			needsUpdate := false
			originalLen := len(group.ServerUUIDs)
			// Filter out the server UUID
			group.ServerUUIDs = slices.DeleteFunc(group.ServerUUIDs, func(uuid string) bool {
				return uuid == targetServerUUID
			})
			if len(group.ServerUUIDs) < originalLen {
				needsUpdate = true
			}

			if needsUpdate {
				if err := saveGroup(&group); err != nil {
					logger.Error("Failed to update group after removing server", slog.Any("error", err), slog.String("groupUUID", group.UUID), slog.String("serverUUID", targetServerUUID))
					// Log and continue
				} else {
					logger.Debug("Removed server from group", slog.String("groupUUID", group.UUID), slog.String("serverUUID", targetServerUUID))
				}
			}
		}
	}

	if err := deleteServer(targetServerUUID); err != nil {
		logger.Error("Failed to delete server", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
		return fiber.NewError(http.StatusInternalServerError, "Failed to delete server")
	}

	logger.Info("Server deleted", slog.String("serverUUID", targetServerUUID), slog.String("deletedBy", requestUser.UUID))
	return c.SendStatus(fiber.StatusNoContent)
}

func handleListServers(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	// Authorization: Only Admins can list servers (or Managers?) - Admin only for now.
	if !canManageServer(requestUser) {
		logger.Warn("Forbidden attempt to list servers by non-admin", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required"})
	}

	servers, err := listServers()
	if err != nil {
		logger.Error("Failed to list servers", slog.Any("error", err))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving server list")
	}

	return c.JSON(servers)
}
