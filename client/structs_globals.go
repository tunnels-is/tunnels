package client

import (
	"context"
	"embed"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/types"
)

const (
	tunnelFileSuffix = ".conf"
	configFileSuffix = ".conf"
	backupFileSuffix = ".bak"

	DefaultAPIIP   = "127.0.0.1"
	DefaultAPIPort = "7777"

	DefaultDNSIP   = "127.0.0.1"
	DefaultDNSPort = "53"
)

var (
	PRODUCTION = true

	DefaultTunnelName = "tunnels"
	// CertPool          *x509.CertPool

	// New global state and config
	STATE  atomic.Pointer[stateV2]
	CONFIG atomic.Pointer[configV2]

	// Tunnels, Servers, Meta
	TunnelMetaMap *xsync.MapOf[string, *TunnelMETA]
	TunnelMap     *xsync.MapOf[string, *TUN]

	// Logs stuff
	LogQueue      = make(chan string, 1000)
	APILogQueue   = make(chan string, 1000)
	logRecordHash *xsync.MapOf[string, bool]

	// Go Routine monitors
	concurrencyMonitor = make(chan *goSignal, 1000)
	tunnelMonitor      = make(chan *TUN, 1000)
	interfaceMonitor   = make(chan *TUN, 1000)

	// NOT SURE YET
	highPriorityChannel   = make(chan *event, 100)
	mediumPriorityChannel = make(chan *event, 100)
	lowPriorityChannel    = make(chan *event, 100)

	// Context
	quit          = make(chan os.Signal, 10)
	GlobalContext = context.Background()
	CancelContext context.Context
	CancelFunc    context.CancelFunc

	// DNS
	DNSGlobalBlock atomic.Bool
	DNSBlockList   atomic.Pointer[*xsync.MapOf[string, bool]]
	DNSCache       *xsync.MapOf[string, any]
	DNSStatsMap    *xsync.MapOf[string, any]
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

type CLIInfo struct {
	AuthServer string
	DeviceID   string
	DeviceKey  string
	ServerID   string
	DNS        bool
	Secure     bool
	Enabled    bool
	SendStats  bool
}

type ControlServer struct {
	ID                  string
	Host                string
	Port                string
	CertificatePath     string
	ValidateCertificate bool
	// STUNHost string
	// STUNKey  string
}

func (c *ControlServer) GetHostAndPort() string {
	hostPort := c.Host
	if c.Port != "" {
		hostPort += ":" + c.Port
	}
	return hostPort
}

func (c *ControlServer) GetURL(path string) string {
	url := c.GetHostAndPort()
	path = strings.TrimPrefix(path, "/")
	url = "https://" + url + "/" + path

	return url
}

type CLIConfig struct {
	// Cli specific settings
	ControlServerID  string
	DeviceID         string
	ServerID         string
	SendStats        bool
	PinVersion       bool
	SkipUpdatePrompt bool
}

type configV2 struct {
	OpenUI bool

	ControlServers    []*ControlServer
	DisableBlockLists bool
	CLIConfig         *CLIConfig

	// Updating
	AutoDownloadUpdate   bool
	ExitPostUpdate       bool
	RestartPostUpdate    bool
	UpdateWhileConnected bool
	UpdateCheckInterval  int
	DisableUpdates       bool

	// API Setting
	APIIP          string
	APIPort        string
	APICert        string
	APIKey         string
	APICertDomains []string
	APICertIPs     []string
	APICertType    certs.CertType

	// Generic
	DisableDNS        bool
	LogBlockedDomains bool
	LogAllDomains     bool
	DebugLogging      bool
	DeepDebugLoggin   bool
	ConsoleLogging    bool
	InfoLogging       bool
	ErrorLogging      bool
	ConsoleLogOnly    bool
	ConnectionTracer  bool

	// DNS
	DNS1Default   string
	DNS2Default   string
	DNSOverHTTPS  bool
	DNSstats      bool
	DNSServerIP   string
	DNSServerPort string
	DNSBlockLists []*BlockList
	DNSRecords    []*types.DNSRecord
}

type stateV2 struct {
	adminState bool

	// user atomic.Pointer[User]

	// Networking parameters
	DefaultGateway       atomic.Pointer[net.IP] `json:"-"`
	DefaultInterface     atomic.Pointer[net.IP] `json:"-"`
	DefaultInterfaceID   atomic.Int32           `json:"-"`
	DefaultInterfaceName atomic.Pointer[string] `json:"-"`

	// Flags
	Debug         bool
	RequireConfig bool
	TunnelType    string

	// Disk Paths and filenames
	BlockListPath  string
	LogPath        string
	ConfigFileName string
	BasePath       string
	TunnelsPath    string
	LogFileName    string
	UserPath       string
}

type TunnelState int

const (
	TUN_Error TunnelState = iota
	TUN_Disconnecting
	TUN_Disconnected
	// >= TUN_Connected is reserved for connected or potentially connected states
	TUN_Connected
	TUN_Connecting
	TUN_NotReady
	TUN_Ready
)

type Mapping struct {
	Proto    byte
	rstFound atomic.Bool
	finCount atomic.Int32

	// LastActivity     time.Time
	SrcPort          [2]byte
	DstPort          [2]byte
	MappedPort       [2]byte
	OriginalSourceIP [4]byte
	DestinationIP    [4]byte
	UnixTime         atomic.Int64
}

type TUN struct {
	ID    string
	state atomic.Pointer[TunnelState] `json:"-"`

	meta atomic.Pointer[TunnelMETA] `json:"-"`
	// server atomic.Pointer[any]
	tunnel atomic.Pointer[TInterface] `json:"-"`

	// encWrapper wraps connection with encryption
	encWrapper *crypt.SocketWrapper
	connection net.Conn

	// Connection Requests + Response
	CR             *ConnectionRequest
	ServerResponse *types.ServerConnectResponse

	pingTime                atomic.Pointer[time.Time]
	localInterfaceNetIP     net.IP
	localDNSClient          *dns.Client
	localInterfaceIP4bytes  [4]byte
	serverInterfaceNetIP    net.IP
	serverInterfaceIP4bytes [4]byte
	startPort               uint16
	endPort                 uint16

	NATEgress  map[[4]byte][4]byte `json:"-"`
	NATIngress map[[4]byte][4]byte `json:"-"`

	Nonce2Bytes []byte

	serverVPLIP [4]byte
	dhcp        *types.DHCPRecord
	VPLNetwork  *types.Network
	VPLEgress   map[[4]byte]struct{} `json:"-"`
	VPLIngress  map[[4]byte]struct{} `json:"-"`

	// TCP and UDP Natting
	// ingress
	// index == local port number
	// lport/dip/dp
	AvailableTCPPorts []*xsync.MapOf[any, *Mapping]
	AvailableUDPPorts []*xsync.MapOf[any, *Mapping]

	// egress
	// sip/dip/sp/dp
	// key == [12]byte
	ActiveTCPMapping *xsync.MapOf[any, *Mapping]
	ActiveUDPMapping *xsync.MapOf[any, *Mapping]

	Index []byte

	// Stats
	egressBytes  atomic.Int64
	ingressBytes atomic.Int64

	// Server States
	PingInt             atomic.Int64
	CPU                 byte
	DISK                byte
	MEM                 byte
	ServerToClientMicro atomic.Int64

	// Random mappint stuff
	// LOCAL_IF_IP [4]byte

	// EGRESS PACKET STUFF
	EP_Protocol         byte
	EP_DstIP            [4]byte
	EP_IPv4HeaderLength byte
	EP_IPv4Header       []byte
	EP_TPHeader         []byte
	EP_SrcPort          [2]byte
	EP_DstPort          [2]byte
	EP_NAT_IP           [4]byte
	EP_NAT_OK           bool

	// INGRESS PACKET STUFF
	IP_Protocol         byte
	IP_SrcIP            [4]byte
	IP_IPv4HeaderLength byte
	IP_IPv4Header       []byte
	IP_TPHeader         []byte
	IP_DstPort          [2]byte
	IP_SrcPort          [2]byte
	IP_NAT_IP           [4]byte
	IP_NAT_OK           bool

	// NEW PORT MAPPING
	EgressMapping  *Mapping
	IngressMapping *Mapping
	// EP_SYN         byte
}

type event struct {
	// method is executed inside priority channels
	method func()
	// done is executed on method completion
	done chan any
}

type goSignal struct {
	monitor chan *goSignal
	ctx     context.Context
	// cancel context.CancelFunc
	method func()
	tag    string
}

func init() {
	STATE.Store(&stateV2{})
	CONFIG.Store(&configV2{})

	// Initialize xsync maps
	TunnelMetaMap = xsync.NewMapOf[string, *TunnelMETA]()
	TunnelMap = xsync.NewMapOf[string, *TUN]()
	logRecordHash = xsync.NewMapOf[string, bool]()
	DNSCache = xsync.NewMapOf[string, any]()
	DNSStatsMap = xsync.NewMapOf[string, any]()
}

func (t *TUN) GetState() TunnelState {
	ts := t.state.Load()
	if ts == nil {
		return TUN_NotReady
	}

	return *ts
}

func (t *TUN) SetState(state TunnelState) {
	t.state.Store(&state)
}

func (t *TUN) registerPing(ping time.Time) {
	t.pingTime.Store(&ping)
}

// Implement MarshalJSON method
func (t *TUN) MarshalJSON() ([]byte, error) {
	// Create a type alias to avoid recursion

	var pingTime time.Time
	if t.pingTime.Load() != nil {
		pingTime = *t.pingTime.Load()
	}
	eb := BandwidthBytesToString(t.egressBytes.Load())
	ib := BandwidthBytesToString(t.ingressBytes.Load())

	// Define the structure we want in JSON
	return json.Marshal(struct {
		ID         string
		CR         *ConnectionRequest
		CRResponse *types.ServerConnectResponse
		Ping       time.Time
		StartPort  int
		EndPort    int
		DHCP       *types.DHCPRecord
		LAN        *types.Network
		CPU        byte
		DISK       byte
		MEM        byte
		Egress     string
		Ingress    string
		MS         int64
	}{
		t.ID,
		t.CR,
		t.ServerResponse,
		pingTime,
		int(t.startPort),
		int(t.endPort),
		t.dhcp,
		t.VPLNetwork,
		t.CPU,
		t.DISK,
		t.MEM,
		eb,
		ib,
		t.ServerToClientMicro.Load(),
	})
}

func (t *TUN) InitPortMap() {
	t.AvailableTCPPorts = make([]*xsync.MapOf[any, *Mapping], t.endPort-t.startPort)
	t.AvailableUDPPorts = make([]*xsync.MapOf[any, *Mapping], t.endPort-t.startPort)

	// Initialize each map in the slice
	for i := range t.AvailableTCPPorts {
		t.AvailableTCPPorts[i] = xsync.NewMapOf[any, *Mapping]()
	}
	for i := range t.AvailableUDPPorts {
		t.AvailableUDPPorts[i] = xsync.NewMapOf[any, *Mapping]()
	}

	t.ActiveTCPMapping = xsync.NewMapOf[any, *Mapping]()
	t.ActiveUDPMapping = xsync.NewMapOf[any, *Mapping]()
}
