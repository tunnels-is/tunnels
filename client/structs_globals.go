package client

import (
	"embed"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/types"
)

var (
	PRODUCTION = true

	DefaultTunnelName = "tunnels"
	// CertPool          *x509.CertPool
)

type DNSStats struct {
	Count        int
	Tag          string
	LastSeen     time.Time
	FirstSeen    time.Time
	LastResolved time.Time
	LastBlocked  time.Time
	Answers      []string
	m            sync.Mutex
}

type ConnectionRequest struct {
	Server       *ControlServer
	ServerPubKey string

	DeviceKey string `json:"DeviceKey"`

	DeviceToken string `json:"DeviceToken"`
	UserID      string `json:"UserID"`

	Tag      string `json:"Tag"`
	ServerID string `json:"ServerID"`

	// Set using API call in PublicConnect
	ServerIP   string `json:"ServerIP"`
	ServerPort string `json:"ServerPort"`
}

type ErrorResponse struct {
	Error string `json:"Error"`
}

type SignedConnectRequest struct {
	Signature []byte
	Payload   []byte
}

// type DHCPRecord struct {
// 	IP       [4]byte
// 	Token    string
// 	Hostname string
// }

var (
	DIST_EMBED embed.FS
	DLL_EMBED  embed.FS
)

var (
	AppStartTime        = time.Now()
	DEFAULT_TUNNEL      *TInterface
	DEFAULT_DNS_SERVERS []string
	DNSClient           = new(dns.Client)

	uiChan = make(chan struct{}, 1)

	// HTTP
	API_SERVER http.Server
	API_PORT   string

	CURRENT_UBBS           = 0
	CURRENT_DBBS           = 0
	EGRESS_PACKETS  uint64 = 0
	INGRESS_PACKETS uint64 = 0

	TAG_ERROR   = "ERROR"
	TAG_GENERAL = "GENERAL"
	LogFile     *os.File
	TraceFile   *os.File
	// UDPDNSServer *dns.Server
	UDPDNSServer atomic.Pointer[dns.Server]
)

type DNSReply struct {
	// M       *dns.Msg
	A       []dns.RR
	Expires time.Time
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")

// var LastRouterPing = time.Now()
var LastConnectionAttemp = time.Now()

var (
	BUFFER_ERROR             bool
	IGNORE_NEXT_BUFFER_ERROR bool
)

var DNSWhitelist = make(map[string]bool)

type IP struct {
	LOCAL  map[uint16]*RemotePort
	REMOTE map[uint16]*RemotePort
}

type RemotePort struct {
	Local        uint16
	Original     uint16
	Mapped       uint16
	LastActivity time.Time
}

type LogItem struct {
	Type string
	Line string
}

type LogoutForm struct {
	Email       string
	DeviceToken string
}

type State struct {
	IsAdmin bool    `json:"IsAdmin"`
	C       *Config `json:"C"`
	User    User

	UMbps           int    `json:"UMbps"`
	DMbps           int    `json:"DMbps"`
	UMbpsString     string `json:"UMbpsString"`
	DMbpsString     string `json:"DMbpsString"`
	IngressPackets  uint64 `json:"IngressPackets"`
	EgressPackets   uint64 `json:"EgressPackets"`
	ConnectionStats []TunnelSTATS

	LastNodeUpdate         time.Time
	SecondsUntilNodeUpdate int

	AvailableCountries []string `json:"AvailableCountries"`

	// FILE PATHS
	BlockListPath string `json:"BlockListPath"`
	TraceFileName string `json:"TraceFileName"`
	TracePath     string `json:"TracePath"`
	LogFileName   string `json:"LogFileName"`
	LogPath       string `json:"LogPath"`
	ConfigPath    string `json:"ConfigPath"`
	BasePath      string `json:"BasePath"`
	Version       string `json:"Version"`

	ActiveConnections []*TunnelMETA `json:"ActiveConnections"`

	// DNS stats
	DNSBlocksMap   map[string]*DNSStats `json:"DNSBlocks"`
	DNSResolvesMap map[string]*DNSStats `json:"DNSResolves"`
}

type List struct {
	FullPath string
	Tag      string
	Enabled  bool
	Domains  string
}

type DisconnectForm struct {
	ID string `json:"ID"`
}

type CONFIG_FORM struct {
	DNS1                      string   `json:"DNS1"`
	DNS2                      string   `json:"DNS2"`
	ManualRouter              bool     `json:"ManualRouter"`
	Region                    string   `json:"Region"`
	Version                   string   `json:"Version"`
	RouterFilePath            string   `json:"RouterFilePath"`
	DebugLogging              bool     `json:"DebugLogging"`
	AutoReconnect             bool     `json:"AutoReconnect"`
	KillSwitch                bool     `json:"KillSwitch"`
	DisableIPv6OnConnect      bool     `json:"DisableIPv6OnConnect"`
	CloseConnectionsOnConnect bool     `json:"CloseConnectionsOnConnect"`
	CustomDNS                 bool     `json:"CustomDNS"`
	EnabledBlockLists         []string `json:"EnabledBlockLists"`
	LogBlockedDomains         bool     `json:"LogBlockedDomains"`
}

var (
// TunList [1000]*Tunnel
// ConLock = sync.Mutex{}
// IFList [1000]*TunnelInterface
// IFLock = sync.Mutex{}
)

type ConnectionOverwrite struct {
	ServerID string `json:"ServerID"`
	Network  string `json:"Network" bson:"Network"`
	Nat      string `json:"Nat" bson:"Nat"`
}

type Route struct {
	Address string
	Metric  string
}

// type ServerDNS struct {
// 	Domain   string   `json:"Domain"`
// 	Wildcard bool     `json:"Wildcard" bson:"Wildcard"`
// 	IP       []string `json:"IP" bson:"IP"`
// 	TXT      []string `json:"TXT" bson:"TXT"`
// 	CNAME    string   `json:"CNAME" bson:"CNAME"`
// }
// type ServerNetwork struct {
// 	Tag     string   `json:"Tag" bson:"Tag"`
// 	Network string   `json:"Network" bson:"Network"`
// 	Nat     string   `json:"Nat" bson:"Nat"`
// 	Routes  []*Route `json:"Routes" bson:"Routes"`

// 	// Post Init
// 	NatIPNet *net.IPNet `json:"-"`
// 	NetIPNet *net.IPNet `json:"-"`
// }

type ActiveConnectionMeta struct {
	Country        string
	RouterIndex    int
	NodeID         string
	Tag            string
	LocalInterface string
	IPv4Address    string
	IPv6Address    string
	StartPort      uint16
	EndPort        uint16
}

type TunnelMETA struct {
	ServerID    string
	WindowsGUID string

	// controlled by user only
	DNSBlocking     bool
	LocalhostNat    bool
	AutoReconnect   bool
	AutoConnect     bool
	RequestVPNPorts bool
	KillSwitch      bool

	EncryptionType crypt.EncType

	TxQueueLen int32
	MTU        int32
	IFName     string

	Tag         string
	IPv4Address string
	IPv6Address string
	NetMask     string

	// VPL Firewall
	AllowedHosts    []string
	DisableFirewall bool

	// This overwrites or adds to settings
	// that are applied to the Node
	EnableDefaultRoute bool
	DNSServers         []string
	DNSRecords         []*types.DNSRecord
	Networks           []*types.Network
	Routes             []*types.Route
}

type AllowedHost struct {
	Host    string
	Expires time.Time
}

type TunnelSTATS struct {
	// Stats
	StatsTag      string
	EgressBytes   int
	EgressString  string
	IngressBytes  int
	IngressString string

	// Port range on server
	StartPort uint16
	EndPort   uint16

	// Security stuff
	Nonce1 uint64
	Nonce2 uint64

	// FROM NODE
	CPU                 byte
	DISK                byte
	MEM                 byte
	ServerToClientMicro int64
	PingTime            time.Time

	DHCP *types.DNSRecord
	LAN  *types.Network
}

type FirewallRequest struct {
	DHCPToken       string
	IP              string
	Hosts           []string
	DisableFirewall bool
}

type Config struct {
	Connections []*TunnelMETA

	DarkMode bool

	// Security settings
	IsolationMode bool

	// API Setting
	APIIP          string
	APIPort        string
	APICert        string
	APIKey         string
	APICertDomains []string
	APICertIPs     []string
	APICertType    certs.CertType

	// Optional Debugging Settings
	LogBlockedDomains bool
	LogAllDomains     bool
	DebugLogging      bool
	DeepDebugLoggin   bool
	ConsoleLogging    bool
	InfoLogging       bool
	ErrorLogging      bool
	ConsoleLogOnly    bool
	ConnectionTracer  bool

	// DNS Settings
	DNS1Default         string
	DNS2Default         string
	DNSOverHTTPS        bool
	DNSstats            bool
	DNSServerIP         string
	DNSServerPort       string
	DomainWhitelist     string
	EnabledBlockLists   []string
	AvailableBlockLists []*BlockList

	DNSRecords []*types.DNSRecord
}

type LOADING_LOGS_RESPONSE struct {
	Lines [100]string
}
type GENERAL_LOGS_RESPONSE struct {
	Lines []string
}
type GeneralLogResponse struct {
	Content  []string
	Time     []string
	Function []string
	Color    []string
}

type DEBUG_OUT struct {
	Lines []string
	File  string
}

type FORWARD_REQUEST struct {
	Server *ControlServer
	// URL      string
	// Secure   bool
	Path     string
	Method   string
	Timeout  int
	JSONData any
}

type TWO_FACTOR_CONFIRM struct {
	Email  string
	Code   string
	Digits string
}

type QR_CODE struct {
	Value string
}

// Device token struct need for the login respons from user scruct
type DEVICE_TOKEN struct {
	DT      string    `bson:"DT"`
	N       string    `bson:"N"`
	Created time.Time `bson:"C"`
}

type DelUserForm struct {
	Hash string
}

// use struct you get from the login request
type User struct {
	ID                    string          `json:"_id,omitempty"`
	APIKey                string          `json:"APIKey"`
	Email                 string          `json:"Email"`
	DeviceToken           *DEVICE_TOKEN   `json:",omitempty"`
	Tokens                []*DEVICE_TOKEN `json:"Tokens"`
	OrgID                 string          `json:"OrgID" `
	Key                   *LicenseKey     `json:"Key"`
	Trial                 bool            `json:"Trial"`
	Disabled              bool            `json:"Disabled"`
	TwoFactorEnabled      bool            `json:"TwoFactorEnabled"`
	Updated               time.Time       `json:"Updated"`
	SubExpiration         time.Time       `json:"SubExpiration"`
	AdditionalInformation string          `json:"AdditionalInformation,omitempty"`
	IsAdmin               bool            `json:"IsAdmin"`
	IsManager             bool            `json:"IsManager"`

	// Client only
	ControlServer *ControlServer
	SaveFileHash  string
}

type LicenseKey struct {
	Created time.Time
	Months  int
	Key     string
}

type BlockList struct {
	Tag          string
	URL          string
	Disk         string
	Enabled      bool
	Count        int
	LastDownload time.Time
}
