package main

import (
	"strings"
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

type userEnableQuery struct {
	Email string
	Code  string
	OrgID uuid.UUID
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

type twoFactorCreateRequest struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
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

type userSubUpdateRequest struct {
	Email       string
	DeviceToken string
	Disable     bool
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

type LicenseKey struct {
	Created time.Time
	Months  int
	Key     string
}

type User struct {
	ID uuid.UUID `json:"_id"`

	Email    string    `json:"Email"`
	Updated  time.Time `json:"Updated"`
	Disabled bool      `json:"Disabled"`

	DeviceToken *DeviceToken `json:"DeviceToken,omitempty"`
	APIKey      string       `json:"APIKey"`

	Password         string         `json:"Password"`
	Password2        string         `json:"-"`
	ConfirmCode      string         `json:"ConfirmCode"`
	LastResetRequest time.Time      `json:"-"`
	RecoveryCodes    []byte         `json:"RecoveryCodes"`
	TwoFactorCode    []byte         `json:"TwoFactorCode"`
	TwoFactorEnabled bool           `json:"TwoFactorEnabled"`
	Tokens           []*DeviceToken `json:"Tokens"`

	IsAdmin bool        `json:"IsAdmin"`
	Groups  []uuid.UUID `json:"Groups"`

	Trial         bool        `json:"Trial"`
	Key           *LicenseKey `json:"Key"`
	SubExpiration time.Time   `json:"SubExpiration"`
}

func (u *User) ToMinifiedUser() MinifiedUser {
	return MinifiedUser{
		ID:       u.ID.String(),
		Email:    u.Email,
		Disabled: u.Disabled,
		IsAdmin:  u.IsAdmin,
	}
}

type MinifiedUser struct {
	ID        string `json:"_id,omitempty"`
	Email     string `json:"Email"`
	Disabled  bool   `json:"Disabled"`
	IsAdmin   bool   `json:"IsAdmin"`
	IsManager bool   `json:"IsManager"`
}

func (u *User) RemoveSensitiveInformation() {
	if u.Key != nil && u.Key.Key != "" {
		ks := strings.Split(u.Key.Key, "-")
		if len(ks) < 2 {
			u.Key.Key = redactKey(u.Key.Key)
		} else {
			u.Key.Key = ks[len(ks)-1]
		}
	}

	u.Password = ""
	u.Password2 = ""
	u.ConfirmCode = ""
	u.RecoveryCodes = nil
	u.TwoFactorCode = nil
	u.APIKey = ""

	// Session secrets: keep only the current DeviceToken.DT (needed by the
	// client for subsequent auth). Strip every other Tokens[].DT so login and
	// admin list/get responses cannot impersonate other devices.
	currentDT := ""
	if u.DeviceToken != nil {
		currentDT = u.DeviceToken.DT
	}
	for _, t := range u.Tokens {
		if t == nil {
			continue
		}
		if currentDT == "" || t.DT != currentDT {
			t.DT = ""
		}
	}
}

type DeviceToken struct {
	DT      string    `json:"DT"`
	N       string    `json:"N"`
	Created time.Time `json:"C"`
}

type Group struct {
	ID          uuid.UUID `json:"_id"`
	Tag         string    `json:"Tag"`
	Description string    `json:"Description"`
	CreatedAt   time.Time `json:"CreatedAt"`
}
