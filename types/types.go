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

	// WireGuardKey is the client's Curve25519 public key (base64).
	// IP assignment is owned by each wg-server; the controller only stores the key.
	WireGuardKey string `json:"WireGuardKey,omitempty" bson:"WireGuardKey"`
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

	WireGuardPort   string `json:"WireGuardPort,omitempty" bson:"WireGuardPort"`
	WireGuardPubKey string `json:"WireGuardPubKey,omitempty" bson:"WireGuardPubKey"`

	// WGBaseURL is the base HTTP URL of this wg-server's local management
	// listener (e.g. "http://127.0.0.1:8181"). The controller uses it to call
	// /v3/wg/assign (IP assignment) and /v3/wg/sync (instant peer pickup).
	// Leave empty to skip WireGuard integration for this server.
	WGBaseURL string `json:"WGBaseURL,omitempty" bson:"WGBaseURL"`
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

func CreateCRRFromServer(S *ServerConfig) (CRR *ServerConnectResponse) {
	return &ServerConnectResponse{
		InterfaceIP: S.VPNIP,
		DNSRecords:  S.DNSRecords,
		Networks:    S.SubNets,
		Routes:      S.Routes,
		DNSServers:  S.DNSServers,
	}
}

type ControllerConnectRequest struct {
	DeviceKey   string             `json:"DeviceKey"`
	DeviceToken string             `json:"DeviceToken"`
	UserID      primitive.ObjectID `json:"UserID"`

	ServerID primitive.ObjectID `json:"ServerID"`

	// These are added by the golang client
	Version int       `json:"Version"`
	Created time.Time `json:"Created"`

	// WireGuard: client's base64 Curve25519 public key; triggers inline WG registration
	WireGuardPubKey string `json:"WireGuardPubKey,omitempty"`
}

type FORM_GET_DEVICE struct {
	DeviceID primitive.ObjectID
}

// WireGuard types

type WGPeer struct {
	PublicKeyHex string `json:"PublicKeyHex"`
	DeviceID     string `json:"DeviceID"`
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
