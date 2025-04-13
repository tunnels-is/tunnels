package main

import (
	"io"
	"sync"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/types"
	"github.com/zveinn/crypt"
)

const (
	ping byte = 0
	// ok   byte = 1
	// fail byte = 2
)

type ErrorResponse struct {
	Error string `json:"Error"`
}
type PortRange struct {
	StartPort uint16
	EndPort   uint16
	Client    *UserCoreMapping
}
type UserCoreMapping struct {
	ID                 string
	DeviceToken        string
	Version            int
	PortRange          *PortRange
	LastPingFromClient time.Time
	EH                 *crypt.SocketWrapper
	Uindex             []byte
	Created            time.Time

	ToUser   chan []byte
	FromUser chan Packet

	Addr syscall.Sockaddr

	// VPL
	APIToken        string
	Allowedm        sync.Mutex
	AllowedHosts    []*AllowedHost
	DHCP            *types.DHCPRecord
	DisableFirewall bool

	// IOT Client Only
	CPU  byte
	RAM  byte
	Disk byte
}

type Packet struct {
	addr syscall.Sockaddr
	data []byte
}
type RawSocket struct {
	Name          string
	IPv4Address   string
	IPv6Address   string
	InterfaceName string
	SocketBuffer  []byte

	Domain int
	Type   int
	Proto  int

	RWC io.ReadWriteCloser
}

type AllowedHost struct {
	IP   [4]byte
	PORT [2]byte
	Type string
	FFIN bool
	TFIN bool
}

func (u *UserCoreMapping) IsHostAllowed(host [4]byte, port [2]byte) *AllowedHost {
	for i, v := range u.AllowedHosts {
		if v.IP == host {
			if v.Type == "manual" {
				return u.AllowedHosts[i]
			} else if v.PORT == port {
				return u.AllowedHosts[i]
			}
		}
	}
	return nil
}

func (u *UserCoreMapping) SetFin(host [4]byte, port [2]byte, fromUser bool) {
	for i := range u.AllowedHosts {
		if u.AllowedHosts[i].IP == host {
			if u.AllowedHosts[i].PORT == port {
				if fromUser {
					u.AllowedHosts[i].FFIN = true
				} else {
					u.AllowedHosts[i].TFIN = true
				}
			}
			break
		}
	}
	return
}

func (u *UserCoreMapping) AddHost(host [4]byte, port [2]byte, t string) {
	found := false
	for i := range u.AllowedHosts {
		if u.AllowedHosts[i].IP == host {
			if u.AllowedHosts[i].Type == "manual" {
				found = true
			} else if u.AllowedHosts[i].PORT == port {
				found = true
			}
			break
		}
	}

	if !found {
		u.Allowedm.Lock()
		u.AllowedHosts = append(u.AllowedHosts,
			&AllowedHost{
				IP:   host,
				PORT: port,
				Type: t,
			})
		u.Allowedm.Unlock()
	}
}

func (u *UserCoreMapping) DelHost(host [4]byte, t string) {
	u.Allowedm.Lock()
	defer u.Allowedm.Unlock()
	for i := range u.AllowedHosts {
		if u.AllowedHosts[i].IP == host && u.AllowedHosts[i].Type == t {
			if len(u.AllowedHosts) < 2 {
				u.AllowedHosts = make([]*AllowedHost, 0)
				break
			} else {
				u.AllowedHosts[i] = u.AllowedHosts[len(u.AllowedHosts)-1]
				u.AllowedHosts = u.AllowedHosts[:len(u.AllowedHosts)-1]
				break
			}
		}
	}
}

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
