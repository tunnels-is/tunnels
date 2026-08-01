package types

import (
	"net"
	"time"

	"github.com/google/uuid"
)

type TunnelType string

const (
	DefaultTun TunnelType = "default"
	IoTTun     TunnelType = "iot"
)

var Uptime = time.Now()

type HealthResponse struct {
	ServerVersion string
	ClientVersion string
	Uptime        time.Time
}

type ServerConfig struct {
	LogAPIHosts bool

	ClientVersion string

	APIIP   string
	APIPort string

	AllowedOrigins []string

	AdminAPIKey      string
	DBurl            string
	TwoFactorKey     string
	CookieSigningKey string
	PayKey           string
	CertPem          string
	KeyPem           string

	CertPems []string
	KeyPems  []string
}

type WGBootstrap struct {
	APIKey             string
	ControllerURL      string
	InsecureSkipVerify bool
}

type Device struct {
	ID        uuid.UUID `json:"_id"`
	CreatedAt time.Time `json:"CreatedAt"`
	Tag       string    `json:"Tag"`

	UserID uuid.UUID `json:"UserID,omitempty"`

	ServerID uuid.UUID `json:"ServerID,omitempty"`

	WireGuardKey string `json:"WireGuardKey,omitempty"`

	WireGuardIP string `json:"WireGuardIP,omitempty"`

	WireGuardIPv6 string `json:"WireGuardIPv6,omitempty"`
}

type FORM_GET_SERVER struct {
	DeviceToken string    `json:"DeviceToken"`
	DeviceKey   string    `json:"DeviceKey"`
	UID         uuid.UUID `json:"UID"`
	ServerID    uuid.UUID `json:"ServerID"`
}

type WAN struct {
	ID          uuid.UUID `json:"ID"`
	Tag         string    `json:"Tag"`
	CIDR        string    `json:"CIDR"`
	Description string    `json:"Description,omitempty"`
}

type Server struct {
	ID       uuid.UUID   `json:"_id"`
	Tag      string      `json:"Tag"`
	InfraTag string      `json:"InfraTag,omitempty"`
	Country  string      `json:"Country"`
	IP       string      `json:"IP"`
	Port     string      `json:"Port"`
	Groups   []uuid.UUID `json:"Groups,omitempty"`

	WANID string `json:"WANID,omitempty"`

	WAN *WAN `json:"WAN,omitempty"`

	MeshGroupID string `json:"MeshGroupID,omitempty"`

	APIKey string `json:"APIKey,omitempty"`

	WireGuardPort   int    `json:"WireGuardPort,omitempty"`
	WireGuardPubKey string `json:"WireGuardPubKey,omitempty"`
	WireGuardIface  string `json:"WireGuardIface,omitempty"`

	WireGuardMeshPort int `json:"WireGuardMeshPort,omitempty"`

	WireGuardSubnet string `json:"WireGuardSubnet,omitempty"`

	WireGuardSubnet6 string `json:"WireGuardSubnet6,omitempty"`

	InternetIface string `json:"InternetIface,omitempty"`

	EnableFirewall bool `json:"EnableFirewall"`

	InsecureSkipVerify bool `json:"InsecureSkipVerify,omitempty"`
}

type WGServerConfigResponse struct {
	ServerID string `json:"ServerID"`

	ServerIP string `json:"ServerIP,omitempty"`

	WireGuardPort     int    `json:"WireGuardPort"`
	WireGuardMeshPort int    `json:"WireGuardMeshPort,omitempty"`
	WireGuardSubnet   string `json:"WireGuardSubnet"`
	WireGuardSubnet6  string `json:"WireGuardSubnet6,omitempty"`
	WireGuardIface    string `json:"WireGuardIface"`

	InternetIface string `json:"InternetIface"`

	EnableFirewall     bool `json:"EnableFirewall"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify"`
}

type MeshGroup struct {
	ID          uuid.UUID `json:"_id"`
	Tag         string    `json:"Tag"`
	Description string    `json:"Description,omitempty"`
	CreatedAt   time.Time `json:"CreatedAt"`
}

type WGMeshPeer struct {
	PublicKeyHex   string   `json:"PublicKeyHex"`
	Endpoint       string   `json:"Endpoint"`
	AllowedSubnets []string `json:"AllowedSubnets"`
}

type WGMeshResponse struct {
	Peers []WGMeshPeer `json:"Peers"`
}

type Route struct {
	Address string
	Metric  string
	Gateway string
}

type Network struct {
	Tag     string `json:"Tag"`
	Network string `json:"Network"`
	Nat     string `json:"Nat"`

	NetIPNet *net.IPNet `json:"-"`
	NatIPNet *net.IPNet `json:"-"`
}

type DNSRecord struct {
	Domain   string   `json:"Domain"`
	Wildcard bool     `json:"Wildcard"`
	IP       []string `json:"IP"`
	TXT      []string `json:"TXT"`
}

type ServerConnectResponse struct {
	InterfaceIP string `json:"InterfaceIP"`

	DNSRecords []*DNSRecord `json:"DNSRecords"`
	Networks   []*Network   `json:"Networks"`
	Routes     []*Route     `json:"Routes"`
	DNSServers []string     `json:"DNSServers"`

	WireGuardIP      string `json:"WireGuardIP,omitempty"`
	WireGuardIPv6    string `json:"WireGuardIPv6,omitempty"`
	WireGuardPubKey  string `json:"WireGuardPubKey,omitempty"`
	WireGuardPort    string `json:"WireGuardPort,omitempty"`
	WireGuardSubnet  string `json:"WireGuardSubnet,omitempty"`
	WireGuardSubnet6 string `json:"WireGuardSubnet6,omitempty"`

	WANCIDR string `json:"WANCIDR,omitempty"`

	EnableFirewall bool `json:"EnableFirewall,omitempty"`
}

type FORM_GET_DEVICE struct {
	DeviceID uuid.UUID
}

type WGPeer struct {
	PublicKeyHex  string `json:"PublicKeyHex"`
	DeviceID      string `json:"DeviceID"`
	WireGuardIP   string `json:"WireGuardIP,omitempty"`
	WireGuardIPv6 string `json:"WireGuardIPv6,omitempty"`
}

type WGPeersResponse struct {
	Peers      []WGPeer `json:"Peers"`
	Limit      int      `json:"Limit"`
	Offset     int      `json:"Offset"`
	NextOffset int      `json:"NextOffset"`
}
