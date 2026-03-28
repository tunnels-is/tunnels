package main

import (
	"time"

	"github.com/tunnels-is/tunnels/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func DB_DeleteDeviceByID(id primitive.ObjectID) (err error) {
	return BBolt_DeleteDeviceByID(objectIDToString(id))
}

func DB_UpdateDevice(D *types.Device) (err error) {
	return BBolt_UpdateDevice(D)
}

func DB_GetDevices(limit, offset int64) (DL []*types.Device, err error) {
	return BBolt_GetDevices(limit, offset)
}

func DB_GetDevicesByUserID(userID primitive.ObjectID) (DL []*types.Device, err error) {
	return BBolt_GetDevicesByUserID(userID)
}

func DB_getUsers(limit, offset int64) (UL []*User, err error) {
	return BBolt_getUsers(limit, offset)
}

func DB_findUserByAPIKey(Key string) (USER *User, err error) {
	return BBolt_findUserByAPIKey(Key)
}

func DB_findUserByID(UID primitive.ObjectID) (USER *User, err error) {
	return BBolt_findUserByID(objectIDToString(UID))
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

func DB_toggleUserSubscriptionStatus(UF *USER_UPDATE_SUB_FORM) (err error) {
	return BBolt_toggleUserSubscriptionStatus(UF)
}

func DB_userUpdateTwoFactorCodes(TFP *TWO_FACTOR_DB_PACKAGE) (err error) {
	return BBolt_userUpdateTwoFactorCodes(TFP)
}

func DB_userResetPassword(user *User) error {
	return BBolt_userResetPassword(user)
}

func DB_FindServersWithoutGroups(limit, offset int64) (DL []*types.Server, err error) {
	return BBolt_FindServersWithoutGroups(limit, offset)
}

func DB_FindServersByGroups(groups []primitive.ObjectID, limit, offset int64) (DL []*types.Server, err error) {
	return BBolt_FindServersByGroups(objectIDSliceToString(groups), limit, offset)
}

func DB_FindEntitiesByGroupID(id primitive.ObjectID, objType string, limit, offset int64) (IL []any, err error) {
	return BBolt_FindEntitiesByGroupID(objectIDToString(id), objType, limit, offset)
}

func DB_UpdateGroup(G *Group) (err error) {
	return BBolt_UpdateGroup(G)
}

func DB_UpdateServer(S *types.Server) (RS *types.Server, err error) {
	return BBolt_UpdateServer(S)
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

func DB_SetServerWGSubnet(id primitive.ObjectID, subnet string) error {
	return BBolt_SetServerWGSubnet(objectIDToString(id), subnet)
}

func DB_FindAllServers() ([]*types.Server, error) {
	return BBolt_FindAllServers()
}

func DB_FindServerByID(ID primitive.ObjectID) (S *types.Server, err error) {
	return BBolt_FindServerByID(objectIDToString(ID))
}

func DB_WipeUserConfirmCode(UF *USER_ENABLE_QUERY) (err error) {
	return BBolt_WipeUserConfirmCode(UF)
}

func DB_UserActivateKey(SubExpiration time.Time, Key *LicenseKey, userID primitive.ObjectID) (err error) {
	return BBolt_UserActivateKey(SubExpiration, Key, objectIDToString(userID))
}

func DB_AddToGroup(groupID primitive.ObjectID, typeID primitive.ObjectID, objType string) (err error) {
	return BBolt_AddToGroup(objectIDToString(groupID), objectIDToString(typeID), objType)
}

func DB_RemoveFromGroup(groupID primitive.ObjectID, typeID primitive.ObjectID, objType string) (err error) {
	return BBolt_RemoveFromGroup(objectIDToString(groupID), objectIDToString(typeID), objType)
}

func DB_FindDeviceByID(id primitive.ObjectID) (dev *types.Device, err error) {
	return BBolt_FindDeviceByID(objectIDToString(id))
}

func DB_findGroupByID(id primitive.ObjectID) (G *Group, err error) {
	return BBolt_findGroupByID(objectIDToString(id))
}

func DB_DeleteGroupByID(id primitive.ObjectID) (err error) {
	return BBolt_DeleteGroupByID(objectIDToString(id))
}

func DB_findGroups() (gl []*Group, err error) {
	return BBolt_findGroups()
}

func DB_CreateWGServerConfig(cfg *types.WGServerConfig) error {
	return BBolt_CreateWGServerConfig(cfg)
}

func DB_FindWGServerConfigByID(id primitive.ObjectID) (*types.WGServerConfig, error) {
	return BBolt_FindWGServerConfigByID(objectIDToString(id))
}

func DB_FindWGServerConfigByAPIKey(apiKey string) (*types.WGServerConfig, error) {
	return BBolt_FindWGServerConfigByAPIKey(apiKey)
}

func DB_UpdateWGServerConfig(cfg *types.WGServerConfig) error {
	return BBolt_UpdateWGServerConfig(cfg)
}

func DB_CountNetworks() (int64, error) {
	return BBolt_CountNetworks()
}

func DB_CreateNetworksBatch(networks []*Network) error {
	return BBolt_CreateNetworksBatch(networks)
}

func DB_GetNetworks(limit, offset int64) ([]*Network, error) {
	return BBolt_GetNetworks(limit, offset)
}

func DB_FindNetworkByID(id primitive.ObjectID) (*Network, error) {
	return BBolt_FindNetworkByID(id)
}

func DB_UpdateNetwork(n *Network) error {
	return BBolt_UpdateNetwork(n)
}

func DB_ListWGServerConfigs() ([]*types.WGServerConfig, error) {
	return BBolt_ListWGServerConfigs()
}

func DB_SetServerWGConfigID(serverID primitive.ObjectID, wgCfg *types.WGServerConfig, pubKey, subnet string) error {
	return BBolt_SetServerWGConfigID(objectIDToString(serverID), wgCfg, pubKey, subnet)
}
