package main

import (
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

type ErrorResponse struct {
	Error string
}

type getServerRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	DeviceKey   string    `json:"DeviceKey"`
	UID         uuid.UUID `json:"UID"`
	ServerID    uuid.UUID `json:"ServerID"`
}

type getDeviceRequest struct {
	DeviceID uuid.UUID
}

type licenseActivateRequest struct {
	UID         uuid.UUID `json:"UID"`
	DeviceToken string    `json:"DeviceToken"`
	Key         string
}

type registerRequest struct {
	Email                 string
	Password              string
	Password2             string
	AdditionalInformation string
}

type getGroupRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GID         uuid.UUID `json:"GID"`
}

type getGroupEntitiesRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GID         uuid.UUID `json:"GID"`
	Type        string    `json:"Type"`
	Limit       int       `json:"Limit"`
	Offset      int       `json:"Offset"`
}

type deleteGroupRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GID         uuid.UUID `json:"GID"`
}

type deleteDeviceRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	DID         uuid.UUID `json:"DID"`
}

type deleteUserRequest struct {
	DeviceToken  string    `json:"DeviceToken"`
	UID          uuid.UUID `json:"UID"`
	TargetUserID uuid.UUID `json:"TargetUserID"`
}

type deleteServerRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	ServerID    uuid.UUID `json:"ServerID"`
}

type listGroupsRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Limit       int       `json:"Limit"`
	Offset      int       `json:"Offset"`
}

type listUsersRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Limit       int       `json:"Limit"`
	Offset      int       `json:"Offset"`
}

type userLatestResponse struct {
	Users             []*User `json:"Users"`
	Total             int64   `json:"Total"`
	Trial             int64   `json:"Trial"`
	ActiveSubscribers int64   `json:"ActiveSubscribers"`
}

type adminUserSearchRequest struct {
	Email string `json:"Email"`
}

type adminUserGetRequest struct {
	TargetUserID uuid.UUID `json:"TargetUserID"`
}

type listDevicesRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Limit       int       `json:"Limit"`
	Offset      int       `json:"Offset"`
}

type createGroupRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Group       *Group    `json:"Group"`
}

type createDeviceRequest struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         uuid.UUID     `json:"UID"`
	Device      *types.Device `json:"Device"`
}

type updateServerRequest struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         uuid.UUID     `json:"UID"`
	Server      *types.Server `json:"Server"`
}

type createServerRequest struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         uuid.UUID     `json:"UID"`
	Server      *types.Server `json:"Server"`
}

type updateGroupRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Group       *Group    `json:"Group"`
}

type updateDeviceRequest struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         uuid.UUID     `json:"UID"`
	Device      *types.Device `json:"Device"`
}

type groupAddRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GroupID     uuid.UUID `json:"GroupID"`
	Type        string    `json:"Type"`
	TypeID      uuid.UUID `json:"TypeID"`
	TypeTag     string    `json:"TypeTag"`
}

type groupRemoveRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GroupID     uuid.UUID `json:"GroupID"`
	Type        string    `json:"Type"`
	TypeID      uuid.UUID `json:"TypeID"`
}

type twoFactorRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Code        string
	Digits      string
	Password    string
	Recovery    string
}

type userUpdateRequest struct {
	UID                   uuid.UUID
	DeviceToken           string
	APIKey                string
	AdditionalInformation string
}

type adminUserUpdateRequest struct {
	DeviceToken   string    `json:"DeviceToken"`
	UID           uuid.UUID `json:"UID"`
	TargetUserID  uuid.UUID `json:"TargetUserID"`
	Email         string    `json:"Email,omitempty"`
	Disabled      bool      `json:"Disabled"`
	Trial         bool      `json:"Trial"`
	SubExpiration time.Time `json:"SubExpiration,omitempty"`
}

type twoFactorUpdate struct {
	UID      uuid.UUID
	Code     []byte
	Recovery []byte
}

type passwordResetRequest struct {
	Email        string
	Password     string
	ResetCode    string
	UseTwoFactor bool
}

type listServersRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	StartIndex  int
}

type serversByCountryRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Country     string
}

type loginRequest struct {
	Email       string
	Password    string
	DeviceName  string
	DeviceToken string
	Digits      string
	Recovery    string
	Version     string
}

type logoutRequest struct {
	UID           uuid.UUID
	DeviceToken   string
	LogoutToken   string
	LogoutName    string    `json:"LogoutName,omitempty"`
	LogoutCreated time.Time `json:"LogoutCreated,omitempty"`
	All           bool
}

type userTokensUpdate struct {
	ID      uuid.UUID      `json:"_id"`
	Tokens  []*DeviceToken `json:"Tokens"`
	Version string         `json:"version"`
}
