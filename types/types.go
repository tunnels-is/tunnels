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
	LogAPIHosts bool

	ClientVersion string

	APIIP   string
	APIPort string

	// If SecretStore set to "config"
	AdminAPIKey      string
	DBurl            string
	TwoFactorKey     string
	CookieSigningKey string
	PayKey           string
	CertPem          string
	KeyPem           string

	// Enables multiple key/pairs for API SNI rotation
	CertPems []string
	KeyPems  []string
}

type WGBootstrap struct {
	APIKey             string
	ControllerURL      string
	InsecureSkipVerify bool
}

type SecretStore string

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

	// APIKey is the per-server secret; the wg-server running on this host sends
	// it in X-WG-KEY to authenticate /wg/server-config/fetch, /wg/peers, and
	// /wg/servers. Empty when WG is not enabled on this server.
	APIKey string `json:"APIKey,omitempty"`

	WireGuardPort   int    `json:"WireGuardPort,omitempty"`
	WireGuardPubKey string `json:"WireGuardPubKey,omitempty"`
	WireGuardIface  string `json:"WireGuardIface,omitempty"`

	// WireGuardSubnet is the IPv4 CIDR the wg-server hands out to peers.
	WireGuardSubnet string `json:"WireGuardSubnet,omitempty"`

	// WireGuardSubnet6 is the IPv6 CIDR for peers. Optional — when empty, IPv6
	// is not served.
	WireGuardSubnet6 string `json:"WireGuardSubnet6,omitempty"`

	InternetIface string `json:"InternetIface,omitempty"`

	// EnableFirewall turns on the wg-server's peer-to-peer firewall. When
	// enabled, all peer-to-peer ingress is blocked by default; a device opens
	// itself up by announcing the peer IPs allowed to reach it via the ACL
	// control port. Peer traffic to the server's own WG IP is always blocked,
	// regardless of this setting.
	EnableFirewall     bool `json:"EnableFirewall,omitempty"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify,omitempty"`
}

// WGServerConfigResponse is returned by GET /wg/server-config/fetch to the
// wg-server. It includes all operational parameters needed to bring up the
// WireGuard interface. The private key is generated locally by the wg-server.
type WGServerConfigResponse struct {
	// ServerID is the UUID of the Server record linked to this config.
	ServerID string `json:"ServerID"`

	// ServerIP is the public IP of this wg-server, as recorded on the Server
	// record. The wg-server uses it as the SNAT source for outbound traffic so
	// peer egress shows up with the correct public address on multi-homed hosts.
	ServerIP string `json:"ServerIP,omitempty"`

	WireGuardPort    int    `json:"WireGuardPort"`
	WireGuardSubnet  string `json:"WireGuardSubnet"`
	WireGuardSubnet6 string `json:"WireGuardSubnet6,omitempty"`
	WireGuardIface   string `json:"WireGuardIface"`

	InternetIface string `json:"InternetIface"`

	EnableFirewall     bool `json:"EnableFirewall"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify"`
}

// WGServerInfo describes a peer wg-server for cross-server routing.
type WGServerInfo struct {
	WireGuardPubKey  string `json:"WireGuardPubKey"`
	WireGuardPort    int    `json:"WireGuardPort"`
	WireGuardSubnet  string `json:"WireGuardSubnet"`
	WireGuardSubnet6 string `json:"WireGuardSubnet6,omitempty"`
	IP               string `json:"IP"`
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
