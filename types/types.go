package types

import (
	"net"
	"time"

	"github.com/google/uuid"
)

type Feature string

const (
	AUTH Feature = "AUTH"
	DNS  Feature = "DNS"
	WG   Feature = "WG"
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

type LogConfig struct {
	Level  string `json:"Level,omitempty"`
	JSON   bool   `json:"JSON,omitempty"`
	Silent bool   `json:"Silent,omitempty"`
	Source bool   `json:"Source,omitempty"`
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
	TwoFactorKey     string
	CookieSigningKey string
	PayKey           string
	CertPem      string
	SignPem      string
	KeyPem       string

	// Enables multiple key/pairs for API SNI rotation
	CertPems []string
	KeyPems  []string

	Log *LogConfig `json:"Log,omitempty"`

	// WG holds bootstrap config for the wg-server feature.
	// Only required when the WG feature is enabled.
	WG *WGBootstrap
}

// WGBootstrap holds the configuration needed to start the wg-server feature
// inside the server binary. The wg-server uses these values to fetch its full
// config from the controller over HTTP, preserving the same auth layer as the
// standalone wg-server binary.
type WGBootstrap struct {
	// APIKey is the per-server API key (from POST /ui/wg/server-config).
	APIKey string
	// ControllerURL is the base URL of the controller (e.g. "https://1.2.3.4").
	// When empty it defaults to https://APIIP:APIPort (i.e. self).
	ControllerURL string
	// InsecureSkipVerify disables TLS certificate verification when calling
	// the controller. Only use this for testing.
	InsecureSkipVerify bool
	// PrivateKey is the wg-server's Curve25519 private key, base64-encoded.
	// Owned by wg-server (not the controller). Persisted across restarts so
	// existing clients stay valid. Generated on first boot when empty.
	PrivateKey string `json:"PrivateKey,omitempty"`
}

type SecretStore string

const (
	EnvStore    SecretStore = "env"
	ConfigStore SecretStore = "config"
)

type Device struct {
	ID        uuid.UUID   `json:"_id"`
	CreatedAt time.Time   `json:"CreatedAt"`
	Tag       string      `json:"Tag"`
	Groups    []uuid.UUID `json:"Groups"`

	// UserID is the ID of the user who owns this device.
	UserID uuid.UUID `json:"UserID,omitempty"`

	// ServerID links the device to its WireGuard server for subnet-based IP assignment.
	ServerID uuid.UUID `json:"ServerID,omitempty"`

	// WireGuardKey is the client's Curve25519 public key (base64).
	WireGuardKey string `json:"WireGuardKey,omitempty"`

	// WireGuardIP is the IP assigned to this device within the server's WireGuard subnet.
	// Assigned at device creation time by the controller.
	WireGuardIP string `json:"WireGuardIP,omitempty"`

	// WireGuardIPv6 is the IPv6 address assigned within the server's v6 subnet.
	// Empty when IPv6 is not configured on the server.
	WireGuardIPv6 string `json:"WireGuardIPv6,omitempty"`
}

type FORM_GET_SERVER struct {
	DeviceToken string    `json:"DeviceToken"`
	DeviceKey   string    `json:"DeviceKey"`
	UID         uuid.UUID `json:"UID"`
	ServerID    uuid.UUID `json:"ServerID"`
}

type Server struct {
	ID      uuid.UUID   `json:"_id"`
	Tag     string      `json:"Tag"`
	Country string      `json:"Country"`
	IP      string      `json:"IP"`
	Port    string      `json:"Port"`
	Groups  []uuid.UUID `json:"Groups,omitempty"`

	// WGConfigID links this server to its WGServerConfig record.
	WGConfigID uuid.UUID `json:"WGConfigID,omitempty"`

	// Cached fields — source of truth is WGServerConfig; refreshed on assign/fetch.
	WireGuardPort    string `json:"WireGuardPort,omitempty"`
	WireGuardPubKey  string `json:"WireGuardPubKey,omitempty"`
	WireGuardSubnet  string `json:"WireGuardSubnet,omitempty"`
	WireGuardSubnet6 string `json:"WireGuardSubnet6,omitempty"`
}

// WGServerConfig holds all operational configuration for a wg-server instance.
// It is stored in the controller DB and fetched by the wg-server at boot using
// its per-server APIKey.
type WGServerConfig struct {
	ID  uuid.UUID `json:"_id"`
	Tag string    `json:"Tag"`

	// APIKey is the per-server secret; wg-server sends this in X-WG-KEY to
	// authenticate /wg/server-config/fetch, /wg/peers, and /wg/servers.
	APIKey string `json:"APIKey"`

	WireGuardPort   int    `json:"WireGuardPort"`
	WireGuardPubKey string `json:"WireGuardPubKey"`
	WireGuardIface  string `json:"WireGuardIface"`

	// NetworkID references the Network record whose CIDR is the WireGuard subnet.
	// The subnet is resolved at runtime — it is not stored on this struct.
	NetworkID uuid.UUID `json:"NetworkID,omitempty"`

	// NetworkID6 references the Network record whose CIDR is the IPv6 WireGuard subnet.
	// Optional — when empty, IPv6 is not served.
	NetworkID6 uuid.UUID `json:"NetworkID6,omitempty"`

	InternetIface string `json:"InternetIface"`

	PacketInspection   bool `json:"PacketInspection"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify"`
}

// WGServerConfigResponse is returned by GET /wg/server-config/fetch to the
// wg-server. It includes all operational parameters needed to bring up the
// WireGuard interface. The private key is generated locally by the wg-server.
type WGServerConfigResponse struct {
	// ServerID is the UUID of the Server record linked to this config.
	ServerID string `json:"ServerID"`

	WireGuardPort    int    `json:"WireGuardPort"`
	WireGuardSubnet  string `json:"WireGuardSubnet"`
	WireGuardSubnet6 string `json:"WireGuardSubnet6,omitempty"`
	WireGuardIface   string `json:"WireGuardIface"`

	InternetIface string `json:"InternetIface"`

	PacketInspection   bool `json:"PacketInspection"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify"`
}

// WGServerInfo describes a peer wg-server for cross-server routing.
type WGServerInfo struct {
	WireGuardPubKey  string `json:"WireGuardPubKey"`
	WireGuardPort    string `json:"WireGuardPort"`
	WireGuardSubnet  string `json:"WireGuardSubnet"`
	WireGuardSubnet6 string `json:"WireGuardSubnet6,omitempty"`
	IP               string `json:"IP"`
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

	// WireGuard transport fields (populated when server has WG enabled)
	WireGuardIP      string `json:"WireGuardIP,omitempty"`
	WireGuardIPv6    string `json:"WireGuardIPv6,omitempty"`
	WireGuardPubKey  string `json:"WireGuardPubKey,omitempty"`
	WireGuardPort    string `json:"WireGuardPort,omitempty"`
	WireGuardSubnet  string `json:"WireGuardSubnet,omitempty"`
	WireGuardSubnet6 string `json:"WireGuardSubnet6,omitempty"`
}

type FORM_GET_DEVICE struct {
	DeviceID uuid.UUID
}

// WireGuard types

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
	NextOffset int      `json:"NextOffset"` // -1 when there are no more pages
}
