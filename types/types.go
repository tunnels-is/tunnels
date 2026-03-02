package types

import (
	"net"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Feature string

const (
	AUTH  Feature = "AUTH"
	DNS   Feature = "DNS"
	BBOLT Feature = "BBOLT"
	WG    Feature = "WG"
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
	Features           []Feature
	PingTimeoutMinutes int
	LogAPIHosts        bool

	ClientVersion string

	VPNIP string

	APIIP   string
	APIPort string

	NetAdmins []string

	Hostname string
	Routes   []*Route
	SubNets  []*Network

	UserMaxConnections int

	DNSRecords []*DNSRecord
	DNSServers []string

	SecretStore SecretStore
	// If SecretStore set to "config"
	AdminAPIKey  string
	DBurl        string
	TwoFactorKey string
	PayKey       string
	CertPem      string
	SignPem      string
	KeyPem       string

	// Enables multiple key/pairs for API SNI rotation
	CertPems []string
	KeyPems  []string
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

	// ServerID links the device to its WireGuard server for subnet-based IP assignment.
	ServerID primitive.ObjectID `json:"ServerID,omitempty" bson:"ServerID"`

	// WireGuardKey is the client's Curve25519 public key (base64).
	WireGuardKey string `json:"WireGuardKey,omitempty" bson:"WireGuardKey"`

	// WireGuardIP is the IP assigned to this device within the server's WireGuard subnet.
	// Assigned at device creation time by the controller.
	WireGuardIP string `json:"WireGuardIP,omitempty" bson:"WireGuardIP"`
}

type FORM_GET_SERVER struct {
	DeviceToken string             `json:"DeviceToken"`
	DeviceKey   string             `json:"DeviceKey"`
	UID         primitive.ObjectID `json:"UID"`
	ServerID    primitive.ObjectID `json:"ServerID"`
}

type Server struct {
	ID      primitive.ObjectID   `json:"_id" bson:"_id"`
	Tag     string               `json:"Tag" bson:"Tag"`
	Country string               `json:"Country" bson:"Country"`
	IP      string               `json:"IP" bson:"IP"`
	Port    string               `json:"Port" bson:"Port"`
	Groups  []primitive.ObjectID `json:"Groups,omitempty" bson:"Groups"`

	// WGConfigID links this server to its WGServerConfig record.
	WGConfigID primitive.ObjectID `json:"WGConfigID,omitempty" bson:"WGConfigID"`

	// Cached fields — source of truth is WGServerConfig; refreshed on assign/fetch.
	WireGuardPort   string `json:"WireGuardPort,omitempty" bson:"WireGuardPort"`
	WireGuardPubKey string `json:"WireGuardPubKey,omitempty" bson:"WireGuardPubKey"`
	WireGuardSubnet string `json:"WireGuardSubnet,omitempty" bson:"WireGuardSubnet"`
}

// WGServerConfig holds all operational configuration for a wg-server instance.
// It is stored in the controller DB and fetched by the wg-server at boot using
// its per-server APIKey.
type WGServerConfig struct {
	ID  primitive.ObjectID `json:"_id" bson:"_id"`
	Tag string             `json:"Tag" bson:"Tag"`

	// APIKey is the per-server secret; wg-server sends this in X-WG-KEY to
	// authenticate its config fetch.
	APIKey string `json:"APIKey" bson:"APIKey"`

	// AdminAPIKey is the controller's admin key; wg-server needs this to call
	// /v3/wg/peers and other admin endpoints.
	AdminAPIKey string `json:"AdminAPIKey" bson:"AdminAPIKey"`

	WireGuardPort    int    `json:"WireGuardPort" bson:"WireGuardPort"`
	WireGuardPrivKey string `json:"WireGuardPrivKey" bson:"WireGuardPrivKey"`
	WireGuardSubnet  string `json:"WireGuardSubnet" bson:"WireGuardSubnet"`
	WireGuardIface   string `json:"WireGuardIface" bson:"WireGuardIface"`

	InternetIface string `json:"InternetIface" bson:"InternetIface"`

	PacketInspection   bool `json:"PacketInspection" bson:"PacketInspection"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify" bson:"InsecureSkipVerify"`
}

// WGServerConfigResponse is returned by the /v3/wg/server-config/fetch endpoint
// to the wg-server. It includes sensitive fields (PrivKey, AdminAPIKey) that are
// only served on this per-server endpoint.
type WGServerConfigResponse struct {
	// ServerID is the hex ObjectID of the Server record linked to this config.
	ServerID string `json:"ServerID"`

	AdminAPIKey string `json:"AdminAPIKey"`

	WireGuardPort    int    `json:"WireGuardPort"`
	WireGuardPrivKey string `json:"WireGuardPrivKey"`
	WireGuardSubnet  string `json:"WireGuardSubnet"`
	WireGuardIface   string `json:"WireGuardIface"`

	InternetIface string `json:"InternetIface"`

	PacketInspection   bool `json:"PacketInspection"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify"`
}

// WGServerInfo describes a peer wg-server for cross-server routing.
type WGServerInfo struct {
	WireGuardPubKey string `json:"WireGuardPubKey"`
	WireGuardPort   string `json:"WireGuardPort"`
	WireGuardSubnet string `json:"WireGuardSubnet"`
	IP              string `json:"IP"`
}

// WGServersResponse is returned by GET /v3/wg/servers.
type WGServersResponse struct {
	Servers []WGServerInfo `json:"Servers"`
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

type ServerConnectResponse struct {
	InterfaceIP string `json:"InterfaceIP"`

	DNSRecords []*DNSRecord `json:"DNSRecords"`
	Networks   []*Network   `json:"Networks"`
	Routes     []*Route     `json:"Routes"`
	DNSServers []string     `json:"DNSServers"`

	// WireGuard transport fields (populated when server has WG enabled)
	WireGuardIP     string `json:"WireGuardIP,omitempty"`
	WireGuardPubKey string `json:"WireGuardPubKey,omitempty"`
	WireGuardPort   string `json:"WireGuardPort,omitempty"`
}


type FORM_GET_DEVICE struct {
	DeviceID primitive.ObjectID
}

// WireGuard types

type WGPeer struct {
	PublicKeyHex string `json:"PublicKeyHex"`
	DeviceID     string `json:"DeviceID"`
	WireGuardIP  string `json:"WireGuardIP,omitempty"`
}

type WGPeersResponse struct {
	Peers []WGPeer `json:"Peers"`
}

type WGRegisterRequest struct {
	DeviceID     string `json:"DeviceID"`
	PublicKeyB64 string `json:"PublicKeyB64"`
}

type WGRegisterResponse struct {
	AssignedIP   string `json:"AssignedIP"`
	ServerPubKey string `json:"ServerPubKey"`
	ServerIP     string `json:"ServerIP"`
	ServerPort   string `json:"ServerPort"`
	Conf         string `json:"Conf"`
}
