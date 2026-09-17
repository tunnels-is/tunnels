package main

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

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
