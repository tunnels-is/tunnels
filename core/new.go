package core

import (
	"context"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/zveinn/crypt"
)

func init() {
	STATE.Store(&stateV2{})
}

var (
	apiVersion = 1
	version    = "2.0.0"
	STATE      atomic.Pointer[stateV2]
	CONFIG     atomic.Pointer[ConfigV2]

	// Tunnels, Servers, Meta
	TunnelMetaMap sync.Map
	TunnelMap     sync.Map

	// Logs stuff
	LogQueue      = make(chan string, 1000)
	APILogQueue   = make(chan string, 1000)
	logRecordHash sync.Map

	// Go Routine monitors
	concurrencyMonitor = make(chan *goSignal, 1000)
	interfaceMonitor   = make(chan *TunnelInterface, 200)
	tunnelMonitor      = make(chan *Tunnel, 200)

	// testing
	highPriorityChannel   = make(chan *event, 100)
	mediumPriorityChannel = make(chan *event, 100)
	lowPriorityChannel    = make(chan *event, 100)

	// Context
	quit          = make(chan os.Signal, 10)
	GlobalContext = context.Background()
	CancelContext context.Context
	CancelFunc    context.CancelFunc

	// API
	DefaultAPIIP   = "127.0.0.1"
	DefaultAPIPort = "7777"

	// DNS
	DefaultDNSIP   = "127.0.0.1"
	DefaultDNSPort = "53"
	DNSGlobalBlock atomic.Bool
	DNSBlockList   atomic.Pointer[sync.Map]
	DNSCache       sync.Map
	DNSStatsMap    sync.Map
)

type ConfigV2 struct {
	Minimal bool
	OpenUI  bool

	DeviceKey          string
	DNS                string
	Host               string
	Hostname           string
	Port               string
	ServerID           string
	DisableBlockLists  bool
	DisableVPLFirewall bool

	// API Setting
	APIIP          string
	APIPort        string
	APICert        string
	APIKey         string
	APICertDomains []string
	APICertIPs     []string
	APICertType    certs.CertType

	// Generic
	DarkMode          bool
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
	DNS1Default         string
	DNS2Default         string
	DNSOverHTTPS        bool
	DNSstats            bool
	DNSServerIP         string
	DNSServerPort       string
	EnabledBlockLists   []string
	AvailableBlockLists []*BlockList
	DNSRecords          []*ServerDNS
}

type stateV2 struct {
	adminState bool

	// Networking parameters
	DefaultGateway       net.IP
	DefaultInterface     net.IP
	DefaultInterfaceID   int
	DefaultInterfaceName string

	// Disk Paths and filenames
	BlockListPath  string
	LogPath        string
	ConfigFileName string
	BasePath       string
	TunnelsPath    string
	TraceFileName  atomic.Pointer[string]
	TracePath      atomic.Pointer[string]
	LogFileName    atomic.Pointer[string]
}

type TunnelState int

const (
	TUN_NotReady TunnelState = iota
	TUN_Ready
	TUN_Error
	TUN_Disconnected
	// >= TUN_Connected is reserved for connected or potentially connected states
	TUN_Connected
	TUN_Connecting
)

func (t *TUN) GetState() TunnelState {
	ts := t.state.Load()
	if ts == nil {
		return TUN_NotReady
	}

	return *ts
}

type TUN struct {
	id    uuid.UUID
	state atomic.Pointer[TunnelState]
	tag   string

	meta   atomic.Pointer[TunnelMETA]
	server atomic.Pointer[any]
	tunnel atomic.Pointer[TunnelInterface]

	// encWrapper wraps connection with encryption
	encWrapper      *crypt.SocketWrapper
	connection      net.Conn
	ServerCertBytes []byte `json:"-"`

	// Connection Requests + Response
	cr        *ConnectionRequest
	crReponse *ConnectRequestResponse

	// Stats
	egressBytes   int
	egressString  string
	ingressBytes  int
	ingressString string

	// Configurations
	startPort  uint16
	endPort    uint16
	dhcp       *DHCPRecord
	vplNetwork *ServerNetwork

	// Tunnel States
	nonce1 uint64
	nonce2 uint64

	// Server States
	cPU                 byte
	dISK                byte
	mEM                 byte
	derverToClientMicro int64
	pingTime            time.Time

	// Random mappint stuff
	LOCAL_IF_IP [4]byte
	Index       []byte

	// EGRESS PACKET STUFF
	EP_Protocol         byte
	EP_VPNSrcIP         [4]byte
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
	IP_NAT_IP           [4]byte
	IP_NAT_OK           bool

	// NEW PORT MAPPING
	TCP_M  []VPNPort
	UDP_M  []VPNPort
	TCP_EM map[[10]byte]*Mapping
	UDP_EM map[[10]byte]*Mapping
	EP_MP  *Mapping
	IP_MP  *Mapping
	EP_SYN byte

	// VPL
	VPL_IP    [4]byte
	VPL_E_MAP map[[4]byte]struct{} `json:"-"`
	VPL_I_MAP map[[4]byte]struct{} `json:"-"`

	//  NAT
	NAT_CACHE         map[[4]byte][4]byte `json:"-"`
	REVERSE_NAT_CACHE map[[4]byte][4]byte `json:"-"`
}
