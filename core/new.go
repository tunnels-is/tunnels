package core

import (
	"context"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/zveinn/crypt"
)

func init() {
	STATE.Store(&stateV2{})
	CONFIG.Store(&configV2{})
}

const (
	apiVersion       = 1
	version          = "2.0.0"
	tunnelFileSuffix = ".tun.json"

	DefaultAPIIP   = "127.0.0.1"
	DefaultAPIPort = "7777"

	DefaultDNSIP   = "127.0.0.1"
	DefaultDNSPort = "53"
)

var (
	STATE  atomic.Pointer[stateV2]
	CONFIG atomic.Pointer[configV2]

	// Tunnels, Servers, Meta
	TunnelMetaMap sync.Map
	TunnelMap     sync.Map

	// Logs stuff
	LogQueue      = make(chan string, 1000)
	APILogQueue   = make(chan string, 1000)
	logRecordHash sync.Map

	// Go Routine monitors
	concurrencyMonitor = make(chan *goSignal, 1000)
	tunnelMonitor      = make(chan *TUN, 1000)
	interfaceMonitor   = make(chan *TunnelInterface, 1000)

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
	DNSBlockList   atomic.Pointer[sync.Map]
	DNSCache       sync.Map
	DNSStatsMap    sync.Map
)

type configV2 struct {
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

	user atomic.Pointer[User]

	// Networking parameters
	DefaultGateway       atomic.Pointer[net.IP] `json:"-"`
	DefaultInterface     atomic.Pointer[net.IP] `json:"-"`
	DefaultInterfaceID   atomic.Int32           `json:"-"`
	DefaultInterfaceName atomic.Pointer[string] `json:"-"`

	// Disk Paths and filenames
	BlockListPath  string
	LogPath        string
	ConfigFileName string
	BasePath       string
	TracePath      string
	TunnelsPath    string
	TraceFileName  string
	LogFileName    string
}

type TunnelState int

const (
	TUN_NotReady TunnelState = iota
	TUN_Ready
	TUN_Error
	TUN_Disconnecting
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

func (t *TUN) SetState(state TunnelState) {
	t.state.Store(&state)
}

func (t *TUN) registerPing(ping time.Time) {
	t.pingTime.Store(&ping)
}

type TUN struct {
	id    string
	state atomic.Pointer[TunnelState]
	tag   string

	meta atomic.Pointer[TunnelMETA]
	// server atomic.Pointer[any]
	tunnel atomic.Pointer[TunnelInterface]

	// encWrapper wraps connection with encryption
	encWrapper      *crypt.SocketWrapper
	connection      net.Conn
	ServerCertBytes []byte `json:"-"`

	// Connection Requests + Response
	cr        *ConnectionRequest
	crReponse *ConnectRequestResponse

	// NEW MAPPING STUFF
	pingTime                atomic.Pointer[time.Time]
	localInterfaceNetIP     net.IP
	localDNSClient          *dns.Client
	localInterfaceIP4bytes  [4]byte
	serverInterfaceNetIP    net.IP
	serverInterfaceIP4bytes [4]byte
	startPort               uint16
	endPort                 uint16
	TCPEgress               map[[10]byte]*Mapping
	UDPEgress               map[[10]byte]*Mapping

	// Network Natting
	NATEgress  map[[4]byte][4]byte `json:"-"`
	NATIngress map[[4]byte][4]byte `json:"-"`

	// Nonce ?
	Nonce2Bytes []byte

	// VPL
	serverVPLIP [4]byte
	dhcp        *DHCPRecord
	VPLNetwork  *ServerNetwork
	VPLEgress   map[[4]byte]struct{} `json:"-"`
	VPLIngress  map[[4]byte]struct{} `json:"-"`

	// TCP and UDP Natting
	TCPPortMap []VPNPort
	UDPPortMap []VPNPort

	// TODO ....
	Index []byte

	// Stats
	egressBytes   atomic.Int64
	egressString  string
	ingressBytes  atomic.Int64
	ingressString string

	// Tunnel States
	nonce1 uint64
	nonce2 uint64

	// Server States
	CPU                 byte
	DISK                byte
	MEM                 byte
	serverToClientMicro int64

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
	IP_NAT_IP           [4]byte
	IP_NAT_OK           bool

	// NEW PORT MAPPING
	EgressMapping  *Mapping
	IngressMapping *Mapping
	EP_SYN         byte
}

func (t *TUN) InitPortMap() {
	t.TCPPortMap = make([]VPNPort, t.endPort-t.startPort)
	t.UDPPortMap = make([]VPNPort, t.endPort-t.startPort)

	for i := range t.TCPPortMap {
		t.TCPPortMap[i].M = make(map[[4]byte]*Mapping)
	}
	for i := range t.UDPPortMap {
		t.UDPPortMap[i].M = make(map[[4]byte]*Mapping)
	}
}
