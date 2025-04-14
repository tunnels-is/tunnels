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
)

type ErrorResponse struct {
	Error string
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

	APIToken        string
	Allowedm        sync.Mutex
	AllowedHosts    []*AllowedHost
	DHCP            *types.DHCPRecord
	DisableFirewall bool

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

var userMutex = sync.Mutex{}

type User struct {
	UUID         string
	Username     string
	PasswordHash string
	GoogleID     string
	IsAdmin      bool
	IsManager    bool
	OTPSecret    string
	OTPEnabled   bool
	Trial        bool
	SubExpires   time.Time
	Disaled      bool
	APIKey       string
}

var groupMutex = sync.Mutex{}

type Group struct {
	UUID        string
	Name        string
	UserUUIDs   []string
	ServerUUIDs []string
}

var serverMutex = sync.Mutex{}

type Server struct {
	UUID        string
	Tag         string
	IPAddress   string
	PubKey      []byte
	DataPort    uint16
	ControlPort uint16
}

type AuthToken struct {
	UserUUID   string
	TokenUUID  string
	CreatedAt  time.Time
	DeviceName string
}

type CreateUserRequest struct {
	Username       string
	Password       string
	SecondPassword string
	IsAdmin        bool
	IsManager      bool
}

type UpdateUserRequest struct {
	Disaled bool
	APIKey  string
}

type LoginRequest struct {
	Username   string
	Password   string
	DeviceName string
}

type CreateGroupRequest struct {
	Name string
}

type UpdateGroupRequest struct {
	Name string
}

type GoogleCallbackState struct {
	DeviceName string
	Redirect   string
}

type OTPSetupResponse struct {
	ProvisioningUrl string
}

type OTPVerifyRequest struct {
	OTPCode string
}

type PendingOTPInfo struct {
	UserUUID    string
	OTPRequired bool
}
