package client

import (
	"time"
)

type ErrorResponse struct {
	Error string `json:"Error"`
}

type ForwardRequest struct {
	Server *ControlServer

	Path     string
	Method   string
	Timeout  int
	JSONData any
	Headers  map[string]string
}

type CreateDeviceWithKeysForm struct {
	Server      *ControlServer
	Tag         string
	ServerID    string
	DeviceToken string
	UID         string
}

type TwoFactorConfirm struct {
	Email  string
	Code   string
	Digits string
}

type QRCode struct {
	Value string
}

type DeviceToken struct {
	DT      string    `json:"DT"`
	N       string    `json:"N"`
	Created time.Time `json:"C"`
}

type DelUserForm struct {
	Hash string
}

type User struct {
	ID                    string         `json:"_id,omitempty"`
	APIKey                string         `json:"APIKey"`
	Email                 string         `json:"Email"`
	DeviceToken           *DeviceToken   `json:",omitempty"`
	Tokens                []*DeviceToken `json:"Tokens"`
	OrgID                 string         `json:"OrgID" `
	Key                   *LicenseKey    `json:"Key"`
	Trial                 bool           `json:"Trial"`
	Disabled              bool           `json:"Disabled"`
	TwoFactorEnabled      bool           `json:"TwoFactorEnabled"`
	Updated               time.Time      `json:"Updated"`
	SubExpiration         time.Time      `json:"SubExpiration"`
	AdditionalInformation string         `json:"AdditionalInformation,omitempty"`
	IsAdmin               bool           `json:"IsAdmin"`
	IsManager             bool           `json:"IsManager"`

	ControlServer *ControlServer
	SaveFileHash  string
}

type LicenseKey struct {
	Created time.Time
	Months  int
	Key     string
}
