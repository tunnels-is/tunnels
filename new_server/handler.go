package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func cleanTwoFactorPendingMap() {
	pendingTwoFactor.Range(func(key, value any) bool {
		//
		val, ok := value.(*types.TwoFAPending)
		if !ok {
			logger.Error("unable to cast pending auth to type", slog.Any("value", value))
		}
		if time.Since(val.Expires).Seconds() > 1 {
			logger.Info("two factor expired", slog.Any("key", key.(string)))
			pendingTwoFactor.Delete(key)
		}
		return true
	})
}

func handleLogin(c *fiber.Ctx) error {
	defer recoverAndLog()

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return errResponse(c, fiber.StatusBadRequest, "Invalid request body")
	}
	if req.Username == "" || req.Password == "" {
		return errResponse(c, fiber.StatusBadRequest, "Username and password are required")
	}

	userUUID, err := getUserUUIDByUsername(req.Username)
	if err != nil {
		return errResponse(c, fiber.StatusUnauthorized, "Invalid user or password")
	}
	user, err := getUser(userUUID)
	if err != nil {
		return errResponse(c, fiber.StatusUnauthorized, "Invalid user or password")
	}

	if user.PasswordHash == "" || !checkPasswordHash(req.Password, user.PasswordHash) {
		return errResponse(c, fiber.StatusUnauthorized, "Invalid user or password")
	}

	if user.OTPEnabled {
		authID := uuid.NewString()
		pendingAuth := &types.TwoFAPending{
			Expires: time.Now().Add(5 * time.Minute),
			AuthID:  authID,
			UserID:  user.UUID,
		}
		pendingTwoFactor.Store(authID, pendingAuth)

		return c.Status(fiber.StatusAccepted).JSON(pendingAuth)
	}

	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "Unknown"
	}
	authToken, err := generateAndSaveToken(user.UUID, deviceName)
	if err != nil {
		logger.Error("Failed to generate token", slog.Any("error", "failed to generate token"), slog.String("username", req.Username))
		return errResponse(c, http.StatusInternalServerError, "failed generating token", slog.Any("error", err), slog.String("username", req.Username))
	}

	logger.Info("Password login success", slog.String("user", user.UUID))
	return c.JSON(fiber.Map{
		"token": authToken.TokenUUID,
		"user":  mapUserForResponse(user),
	})
}

func handleLogout(c *fiber.Ctx) error {
	_, token, err := authenticateRequest(c)
	if err != nil {
		return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
	}
	if err := deleteToken(token.TokenUUID); err != nil {
		logger.Warn("Failed token delete logout", slog.Any("err", err))
	}
	logger.Info("User logged out", slog.String("user", token.UserUUID))
	return c.SendStatus(fiber.StatusNoContent)
}

func handleCreateUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return errResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err), slog.Any("error", err))
	}
	userMutex.Lock()
	defer func() {
		userMutex.Unlock()
	}()
	defer recoverAndLog()

	if req.Username == "" {
		return errResponse(c, fiber.StatusBadRequest, "Username cannot be empty")
	}

	if req.Password != req.SecondPassword {
		return errResponse(c, fiber.StatusBadRequest, "Password do not match")
	}
	if req.Password != "" {
		return errResponse(c, fiber.StatusBadRequest, "Password cannot be empty")
	}

	hashedPassword, hashErr := hashPassword(req.Password)
	if hashErr != nil {
		return errResponse(c, http.StatusInternalServerError, "unable to hash password")
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
	newUser.PasswordHash = hashedPassword

	if err := setUsernameIndex(newUser.Username, newUser.UUID); err != nil {
		return errResponse(c, http.StatusBadRequest, "username already taken")
	}

	if err := saveUser(newUser); err != nil {
		return errResponse(c, http.StatusInternalServerError, "unable to save user")
	}

	logger.Info("User created", slog.String("newUserUUID", newUser.UUID), slog.String("username", newUser.Username))
	return c.Status(fiber.StatusCreated).JSON(mapUserForResponse(newUser))
}

func handleGetUser(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required")
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	targetUserUUID := c.Params("uuid")
	if targetUserUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing user UUID in path")
	}

	allowed, authErr := canManageUser(requestUser, targetUserUUID)
	if authErr != nil {
		return errResponse(c, http.StatusInternalServerError, "Error checking permissions", slog.Any("error", authErr), slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
	}
	if !allowed {
		return errResponse(c, fiber.StatusForbidden, "Access denied", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
	}

	user, err := getUser(targetUserUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "User not found", slog.String("targetUserUUID", targetUserUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving user", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
	}

	return c.JSON(mapUserForResponse(user))
}

func handleUpdateUser(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required")
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	targetUserUUID := c.Params("uuid")
	if targetUserUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing user UUID in path")
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return errResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err), slog.Any("error", err))
	}

	userMutex.Lock()
	defer func() {
		userMutex.Unlock()
	}()
	defer recoverAndLog()

	allowed, authErr := canManageUser(requestUser, targetUserUUID)
	if authErr != nil {
		return errResponse(c, http.StatusInternalServerError, "Error checking permissions", slog.Any("error", authErr), slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
	}
	if !allowed {
		return errResponse(c, fiber.StatusForbidden, "Access denied", slog.String("requestUser", requestUser.UUID), slog.String("targetUser", targetUserUUID))
	}

	userMutex.Lock()
	defer func() {
		userMutex.Unlock()
	}()
	defer recoverAndLog()

	targetUser, err := getUser(targetUserUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "User not found", slog.String("targetUserUUID", targetUserUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving user", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
	}

	targetUser.Disaled = req.Disaled
	targetUser.APIKey = req.APIKey

	if err := saveUser(targetUser); err != nil {
		logger.Error("Failed to save updated user", slog.Any("error", err), slog.String("targetUserUUID", targetUserUUID))
		return fiber.NewError(http.StatusInternalServerError, "Could not save user")
	}

	logger.Info("User updated", slog.String("targetUserUUID", targetUser.UUID), slog.String("updatedBy", requestUser.UUID))
	return c.JSON(mapUserForResponse(targetUser))
}

func handleListUsers(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required")
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !isManager(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin or Manager privileges required", slog.String("requestUser", requestUser.UUID))
	}

	users, err := listUsers()
	if err != nil {
		logger.Error("Failed to list users", slog.Any("error", err))
		return errResponse(c, http.StatusInternalServerError, "Error retrieving user list", slog.Any("error", err))
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
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required")
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageGroup(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin or Manager privileges required", slog.String("requestUser", requestUser.UUID))
	}

	var req CreateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return errResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err), slog.Any("error", err))
	}

	if req.Name == "" {
		return errResponse(c, fiber.StatusBadRequest, "Group name cannot be empty")
	}

	newGroupUUID, _ := uuid.NewRandom()
	newGroup := &Group{
		UUID:        newGroupUUID.String(),
		Name:        req.Name,
		UserUUIDs:   []string{},
		ServerUUIDs: []string{},
	}

	if err := saveGroup(newGroup); err != nil {
		return errResponse(c, http.StatusInternalServerError, "Could not save group", slog.Any("error", err), slog.String("groupName", req.Name))
	}

	logger.Info("Group created", slog.String("newGroupUUID", newGroup.UUID), slog.String("groupName", newGroup.Name), slog.String("createdBy", requestUser.UUID))
	return c.Status(fiber.StatusCreated).JSON(newGroup)
}

func handleGetGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required")
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageGroup(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin or Manager privileges required", slog.String("requestUser", requestUser.UUID))
	}

	targetGroupUUID := c.Params("uuid")
	if targetGroupUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing group UUID in path")
	}

	group, err := getGroup(targetGroupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Group not found", slog.String("targetGroupUUID", targetGroupUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving group", slog.Any("error", err), slog.String("targetGroupUUID", targetGroupUUID))
	}

	return c.JSON(group)
}

func handleUpdateGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required")
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageGroup(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin or Manager privileges required", slog.String("requestUser", requestUser.UUID))
	}

	targetGroupUUID := c.Params("uuid")
	if targetGroupUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing group UUID in path")
	}

	var req UpdateGroupRequest
	if err := c.BodyParser(&req); err != nil {
		return errResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err), slog.Any("error", err))
	}

	groupMutex.Lock()
	defer func() {
		groupMutex.Unlock()
	}()
	defer recoverAndLog()

	group, err := getGroup(targetGroupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Group not found", slog.String("targetGroupUUID", targetGroupUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving group", slog.Any("error", err), slog.String("targetGroupUUID", targetGroupUUID))
	}
	group.Name = req.Name

	if err := saveGroup(group); err != nil {
		return errResponse(c, http.StatusInternalServerError, "Could not save group", slog.Any("error", err), slog.String("groupUUID", group.UUID))
	}

	logger.Info("Group updated", slog.String("groupUUID", group.UUID), slog.String("updatedBy", requestUser.UUID))
	return c.JSON(group)
}

func handleDeleteGroup(c *fiber.Ctx) error {

	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required")
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageGroup(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin or Manager privileges required", slog.String("requestUser", requestUser.UUID))
	}

	targetGroupUUID := c.Params("uuid")
	if targetGroupUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing group UUID in path")
	}

	_, err = getGroup(targetGroupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Group not found", slog.String("targetGroupUUID", targetGroupUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error checking group existence", slog.Any("error", err), slog.String("targetGroupUUID", targetGroupUUID))
	}

	if err := deleteGroup(targetGroupUUID); err != nil {
		return errResponse(c, http.StatusInternalServerError, "Failed to delete group", slog.Any("error", err), slog.String("targetGroupUUID", targetGroupUUID))
	}

	logger.Info("Group deleted", slog.String("groupUUID", targetGroupUUID), slog.String("deletedBy", requestUser.UUID))
	return c.SendStatus(fiber.StatusNoContent)
}

func handleListGroups(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required")
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageGroup(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin or Manager privileges required", slog.String("requestUser", requestUser.UUID))
	}

	groups, err := listGroups()
	if err != nil {
		return errResponse(c, http.StatusInternalServerError, "Error retrieving group list", slog.Any("error", err))
	}

	return c.JSON(groups)
}

func handleAddUserToGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageGroup(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin or Manager privileges required", slog.String("requestUser", requestUser.UUID))
	}

	groupUUID := c.Params("group_uuid")
	userUUID := c.Params("user_uuid")
	if groupUUID == "" || userUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing group or user UUID in path",
			slog.String("groupUUID", groupUUID), slog.String("userUUID", userUUID))
	}

	groupMutex.Lock()
	defer func() {
		groupMutex.Unlock()
	}()
	defer recoverAndLog()

	group, err := getGroup(groupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Group not found", slog.String("groupUUID", groupUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving group",
			slog.Any("error", err), slog.String("groupUUID", groupUUID))
	}

	_, err = getUser(userUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "User not found", slog.String("userUUID", userUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving user",
			slog.Any("error", err), slog.String("userUUID", userUUID))
	}

	if !slices.Contains(group.UserUUIDs, userUUID) {
		group.UserUUIDs = append(group.UserUUIDs, userUUID)
		if err := saveGroup(group); err != nil {
			return errResponse(c, http.StatusInternalServerError, "Could not update group membership",
				slog.Any("error", err), slog.String("groupUUID", groupUUID), slog.String("userUUID", userUUID))
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleRemoveUserFromGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageGroup(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin or Manager privileges required", slog.String("requestUser", requestUser.UUID))
	}

	groupUUID := c.Params("group_uuid")
	userUUID := c.Params("user_uuid")
	if groupUUID == "" || userUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing group or user UUID in path",
			slog.String("groupUUID", groupUUID), slog.String("userUUID", userUUID))
	}

	groupMutex.Lock()
	defer func() {
		groupMutex.Unlock()
	}()
	defer recoverAndLog()

	group, err := getGroup(groupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Group not found", slog.String("groupUUID", groupUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving group",
			slog.Any("error", err), slog.String("groupUUID", groupUUID))
	}

	originalLen := len(group.UserUUIDs)
	group.UserUUIDs = slices.DeleteFunc(group.UserUUIDs, func(uuid string) bool {
		return uuid == userUUID
	})

	if len(group.UserUUIDs) < originalLen {
		if err := saveGroup(group); err != nil {
			return errResponse(c, http.StatusInternalServerError, "Could not update group membership", slog.Any("error", err))
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleAddServerToGroup(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !isAdmin(requestUser) {
		logger.Warn("Forbidden attempt to add server to group by non-admin", slog.String("requestUser", requestUser.UUID))
		return errResponse(c, fiber.StatusForbidden, "Admin privileges required", slog.String("requestUser", requestUser.UUID))
	}

	groupUUID := c.Params("group_uuid")
	serverUUID := c.Params("server_uuid")
	if groupUUID == "" || serverUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing group or server UUID in path",
			slog.String("groupUUID", groupUUID), slog.String("serverUUID", serverUUID))
	}

	groupMutex.Lock()
	defer func() {
		groupMutex.Unlock()
	}()
	defer recoverAndLog()

	group, err := getGroup(groupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Group not found", slog.String("groupUUID", groupUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving group",
			slog.Any("error", err), slog.String("groupUUID", groupUUID))
	}

	_, err = getServer(serverUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Server not found", slog.String("serverUUID", serverUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving server",
			slog.Any("error", err), slog.String("serverUUID", serverUUID))
	}

	if !slices.Contains(group.ServerUUIDs, serverUUID) {
		group.ServerUUIDs = append(group.ServerUUIDs, serverUUID)
		if err := saveGroup(group); err != nil {
			return errResponse(c, http.StatusInternalServerError, "Could not update group membership",
				slog.Any("error", err), slog.String("groupUUID", groupUUID), slog.String("serverUUID", serverUUID))
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleRemoveServerFromGroup(c *fiber.Ctx) error {

	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !isAdmin(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin privileges required", slog.String("requestUser", requestUser.UUID))
	}

	groupUUID := c.Params("group_uuid")
	serverUUID := c.Params("server_uuid")
	if groupUUID == "" || serverUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing group or server UUID in path",
			slog.String("groupUUID", groupUUID), slog.String("serverUUID", serverUUID))
	}

	groupMutex.Lock()
	defer func() {
		groupMutex.Unlock()
	}()
	defer recoverAndLog()

	group, err := getGroup(groupUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Group not found", slog.String("groupUUID", groupUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving group",
			slog.Any("error", err), slog.String("groupUUID", groupUUID))
	}

	originalLen := len(group.ServerUUIDs)
	group.ServerUUIDs = slices.DeleteFunc(group.ServerUUIDs, func(uuid string) bool {
		return uuid == serverUUID
	})

	if len(group.ServerUUIDs) < originalLen {
		if err := saveGroup(group); err != nil {
			return errResponse(c, http.StatusInternalServerError, "Could not update group membership", slog.Any("error", err))
		}
	}

	return c.SendStatus(fiber.StatusOK)
}

func handleCreateServer(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageServer(requestUser) {
		logger.Warn("Forbidden attempt to create server by non-admin", slog.String("requestUser", requestUser.UUID))
		return errResponse(c, fiber.StatusForbidden, "Admin privileges required", slog.String("requestUser", requestUser.UUID))
	}

	var req Server
	if err := c.BodyParser(&req); err != nil {
		return errResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err), slog.Any("error", err))
	}

	req.UUID = uuid.NewString()

	if err := saveServer(&req); err != nil {
		logger.Error("Failed to save new server", slog.Any("error", err), slog.String("serverName", req.Tag))
		return errResponse(c, http.StatusInternalServerError, "Could not save server",
			slog.Any("error", err), slog.String("serverName", req.Tag))
	}

	logger.Info("Server created", slog.String("newServerUUID", req.UUID), slog.String("serverName", req.Tag), slog.String("createdBy", requestUser.UUID))
	return c.SendStatus(fiber.StatusCreated)
}

func handleGetServer(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageServer(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin privileges required", slog.String("requestUser", requestUser.UUID))
	}

	targetServerUUID := c.Params("uuid")
	if targetServerUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing server UUID in path")
	}

	server, err := getServer(targetServerUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Server not found", slog.String("targetServerUUID", targetServerUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving server", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
	}

	return c.JSON(server)
}

func handleUpdateServer(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageServer(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin privileges required", slog.String("requestUser", requestUser.UUID))
	}

	targetServerUUID := c.Params("uuid")
	if targetServerUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing server UUID in path")
	}

	serverMutex.Lock()
	defer func() {
		serverMutex.Unlock()
	}()
	defer recoverAndLog()

	var req Server
	if err := c.BodyParser(&req); err != nil {
		return errResponse(c, fiber.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err), slog.Any("error", err))
	}

	server, err := getServer(targetServerUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Server not found", slog.String("targetServerUUID", targetServerUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving server", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
	}

	if err := saveServer(server); err != nil {
		return errResponse(c, http.StatusInternalServerError, "Could not save server", slog.Any("error", err), slog.String("serverUUID", server.UUID))
	}

	return c.JSON(server)
}

func handleDeleteServer(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageServer(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin privileges required", slog.String("requestUser", requestUser.UUID))
	}

	targetServerUUID := c.Params("uuid")
	if targetServerUUID == "" {
		return errResponse(c, fiber.StatusBadRequest, "Missing server UUID in path")
	}

	_, err = getServer(targetServerUUID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return errResponse(c, fiber.StatusNotFound, "Server not found", slog.String("targetServerUUID", targetServerUUID))
		}
		return errResponse(c, http.StatusInternalServerError, "Error retrieving server", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
	}

	if err := deleteServer(targetServerUUID); err != nil {
		return errResponse(c, http.StatusInternalServerError, "Failed to delete server", slog.Any("error", err), slog.String("targetServerUUID", targetServerUUID))
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

	logger.Info("Server deleted", slog.String("serverUUID", targetServerUUID), slog.String("deletedBy", requestUser.UUID))
	return c.SendStatus(fiber.StatusNoContent)
}

func handleListServers(c *fiber.Ctx) error {
	requestUser, _, err := authenticateRequest(c)
	if err != nil {
		if errors.Is(err, ErrUnauthorized) {
			return errResponse(c, fiber.StatusUnauthorized, "Authentication required", slog.Any("error", err))
		}
		return errResponse(c, http.StatusInternalServerError, "Error authenticating request", slog.Any("error", err))
	}

	if !canManageServer(requestUser) {
		return errResponse(c, fiber.StatusForbidden, "Admin privileges required", slog.String("requestUser", requestUser.UUID))
	}

	servers, err := listServers()
	if err != nil {
		return errResponse(c, http.StatusInternalServerError, "Error retrieving server list", slog.Any("error", err))
	}

	return c.JSON(servers)
}
