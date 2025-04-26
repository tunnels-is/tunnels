package main

import (
	"io"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

type USER_ENABLE_FORM struct {
	Email string
	Code  string
}

type USER_ENABLE_QUERY struct {
	Email string
	Code  string
	OrgID primitive.ObjectID
}

type KEY_ACTIVATE_FORM struct {
	UserID      primitive.ObjectID `json:"UID"`
	DeviceToken string             `json:"DeviceToken"`
	Key         string
}

type REGISTER_FORM struct {
	Email                 string
	Password              string
	Password2             string
	Code                  string
	AdditionalInformation string
}

type FORM_GET_ORG struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
}

type FORM_GET_GROUP struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	GID         primitive.ObjectID `json:"GID"`
}

type FORM_CREATE_GROUP struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Group       *Group             `json:"Group"`
}

type FORM_UPDATE_SERVER struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Server      *Server            `json:"Server"`
}

type FORM_CREATE_SERVER struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Server      *Server            `json:"Server"`
}

type FORM_UPDATE_GROUP struct {
	DeviceToken string             `json:"DeviceToken"`
	UserID      primitive.ObjectID `json:"UID"`
	Group       *Group             `json:"Group"`
}

type FORM_GROUP_ADD struct {
	DeviceToken string             `json:"DeviceToken"`
	UserID      primitive.ObjectID `json:"UserID"`
	GroupID     primitive.ObjectID `json:"GroupID"`
	Type        string             `json:"Type"`
	TypeID      primitive.ObjectID `json:"TypeID"`
}

type TWO_FACTOR_CONFIRM struct {
	Email    string
	Code     string
	Digits   string
	Password string
	Recovery string
}

type USER_UPDATE_FORM struct {
	UserID                primitive.ObjectID
	DeviceToken           string
	APIKey                string
	AdditionalInformation string
}

type TWO_FACTOR_DB_PACKAGE struct {
	UID      primitive.ObjectID
	Code     []byte
	Recovery []byte
}

type PASSWORD_RESET_FORM struct {
	Email     string
	Password  string
	Password2 string
	ResetCode string
}

type FORM_GET_SERVERS struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	StartIndex  int
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
	UserID      primitive.ObjectID
	DeviceToken string
	All         bool
}

type UPDATE_USER_TOKENS struct {
	ID      primitive.ObjectID `json:"_id" bson:"_id"`
	Tokens  []*DeviceToken     `json:"Tokens" bson:"T"`
	Version string             `json:"version" bson:"V"`
}

type LicenseKey struct {
	Created time.Time
	Months  int
	Key     string
}

type User struct {
	ID primitive.ObjectID `json:"_id" bson:"_id"`

	Email                 string    `json:"Email" bson:"Email"`
	Updated               time.Time `json:"Updated" bson:"Updated"`
	AdditionalInformation string    `json:"AdditionalInformation,omitempty" bson:"AdditionalInformation"`
	Disabled              bool      `json:"Disabled" bson:"Disabled"`

	DeviceToken *DeviceToken `json:"DeviceToken,omitempty" bson:"-"`
	APIKey      string       `json:"APIKey" bson:"APIKey"`

	// these do not get sent over the network for security reasons
	Password         string         `json:"Password" bson:"Password" `
	Password2        string         `json:"-" bson:"-"`
	ResetCode        string         `json:"ResetCode" bson:"ResetCode"`
	ConfirmCode      string         `json:"ConfirmCode" bson:"ConfirmCode"`
	LastResetRequest time.Time      `json:"-" bson:"LastResetRequest"`
	RecoveryCodes    []byte         `json:"RecoveryCodes" bson:"RecoveryCodes"`
	TwoFactorCode    []byte         `json:"TwoFactorCode" bson:"TwoFactorCode"`
	TwoFactorEnabled bool           `json:"TwoFactorEnabled" bson:"TwoFactorEnabled"`
	Tokens           []*DeviceToken `json:"Tokens" bson:"Tokens"`

	IsAdmin   bool                 `json:"IsAdmin" bson:"IsAdmin"`
	IsManager bool                 `json:"IsManager" bson:"IsManager"`
	Groups    []primitive.ObjectID `json:"Groups" bson:"Groups"`

	// tunnels public network
	Trial         bool        `json:"Trial" bson:"Trial"`
	Key           *LicenseKey `json:"Key" bson:"Key"`
	SubExpiration time.Time   `json:"SubExpiration" bson:"SubExpiration"`
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
	u.ResetCode = ""
	u.ConfirmCode = ""
	u.RecoveryCodes = nil
	u.TwoFactorCode = nil
	return
}

type DeviceToken struct {
	DT      string    `bson:"DT"`
	N       string    `bson:"N"`
	Created time.Time `bson:"C"`
}

type Server struct {
	ID       primitive.ObjectID   `json:"_id,omitempty" bson:"_id"`
	Tag      string               `json:"Tag,required" bson:"Tag"`
	Country  string               `json:"Country,required" bson:"Country"`
	IP       string               `json:"IP,required" bson:"IP"`
	Port     string               `json:"Port,required" bson:"Port"`
	DataPort string               `json:"DataPort,required" bson:"DataPort"`
	PubKey   []byte               `json:"PubKey" bson:"PubKey"`
	Groups   []primitive.ObjectID `json:"Groups" bson:"Groups"`
}

type Group struct {
	ID          primitive.ObjectID `json:"_id" bson:"_id"`
	Tag         string             `json:"Tag" bson:"Tag"`
	Description string             `json:"Description" bson:"Desciption"`
}

type Device struct {
	ID        primitive.ObjectID   `json:"_id" bson:"_id"`
	CreatedAt time.Time            `json:"Added" bson:"Added"`
	Tag       string               `json:"Tag" bson:"Tag"`
	Groups    []primitive.ObjectID `json:"Groups" bson:"Groups"`
}
