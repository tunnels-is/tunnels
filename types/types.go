package types

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/crypt"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Feature string

const (
	VPN   Feature = "VPN"
	LAN   Feature = "LAN"
	AUTH  Feature = "AUTH"
	DNS   Feature = "DNS"
	BBOLT Feature = "BBOLT"
)

type ServerConfig struct {
	Features           []Feature `json:"Features"`
	PingTimeoutMinutes int       `json:"PingTimeoutMinutes"`
	DHCPTimeoutHours   int       `json:"DHCPTimeoutHours"`

	VPNIP   string `json:"VPNIP"`
	VPNPort string `json:"VPNPort"`

	APIIP   string `json:"APIIP"`
	APIPort string `json:"APIPort"`

	NetAdmins []string `json:"NetAdmins"`

	Hostname           string
	Lan                *Network   `json:"Lan"`
	DisableLanFirewall bool       `json:"DisableLanFirwall"`
	Routes             []*Route   `json:"Routes"`
	SubNets            []*Network `json:"SubNets"`

	StartPort          int  `json:"StartPort"`
	EndPort            int  `json:"EndPort"`
	UserMaxConnections int  `json:"UserMaxConnections"`
	InternetAccess     bool `json:"InternetAccess"`
	LocalNetworkAccess bool `json:"LocalNetworkAccess"`
	BandwidthMbps      int  `json:"BandwidthMbps"`
	UserBandwidthMbps  int  `json:"BandwidthUserMbps"`

	DNSAllowCustomOnly bool         `json:"DNSAllowCustomOnly"`
	DNSRecords         []*DNSRecord `json:"DNSRecords"`
	DNSServers         []string     `json:"DNSServers"`

	SecretStore SecretStore `json:"SecretStore"`
	// If SecretStore set to "config"
	AdminApiKey  string
	DBurl        string
	TwoFactorKey string
	EmailKey     string
	PayKey       string
	CertPem      string
	SignPem      string
	KeyPem       string
}

type SecretStore string

const (
	EnvStore    SecretStore = "env"
	ConfigStore SecretStore = "config"
)

type Device struct {
	ID        primitive.ObjectID   `json:"_id" bson:"_id"`
	CreatedAt time.Time            `json:"CreatedAt" bson:"CreatedAt"`
	Tag       string               `json:"Tag" bson:"Tag"`
	Groups    []primitive.ObjectID `json:"Groups" bson:"Groups"`
}

type FORM_GET_SERVER struct {
	DeviceToken string             `json:"DeviceToken"`
	DeviceKey   string             `json:"DeviceKey"`
	UID         primitive.ObjectID `json:"UID"`
	ServerID    primitive.ObjectID `json:"ServerID"`
}

type Server struct {
	ID       primitive.ObjectID   `json:"_id" bson:"_id"`
	Tag      string               `json:"Tag" bson:"Tag"`
	Country  string               `json:"Country" bson:"Country"`
	IP       string               `json:"IP" bson:"IP"`
	Port     string               `json:"Port" bson:"Port"`
	DataPort string               `json:"DataPort" bson:"DataPort"`
	PubKey   string               `json:"PubKey,omitempty" bson:"PubKey"`
	Groups   []primitive.ObjectID `json:"Groups,omitempty" bson:"Groups"`
}

type TwoFAPending struct {
	AuthID  string
	UserID  string
	Expires time.Time
	Code    string
}

type Route struct {
	Address string
	Metric  string
	Gateway string
}

type Network struct {
	Tag     string `json:"Tag" bson:"Tag"`
	Network string `json:"Network" bson:"Network"`
	Nat     string `json:"Nat" bson:"Nat"`

	NetIPNet *net.IPNet `json:"-"`
	NatIPNet *net.IPNet `json:"-"`
}

type DNSRecord struct {
	Domain   string   `json:"Domain" bson:"Domain"`
	Wildcard bool     `json:"Wildcard" bson:"Wildcard"`
	IP       []string `json:"IP" bson:"IP"`
	TXT      []string `json:"TXT" bson:"TXT"`
}

type DeviceListResponse struct {
	Devices      []*ListDevice
	DHCPAssigned int
	DHCPFree     int
}

type ListDevice struct {
	DHCP         DHCPRecord
	AllowedIPs   []string
	CPU          byte
	RAM          byte
	Disk         byte
	IngressQueue int
	EgressQueue  int
	Created      time.Time
	StartPort    uint16
	EndPort      uint16
}

type SignedConnectRequest struct {
	Signature      []byte
	Payload        []byte
	X25519PeerPub  []byte
	Mlkem1024Encap []byte
}

type ServerConnectResponse struct {
	X25519Pub       []byte
	Mlkem1024Cipher []byte
	// ServerHandshake          []byte
	ServerHandshakeSignature []byte
	Index                    int `json:"Index"`
	AvailableMbps            int `json:"AvailableMbps"`
	AvailableUserMbps        int `json:"AvailableUserMbps"`

	InternetAccess     bool `json:"InternetAccess"`
	LocalNetworkAccess bool `json:"LocalNetworkAccess"`

	InterfaceIP string `json:"InterfaceIP"`
	DataPort    string `json:"DataPort"`
	StartPort   uint16 `json:"StartPort"`
	EndPort     uint16 `json:"EndPort"`

	DNSRecords         []*DNSRecord `json:"DNSRecords"`
	Networks           []*Network   `json:"Networks"`
	Routes             []*Route     `json:"Routes"`
	DNSServers         []string     `json:"DNSServers"`
	DNSAllowCustomOnly bool         `json:"DNSAllowCustomOnly"`

	DHCP *DHCPRecord `json:"DHCP"`
	LAN  *Network    `json:"LANNetwork"`
}

func CreateCRRFromServer(S *ServerConfig) (CRR *ServerConnectResponse) {
	return &ServerConnectResponse{
		Index:              0,
		StartPort:          0,
		EndPort:            0,
		InterfaceIP:        S.VPNIP,
		DataPort:           S.VPNPort,
		AvailableMbps:      S.BandwidthMbps,
		AvailableUserMbps:  S.UserBandwidthMbps,
		InternetAccess:     S.InternetAccess,
		LocalNetworkAccess: S.LocalNetworkAccess,
		DNSRecords:         S.DNSRecords,
		Networks:           S.SubNets,
		Routes:             S.Routes,
		DNSServers:         S.DNSServers,
		DNSAllowCustomOnly: S.DNSAllowCustomOnly,
		LAN:                S.Lan,
	}
}

type ControllerConnectRequest struct {
	DeviceKey   string             `json:"DeviceKey"`
	DeviceToken string             `json:"DeviceToken"`
	UserID      primitive.ObjectID `json:"UserID"`

	// General
	EncType  crypt.EncType      `json:"EncType"`
	ServerID primitive.ObjectID `json:"ServerID"`

	// These are added by the golang client
	Version int       `json:"Version"`
	Created time.Time `json:"Created"`

	RequestingPorts bool `json:"RequestingPorts"`
}

type DHCPRecord struct {
	m  sync.Mutex `json:"-"`
	IP [4]byte

	Hostname string
	Token    string
	Activity time.Time
}
type FirewallRequest struct {
	DHCPToken       string
	IP              string
	Hosts           []string
	DisableFirewall bool
}

func (d *DHCPRecord) AssignHostname(defaultHostname string) {
	d.m.Lock()
	defer d.m.Unlock()
	d.Activity = time.Now()
	host := fmt.Sprintf("%d-%d-%d-%d",
		d.IP[0],
		d.IP[1],
		d.IP[2],
		d.IP[3],
	)

	if defaultHostname != "" {
		d.Hostname = host + "." + defaultHostname
	} else {
		d.Hostname = host
	}
}

func (d *DHCPRecord) Assign(timeoutHours float64, token string) (ok bool) {
	if !d.Activity.IsZero() {
		if time.Since(d.Activity).Hours() < timeoutHours {
			return
		}
	}
	d.m.Lock()
	defer d.m.Unlock()
	if d.Token == "" {
		d.Token = token
		d.Activity = time.Now()
		ok = true
		return
	}
	return
}

type FORM_GET_DEVICE struct {
	DeviceID primitive.ObjectID
}
