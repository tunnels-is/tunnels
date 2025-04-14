package main

import "time"

type User struct {
	UUID         string `json:"uuid"`
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
	GoogleID     string `json:"googleId"`
	IsAdmin      bool   `json:"isAdmin"`
	IsManager    bool   `json:"isManager"`
	OTPSecret    string `json:"otpSecret"`
	OTPEnabled   bool   `json:"otpEnabled"`
}

type Group struct {
	UUID        string   `json:"uuid"`
	Name        string   `json:"name"`
	UserUUIDs   []string `json:"userUuids"`
	ServerUUIDs []string `json:"serverUuids"`
}

type Server struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ipAddress"`
}

type AuthToken struct {
	UserUUID   string    `json:"userUuid"`
	TokenUUID  string    `json:"tokenUuid"`
	CreatedAt  time.Time `json:"createdAt"`
	DeviceName string    `json:"deviceName"`
}

// Request bodies
type CreateUserRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	IsAdmin   bool   `json:"isAdmin"`
	IsManager bool   `json:"isManager"`
}

type UpdateUserRequest struct {
	Username  *string `json:"username"`
	IsAdmin   *bool   `json:"isAdmin"`
	IsManager *bool   `json:"isManager"`
}

type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	DeviceName string `json:"deviceName"`
}

type CreateGroupRequest struct {
	Name string `json:"name"`
}

type UpdateGroupRequest struct {
	Name *string `json:"name,omitempty"`
}

type CreateServerRequest struct {
	Name      string `json:"name"`
	Hostname  string `json:"hostname"`
	IPAddress string `json:"ipAddress"`
}

type UpdateServerRequest struct {
	Name      *string `json:"name,omitempty"`
	Hostname  *string `json:"hostname,omitempty"`
	IPAddress *string `json:"ipAddress,omitempty"`
}

type GoogleCallbackState struct {
	DeviceName string `json:"deviceName"`
	Redirect   string `json:"redirect"` // Optional redirect after successful auth
}

type OTPSetupResponse struct {
	ProvisioningUrl string `json:"provisioningUrl"` // URL for manual entry
}

type OTPVerifyRequest struct {
	OTPCode string `json:"otpCode"`
}

type PendingOTPInfo struct {
	UserUUID    string `json:"userUuid"`
	OTPRequired bool   `json:"otpRequired"`
}
