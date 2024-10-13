package core

import (
	"context"
	"crypto/x509"
	"embed"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-ping/ping"
	"github.com/miekg/dns"
	"github.com/zveinn/crypt"
)

var (
	DefaultTunnelName = "tunnels"
	CertPool          *x509.CertPool

	DNSLock         = sync.Mutex{}
	DNSBlockedList  = make(map[string]*DNSStats)
	DNSResolvedList = make(map[string]*DNSStats)
	DNSCache        = make(map[string]*DNSReply)
	DNSCacheLock    = sync.Mutex{}
	UsePrimaryDNS   = true
)

type DNSStats struct {
	Count     int
	LastSeen  time.Time
	FirstSeen time.Time
	Answers   []string
}

type UIConnectRequest struct {
	// Only present client side
	Tag         string        `json:"Tag"`
	DeviceToken string        `json:"DeviceToken"`
	UserID      string        `json:"UserID"`
	SeverID     string        `json:"ServerID"`
	ServerIP    string        `json:"ServerIP"`
	ServerPort  string        `json:"ServerPort"`
	EncType     crypt.EncType `json:"EncType"`
}

type ConnectionRequest struct {
	// These are delivered by the user
	DeviceToken string        `json:"DeviceToken"`
	EncType     crypt.EncType `json:"EncType"`
	UserID      string        `json:"UserID"`
	SeverID     string        `json:"ServerID"`
	Serial      string        `json:"Serial"`

	// These are added by the golang client
	Version int       `json:"Version"`
	UUID    string    `json:"UUID"`
	Created time.Time `json:"Created"`
}

type ErrorResponse struct {
	Error string `json:"Error"`
}

type SignedConnectRequest struct {
	Signature []byte
	Payload   []byte
}

type ConnectRequestResponse struct {
	Index             int `json:"Index"`
	AvailableMbps     int `json:"AvailableMbps"`
	AvailableUserMbps int `json:"AvailableUserMbps"`

	InternetAccess     bool `json:"InternetAccess,required"`
	LocalNetworkAccess bool `json:"LocalNetworkAccess"`

	DataPort    string `json:"DataPort"`
	InterfaceIP string `json:"InterfaceIP"`
	StartPort   uint16 `json:"StartPort"`
	EndPort     uint16 `json:"EndPort"`

	DNS                []*ServerDNS     `json:"DNS"`
	Networks           []*ServerNetwork `json:"Networks"`
	DNSServers         []string         `json:"DNSServers"`
	DNSAllowCustomOnly bool             `json:"DNSAllowCustomOnly"`
}

var (
	PRODUCTION  = true
	APP_VERSION = "2.2.1"
	API_VERSION = 1
)

var (
	DIST_EMBED embed.FS
	DLL_EMBED  embed.FS
)

func initializeGlobalVariables() {
	C = new(Config)
	C.DebugLogging = true
	C.InfoLogging = true

	GLOBAL_STATE.DNSBlocksMap = make(map[string]*DNSStats)
	GLOBAL_STATE.DNSResolvesMap = make(map[string]*DNSStats)
}

var (
	AppStartTime  = time.Now()
	C             = new(Config)
	GLOBAL_STATE  = new(State)
	quit          = make(chan os.Signal, 10)
	GlobalContext = context.Background()
	CancelContext context.Context
	CancelFunc    context.CancelFunc

	DEFAULT_TUNNEL      *TunnelInterface
	DEFAULT_DNS_SERVERS []string
	DNSClient           = new(dns.Client)

	// DEFAULT CONNECTION
	DEFAULT_CONNECTION *TunnelMETA

	// IS NATIVE GUI
	NATIVE bool
	// Base Path Overwrite
	BASE_PATH string

	// HTTP
	API_SERVER http.Server
	API_PORT   string

	// INTERFACE RELATED
	DEFAULT_GATEWAY         net.IP
	DEFAULT_INTERFACE       net.IP
	DEFAULT_INTERFACE_ID    int
	DEFAULT_INTERFACE_NAME  string
	ROUTER_PROBE_TIMEOUT_MS = 60000
	LAST_ROUTER_PROBE       = time.Now().AddDate(0, 0, -1)
	// LAST_GOOD_ROUTER_INDEX  = 777777

	// STATISTICS
	CURRENT_UBBS           = 0
	CURRENT_DBBS           = 0
	EGRESS_PACKETS  uint64 = 0
	INGRESS_PACKETS uint64 = 0

	// LOG RELATED
	// L           = new(Logs)
	LogQueue          = make(chan string, 1000)
	APILogQueue       = make(chan string, 1000)
	TAG_ERROR         = "ERROR"
	TAG_GENERAL       = "GENERAL"
	LogFile           *os.File
	TraceFile         *os.File
	UDPDNSServer      *dns.Server
	BLOCK_DNS_QUERIES = false
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

// NETWORKING STUFF
var (
	TCP_MAP      = make(map[[4]byte]*IP)
	TCP_MAP_LOCK = sync.RWMutex{}
)

var (
	UDP_MAP      = make(map[[4]byte]*IP)
	UDP_MAP_LOCK = sync.RWMutex{}
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

type LoggerInterface struct{}

// type Logs struct {
// 	LOGS [1000]string
// }

type LogItem struct {
	Type string
	Line string
}

type LogoutForm struct {
	Email       string
	DeviceToken string
}
type LoginForm struct {
	Email       string
	Password    string
	DeviceName  string
	DeviceToken string
	Digits      string
	Recovery    string
}

type State struct {
	C    *Config `json:"C"`
	User User

	UMbps           int    `json:"UMbps"`
	DMbps           int    `json:"DMbps"`
	UMbpsString     string `json:"UMbpsString"`
	DMbpsString     string `json:"DMbpsString"`
	IngressPackets  uint64 `json:"IngressPackets"`
	EgressPackets   uint64 `json:"EgressPackets"`
	ConnectionStats []TunnelSTATS

	IsAdmin                bool `json:"IsAdmin"`
	BaseFolderInitialized  bool `json:"BaseFolderInitialized"`
	LogFileInitialized     bool `json:"LogFileInitialized"`
	TraceFileInitialized   bool `json:"TraceFileInitialized"`
	ConfigInitialized      bool `json:"ConfigInitialized"`
	DefaultInterfaceOnline bool `json:"DefaultInterfaceOnline"`

	LastNodeUpdate         time.Time
	SecondsUntilNodeUpdate int

	AvailableCountries []string `json:"AvailableCountries"`
	// Servers            []*Server `json:"Servers"`

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

	// BLOCKING AND PARENTAL CONTROLS
	// AvailableBlockLists []*List `json:"AvailableBlockLists"`

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
	GUID string `json:"GUID"`
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
	ConList [1000]*Tunnel
	ConLock = sync.Mutex{}
	IFList  [1000]*TunnelInterface
	IFLock  = sync.Mutex{}
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

type ServerDNS struct {
	Domain   string   `json:"Domain"`
	Wildcard bool     `json:"Wildcard" bson:"Wildcard"`
	IP       []string `json:"IP" bson:"IP"`
	TXT      []string `json:"TXT" bson:"TXT"`
	CNAME    string   `json:"CNAME" bson:"CNAME"`
}
type ServerNetwork struct {
	Tag     string   `json:"Tag" bson:"Tag"`
	Network string   `json:"Network" bson:"Network"`
	Nat     string   `json:"Nat" bson:"Nat"`
	Routes  []*Route `json:"Routes" bson:"Routes"`

	// Post Init
	NatIPNet *net.IPNet `json:"-"`
	NetIPNet *net.IPNet `json:"-"`
}

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
	Private     bool
	PrivateIP   string
	PrivatePort string
	PrivateCert string

	// NEW
	WindowsGUID string

	ServerID string

	// controlled by user only
	DNSBlocking     bool
	AutomaticRouter bool
	LocalhostNat    bool
	AutoReconnect   bool
	AutoConnect     bool
	Persistent      bool
	PreventIPv6     bool

	EncryptionType crypt.EncType

	// EXPERIMENTAL
	CloseConnectionsOnConnect bool

	// Is delivered from company but can be overwirtten by user
	TxQueueLen int32
	MTU        int32
	IFName     string

	// IS controller by ORG if user is part of one
	// ID                  primitive.ObjectID `json:"_id,omitempty" bson:"_id,omitempty"`
	Tag         string
	IPv4Address string
	IPv6Address string
	NetMask     string

	// This overwrites or adds to settings
	// that are applied to the Node
	EnableDefaultRoute bool
	DNSServers         []string
	DNS                []*ServerDNS
	Networks           []*ServerNetwork
}

// func (VC *VPNConnection) Initialize() {
// 	if len(VC.Node.DNSServers) > 0 {
// 		VC.DNS1IP = net.ParseIP(VC.Node.DNSServers[0]).To4()
// 		VC.DNS1Bytes = [4]byte(VC.DNS1IP)
// 	} else {
// 		VC.DNS1IP = net.ParseIP(C.DNS1Default).To4()
// 		VC.DNS1Bytes = [4]byte(VC.DNS1IP)
// 	}
// }

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
	CPU      byte
	DISK     byte
	MEM      byte
	PingTime time.Time
}

type Tunnel struct {
	Meta *TunnelMETA
	TunnelSTATS
	CRR  *ConnectRequestResponse
	UICR UIConnectRequest
	Con  net.Conn

	// TUN/TAP
	Index        []byte
	Nonce2Bytes  []byte
	Interface    *TunnelInterface
	AddressNetIP net.IP
	Routes       []string

	// ??????
	// CRR       *ConnectRequestResponse
	StartPort uint16
	EndPort   uint16
	EH        *crypt.SocketWrapper

	// STATES
	Connected              bool
	UserRWLoopAbnormalExit bool
	Connecting             bool
	Exiting                bool

	// VPN NODE
	// CRR     *VPNNode
	PingBuffer [8]byte

	// DNS1Bytes     [4]byte `json:"-"`
	// DNS1IP        net.IP  `json:"-"`
	PrevDNS       net.IP
	DNSBytes      [4]byte
	DNSIP         net.IP
	DNSEgressLock sync.Mutex

	// NEW PORT MAPPING
	TCP_M  []VPNPort
	UDP_M  []VPNPort
	TCP_EM map[[10]byte]*Mapping
	UDP_EM map[[10]byte]*Mapping
	EP_MP  *Mapping
	IP_MP  *Mapping
	EP_SYN byte

	//  NAT
	NAT_CACHE         map[[4]byte][4]byte `json:"-"`
	REVERSE_NAT_CACHE map[[4]byte][4]byte `json:"-"`

	// BufferError bool

	//  PACKET MANIPULATION
	EP_Version  byte
	EP_Protocol byte

	EP_DstIP [4]byte

	EP_IPv4HeaderLength byte
	EP_IPv4Header       []byte
	EP_TPHeader         []byte

	EP_SrcPort [2]byte
	EP_DstPort [2]byte

	EP_NAT_IP [4]byte
	EP_NAT_OK bool

	EP_DNS_Response         []byte
	EP_DNS_Local            bool
	EP_DNS_Drop             bool
	EP_DNS_Forward          bool
	EP_DNS_Port_Placeholder [2]byte
	EP_DNS_Packet           []byte

	// This IP gets over-written on connect
	EP_VPNSrcIP [4]byte

	// EP_NEW_RST  byte
	PREV_DNS_IP [4]byte
	IS_UNIX     bool

	IP_Version  byte
	IP_Protocol byte

	IP_DstIP [4]byte
	IP_SrcIP [4]byte

	IP_IPv4HeaderLength byte
	IP_IPv4Header       []byte
	IP_TPHeader         []byte

	IP_SrcPort [2]byte
	IP_DstPort [2]byte

	IP_NAT_IP [4]byte
	IP_NAT_OK bool
}

type Config struct {
	DNS1Default    string   `json:"DNS1Default"`
	DNS2Default    string   `json:"DNS2Default"`
	DNSOverHTTPS   bool     `json:"DNSOverHTTPS"`
	APICert        string   `json:"APICert"`
	APIKey         string   `json:"APIKey"`
	APICertDomains []string `json:"APICertDomains"`
	APICertIPs     []string `json:"APICertIPs"`
	APICertType    certType `json:"APICertType"`
	APIAutoTLS     bool     `json:"APIAutoTLS"`
	// AutoReconnect        bool
	// KillSwitch           bool
	// ManualRouter         bool
	DebugLogging   bool
	ConsoleLogging bool
	InfoLogging    bool
	ErrorLogging   bool

	DNSstats bool

	DarkMode bool

	RouterFilePath string

	// Security settings
	IsolationMode bool

	// DNS Blocking
	DomainWhitelist   string
	EnabledBlockLists []string
	LogBlockedDomains bool
	LogAllDomains     bool
	DNSServerIP       string
	DNSServerPort     string

	APIIP   string
	APIPort string

	ConnectionTracer bool

	AvailableBlockLists []*BlockList

	// Connections
	Connections              []*TunnelMETA
	RouterDialTimeoutSeconds int
}

var (
	DNSBlockList = make(map[string]struct{})
	DNSBlockLock = sync.Mutex{}
)

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

type CONTROLL_PUBLIC_DEVCE_RESPONSE struct {
	Routers      []*Server
	AccessPoints []*VPNNode
}

type FORWARD_REQUEST struct {
	Path    string
	Method  string
	Timeout int
	Authed  bool
	// Data     []byte
	JSONData interface{}
}

//	type CONNECTION_SETTINGS struct {
//		DNS1          string
//		DNS2          string
//		AutoDNS       bool
//		IP6Method     string
//		IPV6Enabled   bool
//		IFName        string
//		DefaultRouter string
//		AdapterName   string
//	}
//
//	type INTERFACE_SETTINGS struct {
//		Index           int
//		Flags           net.Flags
//		MTU             int
//		HardwareAddress net.HardwareAddr
//		OIF             net.Interface
//		Hop             string
//		Metric          int
//	}
type Server struct {
	ID      string `json:"ID"`
	IP      string `json:"IP"`
	Tag     string `json:"Tag"`
	Country string `json:"Country"`

	// TCPControllerConnection net.Conn `json:"-"`
	// TCPTunnelConnection     net.Conn `json:"-"`

	// ROUTER_STATS

	// Online    bool   `json:"Online"`
	// Score             int    `json:"Score"`
	// AvailableSlots    int `json:"AvailableSlots"`
	LastPing  time.Time       `json:"-"`
	PingStats ping.Statistics `json:"-"`

	MS                 uint64 `json:"MS"`
	Slots              int    `json:"Slots"`
	AvailableMbps      int    `json:"AvailableMbps"`
	AvailableUserMbps  int    `json:"AvailableUserMbps"`
	InternetAccess     bool   `json:"InternetAccess"`
	LocalNetworkAccess bool   `json:"LocalNetworkAccess"`

	DNSAllowCustomOnly bool `json:"DNSAllowCustomOnly"`

	DNSServers []string         `json:"DNSServers"`
	DNS        []*ServerDNS     `json:"DNS"`
	Networks   []*ServerNetwork `json:"Networks"`
}

type ROUTER_STATS struct {
	AEBP      float64
	AIBP      float64
	CPUP      int
	RAMUsage  int
	DiskUsage int
}

type FullConnectResponse struct {
	Node    *VPNNode
	Session *SessionFromNode
}

// type CONTROLLER_SESSION_REQUEST struct {
// UserID      primitive.ObjectID `json:"UserID"`
// DeviceToken string             `json:"DeviceToken"`
//
// RouterIndex int                `json:"RouterIndex"`
// NodeIndex   int                `json:"NodeIndex"`
// NodePrivate primitive.ObjectID `json:"NodePrivate"`

// QUICK CONNECT
// Country string `json:"Country,omitempty"`

// COMES BACK ON SUCCESS
// ID         primitive.ObjectID `json:"_id,omitempty"`
// ProxyIndex int                `json:"ProxyIndex,omitempty"`

// MAYBE USE LATER
// SLOTID int
// Type   string `json:",omitempty"`
// Permanent bool `json:",omitempty"`
// Count     int  `json:",omitempty"`
// Proto string `json:"Proto,omitempty"`
// Port  string `json:"Port,omitempty"`
// }

type SessionFromNode struct {
	UUID        string `json:",omitempty"`
	Version     int    `json:"Version"`
	Created     time.Time
	StartPort   uint16
	EndPort     uint16
	InterfaceIP net.IP
	Type        crypt.EncType
}

//type CONTROLLER_SESSION struct {
//	UserID primitive.ObjectID `bson:"UID"`
//	ID     primitive.ObjectID `bson:"_id"`
//
//	Permanent bool `bson:"P"`
//	Count     int  `bson:"C"`
//	SLOTID    int  `bson:"SLOTID"`
//
//	GROUP     uint8 `bson:"G"`
//	ROUTERID  uint8 `bson:"RID"`
//	SESSIONID uint8 `bson:"SID"`
//
//	XGROUP    uint8 `bson:"XG"`
//	XROUTERID uint8 `bson:"XRID"`
//	DEVICEID  uint8 `bson:"APID"`
//
//	Assigned     time.Time `bson:"A"`
//	ShouldDelete bool      `bson:"-"`
//}

type VPNNode struct {
	// DELIVERED WITH INITIAL LIST
	Tag               string `json:"Tag"`
	ListIndex         int    `json:"ListIndex"`
	IP                string `json:"IP"`
	InterfaceIP       string `json:"InterfaceIP"`
	Status            int    `json:"Status"`
	Country           string `json:"Country"`
	AvailableMbps     int    `json:"AvailableMbps"`
	Slots             int    `json:"Slots"`
	AvailableUserMbps int    `json:"AvailableUserMbps"`

	// PARSED AFTER LIST DELIVERY
	MS int `json:"MS"`

	// DELIVERED ON CONNECT
	// UID            primitive.ObjectID `json:"-"`
	// ID             primitive.ObjectID `json:"_id,omitempty"`
	AvailableSlots int `json:"AvailableSlots"`

	Access             []*DeviceUserRegistration `json:"Access"`
	Updated            time.Time                 `json:"Updated"`
	InternetAccess     bool                      `json:"InternetAccess"`
	LocalNetworkAccess bool                      `json:"LocalNetworkAccess"`
	Public             bool                      `json:"Public"`

	Online     bool      `json:"Online"`
	LastOnline time.Time `json:"LastOnline"`

	DNSAllowCustomOnly bool             `json:"DNSAllowCustomOnly"`
	DNSServers         []string         `json:"DNSServers" bson:"DNSServers"`
	DNS                []*ServerDNS     `json:"DNS"`
	Networks           []*ServerNetwork `json:"Networks"`
	EncryptionProtocol int              `json:"EncryptionProtocol"` // default 3 (AES256)
}

type DeviceUserRegistration struct {
	// UID primitive.ObjectID `json:"UID" bson:"UID"`
	Tag string `json:"Tag" bson:"T"`
}

type AP_GEO_DB struct {
	Updated     time.Time `json:"Updated" bson:"U"`
	IPV         string    `bson:"IPV" json:"-"`
	Country     string    `bson:"Country" json:"Country"`
	CountryFull string    `bson:"CountryFull" json:"CountryFull"`
	City        string    `bson:"City" json:"City"`
	// ASN     string `bson:"ASN" json:"ASN"`
	ISP   string `bson:"ISP" json:"-"`
	Proxy bool   `bson:"Proxy" json:"Proxy"`
	Tor   bool   `bson:"Tor" json:"Tor"`
}

var PS_IFLIST []*PS_DEFAULT_ROUTES

type PS_DEFAULT_ROUTES struct {
	// CimClass struct {
	// 	CimSuperClassName string `json:"CimSuperClassName,omitempty"`
	// 	CimSuperClass     struct {
	// 		CimSuperClassName   string `json:"CimSuperClassName"`
	// 		CimSuperClass       string `json:"CimSuperClass"`
	// 		CimClassProperties  string `json:"CimClassProperties"`
	// 		CimClassQualifiers  string `json:"CimClassQualifiers"`
	// 		CimClassMethods     string `json:"CimClassMethods"`
	// 		CimSystemProperties string `json:"CimSystemProperties"`
	// 	} `json:"CimSuperClass,omitempty"`
	// 	CimClassProperties  []string `json:"CimClassProperties,omitempty"`
	// 	CimClassQualifiers  []string `json:"CimClassQualifiers,omitempty"`
	// 	CimClassMethods     []string `json:"CimClassMethods,omitempty"`
	// 	CimSystemProperties struct {
	// 		Namespace  string      `json:"Namespace"`
	// 		ServerName string      `json:"ServerName"`
	// 		ClassName  string      `json:"ClassName"`
	// 		Path       interface{} `json:"Path"`
	// 	} `json:"CimSystemProperties,omitempty"`
	// } `json:"CimClass,omitempty"`
	// CimInstanceProperties []struct {
	// 	Name            string      `json:"Name"`
	// 	Value           interface{} `json:"Value"`
	// 	CimType         int         `json:"CimType"`
	// 	Flags           string      `json:"Flags"`
	// 	IsValueModified bool        `json:"IsValueModified"`
	// } `json:"CimInstanceProperties,omitempty"`
	// CimSystemProperties struct {
	// 	Namespace  string      `json:"Namespace"`
	// 	ServerName string      `json:"ServerName"`
	// 	ClassName  string      `json:"ClassName"`
	// 	Path       interface{} `json:"Path"`
	// } `json:"CimSystemProperties,omitempty"`
	// Publish            int         `json:"Publish"`
	// Protocol           int         `json:"Protocol"`
	// Store              int         `json:"Store"`
	// AddressFamily      int         `json:"AddressFamily"`
	// State              int         `json:"State"`
	// IfIndex int `json:"ifIndex"`
	// Caption            interface{} `json:"Caption"`
	// Description        interface{} `json:"Description"`
	// ElementName        interface{} `json:"ElementName"`
	// InstanceID         string      `json:"InstanceID"`
	// AdminDistance      interface{} `json:"AdminDistance"`
	// DestinationAddress interface{} `json:"DestinationAddress"`
	// IsStatic           interface{} `json:"IsStatic"`
	RouteMetric int `json:"RouteMetric"`
	// TypeOfRoute        int         `json:"TypeOfRoute"`
	// CompartmentID      int         `json:"CompartmentId"`
	DestinationPrefix string `json:"DestinationPrefix"`
	InterfaceAlias    string `json:"InterfaceAlias"`
	InterfaceIndex    int    `json:"InterfaceIndex"`
	InterfaceMetric   int    `json:"InterfaceMetric"`
	NextHop           string `json:"NextHop"`
	// PreferredLifetime  struct {
	// 	Ticks             int64   `json:"Ticks"`
	// 	Days              int     `json:"Days"`
	// 	Hours             int     `json:"Hours"`
	// 	Milliseconds      int     `json:"Milliseconds"`
	// 	Minutes           int     `json:"Minutes"`
	// 	Seconds           int     `json:"Seconds"`
	// 	TotalDays         float64 `json:"TotalDays"`
	// 	TotalHours        float64 `json:"TotalHours"`
	// 	TotalMilliseconds int64   `json:"TotalMilliseconds"`
	// 	TotalMinutes      float64 `json:"TotalMinutes"`
	// 	TotalSeconds      float64 `json:"TotalSeconds"`
	// } `json:"PreferredLifetime"`
	// ValidLifetime struct {
	// 	Ticks             int64   `json:"Ticks"`
	// 	Days              int     `json:"Days"`
	// 	Hours             int     `json:"Hours"`
	// 	Milliseconds      int     `json:"Milliseconds"`
	// 	Minutes           int     `json:"Minutes"`
	// 	Seconds           int     `json:"Seconds"`
	// 	TotalDays         float64 `json:"TotalDays"`
	// 	TotalHours        float64 `json:"TotalHours"`
	// 	TotalMilliseconds int64   `json:"TotalMilliseconds"`
	// 	TotalMinutes      float64 `json:"TotalMinutes"`
	// 	TotalSeconds      float64 `json:"TotalSeconds"`
	// } `json:"ValidLifetime"`
	// PSComputerName interface{} `json:"PSComputerName"`
}

type TWO_FACTOR_CONFIRM struct {
	Email  string
	Code   string
	Digits string
}

type QR_CODE struct {
	Value string
	// Recovery string
}

// var CurrentOpenSockets []*OpenSockets

//	type OpenSockets struct {
//		RemoteAddress string  `json:"RemoteAddress"`
//		RemoteIP      [4]byte `json:"-"`
//		LocalPort     uint16  `json:"LocalPort"`
//		RemotePort    uint16  `json:"RemotePort"`
//	}
// type MIB_TCPROW_OWNER_PID struct {
// 	dwState      uint32
// 	dwLocalAddr  uint32
// 	dwLocalPort  uint32
// 	dwRemoteAddr uint32
// 	dwRemotePort uint32
// 	dwOwningPid  uint32
// }
//
// type MIB_TCPTABLE_OWNER_PID struct {
// 	dwNumEntries uint32
// 	table        [30000]MIB_TCPROW_OWNER_PID
// }

// Device token struct need for the login respons from user scruct
type DEVICE_TOKEN struct {
	DT      string    `bson:"DT"`
	N       string    `bson:"N"`
	Created time.Time `bson:"C"`
}

// use struct you get from the login request
type User struct {
	ID                    string          `json:"_id,omitempty" bson:"_id,omitempty"`
	APIKey                string          `bson:"AK" json:"APIKey"`
	Email                 string          `bson:"E"`
	DeviceToken           *DEVICE_TOKEN   `json:",omitempty" bson:"-"`
	Tokens                []*DEVICE_TOKEN `json:"Tokens" bson:"Tokens"`
	OrgID                 string          `json:"OrgID" bson:"OrgID"`
	Key                   *LicenseKey     `json:"Key" bson:"Key"`
	Trial                 bool            `json:"Trial" bson:"Trial"`
	Disabled              bool            `json:"Disabled" bson:"Disabled"`
	TwoFactorEnabled      bool            `json:"TwoFactorEnabled" bson:"TwoFactorEnabled"`
	Updated               time.Time       `json:"Updated" bson:"Updated"`
	AdditionalInformation string          `json:"AdditionalInformation,omitempty" bson:"AdditionalInformation"`
}

type LicenseKey struct {
	Created time.Time
	Months  int
	Key     string
}

type BlockList struct {
	Tag         string
	FullPath    string
	DiskPath    string
	Enabled     bool
	Count       int
	LastRefresh time.Time
}
