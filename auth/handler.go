package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func errBadRequest(c *fiber.Ctx, err error) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": err.Error()})
}
func errUnauthorized(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": msg})
}

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
	}
	user, err := getUser(userUUID)
	if err != nil {
		return errUnauthorized(c, "invalid credentials")
	}

	if user.PasswordHash == "" || !checkPasswordHash(req.Password, user.PasswordHash) {
		return errUnauthorized(c, "invalid credentials")
	}

	if user.OTPEnabled {
		return c.Status(fiber.StatusAccepted).JSON(PendingOTPInfo{UserUUID: user.UUID, OTPRequired: true})
	}

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
		"user":    mapUserForResponse(user),
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

func handleCreateUser(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	if !isManager(requestUser) {
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

	_, err = getUserUUIDByUsername(req.Username)
	if err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Username already taken"})
	}
	if !errors.Is(err, ErrNotFound) {
		logger.Error("Failed to check username existence", slog.Any("error", err), slog.String("username", req.Username))
		return fiber.NewError(http.StatusInternalServerError, "Error checking username")
	}

	newUserUUID, _ := uuid.NewRandom()
	newUser := &User{
		UUID:       newUserUUID.String(),
		Username:   req.Username,
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

	if requestUser.IsAdmin {
		newUser.IsManager = req.IsManager || req.IsAdmin
	} else if requestUser.IsManager {
		if req.IsAdmin || req.IsManager {
			logger.Warn("Manager attempted to create user with elevated roles", slog.String("requestUser", requestUser.UUID), slog.String("targetUsername", req.Username))
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Managers can only create regular users"})
		}
	}

	if err := saveUser(newUser); err != nil {
		logger.Error("Failed to save new user", slog.Any("error", err), slog.String("username", req.Username))
		return fiber.NewError(http.StatusInternalServerError, "Could not save user")
	}
	if err := setUsernameIndex(newUser.Username, newUser.UUID); err != nil {
		logger.Error("Failed to set username index for new user", slog.Any("error", err), slog.String("username", newUser.Username), slog.String("uuid", newUser.UUID))
	}

	logger.Info("User created", slog.String("newUserUUID", newUser.UUID), slog.String("username", newUser.Username), slog.String("createdBy", requestUser.UUID))
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

	allowed, authErr := canManageUser(requestUser, targetUserUUID)
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

	targetUser, err := getUser(targetUserUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		logger.Error("Failed to get user for update", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving user")
	}

	originalUsername := targetUser.Username
	needsIndexUpdate := false

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

	if req.IsAdmin != nil || req.IsManager != nil {
		if !isManager(requestUser) {
			logger.Warn("Non-manager attempting role modification", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required to modify roles"})
		}

		if req.IsAdmin != nil && *req.IsAdmin != targetUser.IsAdmin && !requestUser.IsAdmin {
			logger.Warn("Manager attempting admin role modification", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only Admins can modify the Admin role"})
		}
		if req.IsAdmin != nil && requestUser.IsAdmin {
			if targetUser.IsAdmin && !(*req.IsAdmin) && targetUserUUID == requestUser.UUID {
				logger.Warn("Admin removing their own admin status", slog.String("userUUID", requestUser.UUID))
			}
			targetUser.IsAdmin = *req.IsAdmin
			if targetUser.IsAdmin {
				targetUser.IsManager = true
			}
		}

		if req.IsManager != nil && *req.IsManager != targetUser.IsManager {
			if requestUser.IsManager && !requestUser.IsAdmin {
				if targetUser.IsAdmin {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot modify Manager role for an Admin"})
				}
			}
			targetUser.IsManager = *req.IsManager
			if req.IsAdmin != nil && !(*req.IsAdmin) && targetUser.IsAdmin {
				if req.IsManager == nil || !(*req.IsManager) {
					targetUser.IsManager = false
				}
			}
		}
		if targetUser.IsAdmin {
			targetUser.IsManager = true
		}
	}

	if err := saveUser(targetUser); err != nil {
		logger.Error("Failed to save updated user", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not save user")
	}

	if needsIndexUpdate {
		if err := deleteUsernameIndex(originalUsername); err != nil {
			logger.Error("Failed to delete old username index entry", slog.Any("error", err), slog.String("username", originalUsername), slog.String("uuid", targetUserUUID))
		}
		if err := setUsernameIndex(targetUser.Username, targetUser.UUID); err != nil {
			logger.Error("Failed to set new username index entry", slog.Any("error", err), slog.String("username", targetUser.Username), slog.String("uuid", targetUserUUID))
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
	if requestUser.UUID == targetUserUUID {
		logger.Warn("Admin attempted self-deletion", slog.String("adminUUID", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot delete your own account"})
	}

	if targetUser.Username != "" {
		if err := deleteUsernameIndex(targetUser.Username); err != nil {
			logger.Error("Failed to delete username index before user deletion", slog.Any("error", err), slog.String("username", targetUser.Username), slog.String("uuid", targetUserUUID))
		}
	}

	allGroups, err := listGroups()
	if err != nil {
		logger.Error("Failed to list groups for user cleanup", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
	} else {
		for _, group := range allGroups {
			needsUpdate := false
			originalLen := len(group.UserUUIDs)
			group.UserUUIDs = slices.DeleteFunc(group.UserUUIDs, func(uuid string) bool {
				return uuid == targetUserUUID
			})
			if len(group.UserUUIDs) < originalLen {
				needsUpdate = true
			}

			if needsUpdate {
				if err := saveGroup(&group); err != nil {
					logger.Error("Failed to update group after removing user", slog.Any("error", err), slog.String("groupUUID", group.UUID), slog.String("userUUID", targetUserUUID))
				} else {
					logger.Debug("Removed user from group", slog.String("groupUUID", group.UUID), slog.String("userUUID", targetUserUUID))
				}
			}
		}
	}

	if err := deleteAllUserTokens(targetUserUUID); err != nil {
		logger.Error("Failed to delete auth tokens before user deletion", slog.Any("error", err), slog.String("userUUID", targetUserUUID))
	}

	if err := deleteUser(targetUserUUID); err != nil {
		logger.Error("Failed to delete user", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Failed to delete user")
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
		responseList[i] = mapUserForResponse(&u)
	}
	return c.JSON(responseList)
}

func handleCreateGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

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

	if !canManageGroup(requestUser) {
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
		group.Name = *req.Name
	} else {
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

	if !canManageGroup(requestUser) {
		logger.Warn("Forbidden attempt to delete group", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin or Manager privileges required"})
	}

	targetGroupUUID := c.Params("uuid")
	if targetGroupUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing group UUID in path"})
	}

	_, err = getGroup(targetGroupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Group not found"})
		}
		logger.Error("Failed to get group for delete check", slog.Any("error", err), slog.String("targetGroupUUID", targetGroupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error checking group existence")
	}

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

func handleAddUserToGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	if !canManageGroup(requestUser) {
		logger.Warn("Forbidden attempt to add user to group", slog.String("requestUser", requestUser.UUID))
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
		logger.Error("Failed to get group for adding user", slog.Any("error", err), slog.String("groupUUID", groupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving group")
	}

	_, err = getUser(userUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		logger.Error("Failed to get user for adding to group", slog.Any("error", err), slog.String("userUUID", userUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving user")
	}

	if !slices.Contains(group.UserUUIDs, userUUID) {
		group.UserUUIDs = append(group.UserUUIDs, userUUID)
		if err := saveGroup(group); err != nil {
			logger.Error("Failed to save group after adding user", slog.Any("error", err), slog.String("groupUUID", groupUUID), slog.String("userUUID", userUUID))
			return fiber.NewError(http.StatusInternalServerError, "Could not update group membership")
		}
		logger.Info("User added to group", slog.String("userUUID", userUUID), slog.String("groupUUID", groupUUID), slog.String("addedBy", requestUser.UUID))
	} else {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User already in group"})
	}

	return c.Status(fiber.StatusOK).JSON(group)
}

func handleRemoveUserFromGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

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

	originalLen := len(group.UserUUIDs)
	group.UserUUIDs = slices.DeleteFunc(group.UserUUIDs, func(uuid string) bool {
		return uuid == userUUID
	})

	if len(group.UserUUIDs) < originalLen {
		if err := saveGroup(group); err != nil {
			logger.Error("Failed to save group after removing user", slog.Any("error", err), slog.String("groupUUID", groupUUID), slog.String("userUUID", userUUID))
			return fiber.NewError(http.StatusInternalServerError, "Could not update group membership")
		}
		logger.Info("User removed from group", slog.String("userUUID", userUUID), slog.String("groupUUID", groupUUID), slog.String("removedBy", requestUser.UUID))
		return c.Status(fiber.StatusOK).JSON(group)
	} else {
		return c.SendStatus(fiber.StatusNoContent)
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

	if !isAdmin(requestUser) {
		logger.Warn("Forbidden attempt to add server to group by non-admin", slog.String("requestUser", requestUser.UUID))
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
		logger.Error("Failed to get group for adding server", slog.Any("error", err), slog.String("groupUUID", groupUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving group")
	}

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
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleCreateServer(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Authentication required"})
		}
		return fiber.NewError(http.StatusInternalServerError, "Error authenticating request")
	}

	if !canManageServer(requestUser) {
		logger.Warn("Forbidden attempt to create server by non-admin", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required"})
	}

	var req CreateServerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid request body: %v", err)})
	}
	if req.Name == "" || req.Hostname == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Server name and hostname cannot be empty"})
	}

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

	if !canManageServer(requestUser) {
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
		server.Hostname = *req.Hostname
		changed = true
	}
	if req.IPAddress != nil && *req.IPAddress != server.IPAddress {
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

	if !canManageServer(requestUser) {
		logger.Warn("Forbidden attempt to delete server by non-admin", slog.String("requestUser", requestUser.UUID))
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin privileges required"})
	}

	targetServerUUID := c.Params("uuid")
	if targetServerUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Missing server UUID in path"})
	}

	_, err = getServer(targetServerUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Server not found"})
		}
		logger.Error("Failed to get server for delete check", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
		return fiber.NewError(http.StatusInternalServerError, "Error retrieving server")
	}

	allGroups, err := listGroups()
	if err != nil {
		logger.Error("Failed to list groups for server cleanup", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
	} else {
		for _, group := range allGroups {
			needsUpdate := false
			originalLen := len(group.ServerUUIDs)
			group.ServerUUIDs = slices.DeleteFunc(group.ServerUUIDs, func(uuid string) bool {
				return uuid == targetServerUUID
			})
			if len(group.ServerUUIDs) < originalLen {
				needsUpdate = true
			}

			if needsUpdate {
				if err := saveGroup(&group); err != nil {
					logger.Error("Failed to update group after removing server", slog.Any("error", err), slog.String("groupUUID", group.UUID), slog.String("serverUUID", targetServerUUID))
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
