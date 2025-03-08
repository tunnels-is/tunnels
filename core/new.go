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
	CLI.Store(&cliParameters{})
}

var (
	version = "2.0.0"
	STATE   atomic.Pointer[stateV2]
	CLI     atomic.Pointer[cliParameters]
	TUNNELS sync.Map

	LogQueue      = make(chan string, 1000)
	APILogQueue   = make(chan string, 1000)
	logRecordHash sync.Map

	// routineMonitor     = make(chan int, 200)
	concurrencyMonitor = make(chan *goSignal, 1000)
	interfaceMonitor   = make(chan *TunnelInterface, 200)
	tunnelMonitor      = make(chan *Tunnel, 200)

	// testing
	highPriorityChannel   = make(chan *event, 100)
	mediumPriorityChannel = make(chan *event, 100)
	lowPriorityChannel    = make(chan *event, 100)

	quit          = make(chan os.Signal, 10)
	GlobalContext = context.Background()
	CancelContext context.Context
	CancelFunc    context.CancelFunc

	// DNS
	DNSBlockList   atomic.Pointer[sync.Map]
	DNSBlocksMap   map[string]*DNSStats
	DNSResolvesMap map[string]*DNSStats

	DNSLock         = sync.Mutex{}
	DNSBlockedList  = make(map[string]*DNSStats)
	DNSResolvedList = make(map[string]*DNSStats)
	DNSCache        = make(map[string]*DNSReply)
	DNSCacheLock    = sync.Mutex{}
	UsePrimaryDNS   = true
)

type cliParameters struct {
	DeviceKey          string
	DNS                string
	Host               string
	Hostname           string
	Port               string
	ServerID           string
	DisableBlockLists  bool
	DisableVPLFirewall bool
	BasePath           string
}

type stateV2 struct {
	// ??
	adminState bool

	// Networking parameters
	DefaultGateway     atomic.Pointer[net.IP]
	DefaultInterface   atomic.Pointer[net.IP]
	DefaultInterfaceID atomic.Pointer[int]

	// Disk Paths and filenames
	BlockListPath  string
	LogPath        string
	ConfigFileName string
	BasePath       string

	TraceFileName atomic.Pointer[string]
	TracePath     atomic.Pointer[string]
	LogFileName   atomic.Pointer[string]

	// Generic configurations
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

	// DNS configurations
	DNS1Default         atomic.Pointer[string]
	DNS2Default         atomic.Pointer[string]
	DNSOverHTTPS        atomic.Bool
	DNSstats            atomic.Bool
	DNSServerIP         atomic.Pointer[string]
	DNSServerPort       atomic.Pointer[string]
	EnabledBlockLists   []string
	AvailableBlockLists []*BlockList
	DNSRecords          []*ServerDNS

	// API Setting
	APIIP          string
	APIPort        string
	APICert        string
	APIKey         string
	APICertDomains []string
	APICertIPs     []string
	APICertType    certs.CertType
}

type TunnelState int

const (
	TUN_NotReady TunnelState = iota
	TUN_Ready
	TUN_Connecting
	TUN_Connected
	TUN_Disconnected
	TUN_Error
)

type TUN struct {
	id    uuid.UUID
	state TunnelState

	meta   atomic.Pointer[TunnelMETA]
	server atomic.Pointer[any]
	tunif  atomic.Pointer[TunnelInterface]

	// Network connection to server
	con net.Conn

	// Connection Requests
	cr        ConnectionRequest
	crReponse ConnectRequestResponse

	// Stats
	tag           string
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

	// Encryption
	EncryptionHandler *crypt.SocketWrapper

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
