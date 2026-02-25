package main

import (
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	xsync "github.com/puzpuzpuz/xsync/v3"
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/signal"
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
	PingInt            atomic.Int64
	DeviceToken        string
	Version            int
	PortRange          *PortRange
	LastPingFromClient time.Time
	EH                 *crypt.SocketWrapper
	Uindex             []byte
	Created            time.Time

	ToUser     chan []byte
	ToSignal   *signal.Signal
	FromUser   chan Packet
	FromSignal *signal.Signal

	Addr syscall.Sockaddr

	APIToken        string
	hostsInit       sync.Once
	AutoHosts       *xsync.MapOf[[6]byte, *AllowedHost]
	ManualHosts     *xsync.MapOf[[4]byte, *AllowedHost]
	DHCP            *types.DHCPRecord
	DisableFirewall bool

	CPU  byte
	RAM  byte
	Disk byte

	Delete sync.Once
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
	FFIN atomic.Bool
	TFIN atomic.Bool
}

func (u *UserCoreMapping) initHosts() {
	u.hostsInit.Do(func() {
		u.AutoHosts = xsync.NewMapOf[[6]byte, *AllowedHost]()
		u.ManualHosts = xsync.NewMapOf[[4]byte, *AllowedHost]()
	})
}

func (u *UserCoreMapping) IsHostAllowed(host [4]byte, port [2]byte) *AllowedHost {
	u.initHosts()
	if v, ok := u.ManualHosts.Load(host); ok {
		return v
	}
	key := [6]byte{host[0], host[1], host[2], host[3], port[0], port[1]}
	if v, ok := u.AutoHosts.Load(key); ok {
		return v
	}
	return nil
}

func (u *UserCoreMapping) SetFin(host [4]byte, port [2]byte, fromUser bool) {
	u.initHosts()
	key := [6]byte{host[0], host[1], host[2], host[3], port[0], port[1]}
	if v, ok := u.AutoHosts.Load(key); ok {
		if fromUser {
			v.FFIN.Store(true)
		} else {
			v.TFIN.Store(true)
		}
	}
}

func (u *UserCoreMapping) AddHost(host [4]byte, port [2]byte, t string) {
	u.initHosts()
	if t == "manual" {
		u.ManualHosts.LoadOrStore(host, &AllowedHost{IP: host, PORT: port, Type: "manual"})
		return
	}
	// Skip if a manual entry already covers this IP (manual matches any port)
	if _, ok := u.ManualHosts.Load(host); ok {
		return
	}
	key := [6]byte{host[0], host[1], host[2], host[3], port[0], port[1]}
	u.AutoHosts.LoadOrStore(key, &AllowedHost{IP: host, PORT: port, Type: "auto"})
}

func (u *UserCoreMapping) ClearHost(host [4]byte) {
	u.initHosts()
	u.AutoHosts.Range(func(key [6]byte, _ *AllowedHost) bool {
		if [4]byte{key[0], key[1], key[2], key[3]} == host {
			u.AutoHosts.Delete(key)
		}
		return true
	})
	u.ManualHosts.Delete(host)
}

func (u *UserCoreMapping) DelHost(host [4]byte, port [2]byte, t string) {
	u.initHosts()
	if t == "manual" {
		u.ManualHosts.Delete(host)
		return
	}
	key := [6]byte{host[0], host[1], host[2], host[3], port[0], port[1]}
	u.AutoHosts.Delete(key)
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
	UID         primitive.ObjectID `json:"UID"`
	DeviceToken string             `json:"DeviceToken"`
	Key         string
}

type REGISTER_FORM struct {
	Email                 string
	Password              string
	Password2             string
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

type FORM_GET_GROUP_ENTITIES struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	GID         primitive.ObjectID `json:"GID"`
	Type        string             `json:"Type"`
	Limit       int                `json:"Limit"`
	Offset      int                `json:"Offset"`
}

type FORM_DELETE_GROUP struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	GID         primitive.ObjectID `json:"GID"`
}

type FORM_DELETE_DEVICE struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	DID         primitive.ObjectID `json:"DID"`
}

type FORM_LIST_GROUP struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
}

type FORM_LIST_USERS struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Limit       int                `json:"Limit"`
	Offset      int                `json:"Offset"`
}

type FORM_CONNECTED_DEVICES struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
}

type FORM_LIST_DEVICE struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Limit       int                `json:"Limit"`
	Offset      int                `json:"Offset"`
}

type FORM_CREATE_GROUP struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Group       *Group             `json:"Group"`
}

type FORM_CREATE_DEVICE struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Device      *types.Device      `json:"Device"`
}

type FORM_UPDATE_SERVER struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Server      *types.Server      `json:"Server"`
}

type FORM_CREATE_SERVER struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Server      *types.Server      `json:"Server"`
}

type FORM_UPDATE_GROUP struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Group       *Group             `json:"Group"`
}

type FORM_UPDATE_DEVICE struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Device      *types.Device      `json:"Device"`
}

type FORM_GROUP_ADD struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	GroupID     primitive.ObjectID `json:"GroupID"`
	Type        string             `json:"Type"`
	TypeID      primitive.ObjectID `json:"TypeID"`
	TypeTag     string             `json:"TypeTag"`
}

type FORM_GROUP_REMOVE struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	GroupID     primitive.ObjectID `json:"GroupID"`
	Type        string             `json:"Type"`
	TypeID      primitive.ObjectID `json:"TypeID"`
}

type TWO_FACTOR_CREATE struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
}

type TWO_FACTOR_FORM struct {
	DeviceToken string             `json:"DeviceToken"`
	UID         primitive.ObjectID `json:"UID"`
	Code        string
	Digits      string
	Password    string
	Recovery    string
}

type USER_UPDATE_FORM struct {
	UID                   primitive.ObjectID
	DeviceToken           string
	APIKey                string
	AdditionalInformation string
}

type USER_ADMIN_UPDATE_FORM struct {
	DeviceToken   string             `json:"DeviceToken"`
	UID           primitive.ObjectID `json:"UID"`
	TargetUserID  primitive.ObjectID `json:"TargetUserID"`
	Email         string             `json:"Email,omitempty"`
	Disabled      bool               `json:"Disabled"`
	IsManager     bool               `json:"IsManager"`
	Trial         bool               `json:"Trial"`
	SubExpiration time.Time          `json:"SubExpiration,omitempty"`
}

type TWO_FACTOR_DB_PACKAGE struct {
	UID      primitive.ObjectID
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
	UID         primitive.ObjectID
	DeviceToken string
	LogoutToken string
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

func (u *User) ToMinifiedUser() MinifiedUser {
	return MinifiedUser{
		ID:        u.ID.Hex(),
		Email:     u.Email,
		Disabled:  u.Disabled,
		IsAdmin:   u.IsAdmin,
		IsManager: u.IsManager,
	}
}

type MinifiedUser struct {
	ID        string `json:"_id,omitempty"`
	Email     string `json:"Email"`
	Disabled  bool   `json:"Disabled"`
	IsAdmin   bool   `json:"IsAdmin" bson:"IsAdmin"`
	IsManager bool   `json:"IsManager" bson:"IsManager"`
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
}

type DeviceToken struct {
	DT      string    `bson:"DT"`
	N       string    `bson:"N"`
	Created time.Time `bson:"C"`
}

type Group struct {
	ID          primitive.ObjectID `json:"_id" bson:"_id"`
	Tag         string             `json:"Tag" bson:"Tag"`
	Description string             `json:"Description" bson:"Description"`
	CreatedAt   time.Time          `json:"CreatedAt" bson:"CreatedAt"`
}
