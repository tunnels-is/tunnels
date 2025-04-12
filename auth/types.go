package main

import "time"

type User struct {
	UUID         string `json:"uuid"`
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
	GoogleID     string `json:"googleId,omitempty"` // Changed: Allow marshalling, keep omitempty
	IsAdmin      bool   `json:"isAdmin"`
	IsManager    bool   `json:"isManager"`
	OTPSecret    string `json:"otpSecret,omitempty"` // Changed: Allow marshalling, keep omitempty
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
	TokenUUID  string    `json:"tokenUuid"` // This is the actual token the client sends
	CreatedAt  time.Time `json:"createdAt"`
	DeviceName string    `json:"deviceName"` // e.g., "Chrome on Desktop", "User's iPhone"
}

// Request bodies
type CreateUserRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password,omitempty"`
	IsAdmin   bool   `json:"isAdmin"`   // Should only be settable by existing admin
	IsManager bool   `json:"isManager"` // Should only be settable by existing admin/manager
}

type UpdateUserRequest struct {
	Username  *string `json:"username,omitempty"`
	IsAdmin   *bool   `json:"isAdmin,omitempty"`   // Admin only
	IsManager *bool   `json:"isManager,omitempty"` // Admin or Manager only
}

type LoginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	DeviceName string `json:"deviceName,omitempty"` // Optional device name
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
