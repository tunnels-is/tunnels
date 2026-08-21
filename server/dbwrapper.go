package main

import (
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func DB_DeleteDeviceByID(id uuid.UUID) (err error) {
	return BBolt_DeleteDeviceByID(id.String())
}

func DB_UpdateDevice(D *types.Device) (err error) {
	return BBolt_UpdateDevice(D)
}

func DB_GetDevices(limit, offset int64) (DL []*types.Device, err error) {
	return BBolt_GetDevices(limit, offset)
}

func DB_GetAllDevices() (DL []*types.Device, err error) {
	return BBolt_GetAllDevices()
}

func DB_GetDevicesByUserID(userID uuid.UUID) (DL []*types.Device, err error) {
	return BBolt_GetDevicesByUserID(userID)
}

func DB_getUsers(limit, offset int64) (UL []*User, err error) {
	return BBolt_getUsers(limit, offset)
}

func DB_getUsersLatest(topN, batchSize int) (users []*User, total, trial, active int64, err error) {
	return BBolt_getUsersLatest(topN, batchSize)
}

func DB_findUserByID(UID uuid.UUID) (USER *User, err error) {
	return BBolt_findUserByID(UID.String())
}

func DB_CreateUser(U *User) (err error) {
	return BBolt_CreateUser(U)
}

func DB_findUserByEmail(Email string) (USER *User, err error) {
	return BBolt_findUserByEmail(Email)
}

func DB_updateUserDeviceTokens(TU *UPDATE_USER_TOKENS) (err error) {
	return BBolt_updateUserDeviceTokens(TU)
}

func DB_updateUserSubTime(u *User) (err error) {
	return BBolt_updateUserSubTime(u)
}

func DB_updateUser(UF *USER_UPDATE_FORM) (err error) {
	return BBolt_updateUser(UF)
}

func DB_updateUserAdmin(UF *USER_ADMIN_UPDATE_FORM) (err error) {
	return BBolt_updateUserAdmin(UF)
}

func DB_userUpdateTwoFactorCodes(TFP *TWO_FACTOR_DB_PACKAGE) (err error) {
	return BBolt_userUpdateTwoFactorCodes(TFP)
}

func DB_updateUserRecoveryCodes(uid uuid.UUID, codes []byte) (err error) {
	return BBolt_updateUserRecoveryCodes(uid, codes)
}

func DB_userResetPassword(user *User) error {
	return BBolt_userResetPassword(user)
}

func DB_FindServersWithoutGroups(limit, offset int64) (DL []*types.Server, err error) {
	return BBolt_FindServersWithoutGroups(limit, offset)
}

func DB_FindServersByGroups(groups []uuid.UUID, limit, offset int64) (DL []*types.Server, err error) {
	return BBolt_FindServersByGroups(groups, limit, offset)
}

func DB_FindEntitiesByGroupID(id uuid.UUID, objType string, limit, offset int64) (IL []any, err error) {
	return BBolt_FindEntitiesByGroupID(id.String(), objType, limit, offset)
}

func DB_UpdateGroup(G *Group) (err error) {
	return BBolt_UpdateGroup(G)
}

func DB_UpdateServer(S *types.Server) (RS *types.Server, err error) {
	return BBolt_UpdateServer(S)
}

func DB_SetServerWireGuardPubKey(id uuid.UUID, pubKey string) error {
	return BBolt_SetServerWireGuardPubKey(id, pubKey)
}

func DB_CreateDevice(D *types.Device) (err error) {
	return BBolt_CreateDevice(D)
}

func DB_CreateGroup(G *Group) (err error) {
	return BBolt_CreateGroup(G)
}

func DB_CreateServer(S *types.Server) (err error) {
	return BBolt_CreateServer(S)
}

func DB_FindServerByAPIKey(apiKey string) (*types.Server, error) {
	return BBolt_FindServerByAPIKey(apiKey)
}

func DB_FindAllServers(limit, offset int64) ([]*types.Server, error) {
	return BBolt_FindAllServers(limit, offset)
}

func DB_FindServerByID(ID uuid.UUID) (S *types.Server, err error) {
	return BBolt_FindServerByID(ID.String())
}

func DB_UserActivateKey(SubExpiration time.Time, Key *LicenseKey, userID uuid.UUID) (err error) {
	return BBolt_UserActivateKey(SubExpiration, Key, userID.String())
}

func DB_AddToGroup(groupID uuid.UUID, typeID uuid.UUID, objType string) (err error) {
	return BBolt_AddToGroup(groupID.String(), typeID.String(), objType)
}

func DB_RemoveFromGroup(groupID uuid.UUID, typeID uuid.UUID, objType string) (err error) {
	return BBolt_RemoveFromGroup(groupID.String(), typeID.String(), objType)
}

func DB_FindDeviceByID(id uuid.UUID) (dev *types.Device, err error) {
	return BBolt_FindDeviceByID(id.String())
}

func DB_FindDeviceByWGKey(wgKey string) (dev *types.Device, err error) {
	return BBolt_FindDeviceByWGKey(wgKey)
}

func DB_findGroupByID(id uuid.UUID) (G *Group, err error) {
	return BBolt_findGroupByID(id.String())
}

func DB_DeleteGroupByID(id uuid.UUID) (err error) {
	return BBolt_DeleteGroupByID(id.String())
}

func DB_DeleteUserByID(id uuid.UUID) (err error) {
	return BBolt_DeleteUserByID(id.String())
}

func DB_DeleteServerByID(id uuid.UUID) (err error) {
	return BBolt_DeleteServerByID(id.String())
}

func DB_ListGroups(limit, offset int64) (gl []*Group, err error) {
	return BBolt_findGroups(limit, offset)
}
