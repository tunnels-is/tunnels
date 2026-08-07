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

type USER_ENABLE_QUERY struct {
	Email string
	Code  string
	OrgID uuid.UUID
}

type KEY_ACTIVATE_FORM struct {
	UID         uuid.UUID `json:"UID"`
	DeviceToken string    `json:"DeviceToken"`
	Key         string
}

type REGISTER_FORM struct {
	Email                 string
	Password              string
	Password2             string
	AdditionalInformation string
}

type FORM_GET_GROUP struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GID         uuid.UUID `json:"GID"`
}

type FORM_GET_GROUP_ENTITIES struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GID         uuid.UUID `json:"GID"`
	Type        string    `json:"Type"`
	Limit       int       `json:"Limit"`
	Offset      int       `json:"Offset"`
}

type FORM_DELETE_GROUP struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GID         uuid.UUID `json:"GID"`
}

type FORM_DELETE_DEVICE struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	DID         uuid.UUID `json:"DID"`
}

type FORM_DELETE_USER struct {
	DeviceToken  string    `json:"DeviceToken"`
	UID          uuid.UUID `json:"UID"`
	TargetUserID uuid.UUID `json:"TargetUserID"`
}

type FORM_DELETE_SERVER struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	ServerID    uuid.UUID `json:"ServerID"`
}

type FORM_LIST_GROUP struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Limit       int       `json:"Limit"`
	Offset      int       `json:"Offset"`
}

type FORM_LIST_USERS struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Limit       int       `json:"Limit"`
	Offset      int       `json:"Offset"`
}


type USER_LATEST_RESPONSE struct {
	Users              []*User `json:"Users"`
	Total              int64   `json:"Total"`
	Trial              int64   `json:"Trial"`
	ActiveSubscribers  int64   `json:"ActiveSubscribers"`
}


type FORM_ADMIN_USER_SEARCH struct {
	Email string `json:"Email"`
}


type FORM_ADMIN_USER_GET struct {
	TargetUserID uuid.UUID `json:"TargetUserID"`
}

type FORM_LIST_DEVICE struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Limit       int       `json:"Limit"`
	Offset      int       `json:"Offset"`
}

type FORM_CREATE_GROUP struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Group       *Group    `json:"Group"`
}

type FORM_CREATE_DEVICE struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         uuid.UUID     `json:"UID"`
	Device      *types.Device `json:"Device"`
}

type FORM_UPDATE_SERVER struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         uuid.UUID     `json:"UID"`
	Server      *types.Server `json:"Server"`
}

type FORM_CREATE_SERVER struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         uuid.UUID     `json:"UID"`
	Server      *types.Server `json:"Server"`
}

type FORM_UPDATE_GROUP struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Group       *Group    `json:"Group"`
}

type FORM_UPDATE_DEVICE struct {
	DeviceToken string        `json:"DeviceToken"`
	UID         uuid.UUID     `json:"UID"`
	Device      *types.Device `json:"Device"`
}

type FORM_GROUP_ADD struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GroupID     uuid.UUID `json:"GroupID"`
	Type        string    `json:"Type"`
	TypeID      uuid.UUID `json:"TypeID"`
	TypeTag     string    `json:"TypeTag"`
}

type FORM_GROUP_REMOVE struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	GroupID     uuid.UUID `json:"GroupID"`
	Type        string    `json:"Type"`
	TypeID      uuid.UUID `json:"TypeID"`
}

type TWO_FACTOR_CREATE struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
}

type TWO_FACTOR_FORM struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Code        string
	Digits      string
	Password    string
	Recovery    string
}

type USER_UPDATE_FORM struct {
	UID                   uuid.UUID
	DeviceToken           string
	APIKey                string
	AdditionalInformation string
}

type USER_ADMIN_UPDATE_FORM struct {
	DeviceToken   string    `json:"DeviceToken"`
	UID           uuid.UUID `json:"UID"`
	TargetUserID  uuid.UUID `json:"TargetUserID"`
	Email         string    `json:"Email,omitempty"`
	Disabled      bool      `json:"Disabled"`
	Trial         bool      `json:"Trial"`
	SubExpiration time.Time `json:"SubExpiration,omitempty"`
}

type TWO_FACTOR_DB_PACKAGE struct {
	UID      uuid.UUID
	Code     []byte
	Recovery []byte
}

type PASSWORD_RESET_FORM struct {
	Email        string
	Password     string
	ResetCode    string
	UseTwoFactor bool
}

type FORM_GET_SERVERS struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	StartIndex  int
}

type FORM_GET_SERVERS_BY_COUNTRY struct {
	DeviceToken string    `json:"DeviceToken"`
	UID         uuid.UUID `json:"UID"`
	Country     string
}

type USER_UPDATE_SUB_FORM struct {
	Email       string
	DeviceToken string
	Disable     bool
}

type LOGIN_FORM struct {
	Email       string
	Password    string
	DeviceName  string
	DeviceToken string
	Digits      string
	Recovery    string
	Version     string
}

type LOGOUT_FORM struct {
	UID           uuid.UUID
	DeviceToken   string
	LogoutToken   string
	LogoutName    string    `json:"LogoutName,omitempty"`
	LogoutCreated time.Time `json:"LogoutCreated,omitempty"`
	All           bool
}

type UPDATE_USER_TOKENS struct {
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
	if u.Key != nil {
		ks := strings.Split(u.Key.Key, "-")
		if len(ks) < 1 {
			u.Key.Key = "redacted"
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
