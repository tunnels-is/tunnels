package client

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/tunnels-is/tunnels/certs"
	"github.com/tunnels-is/tunnels/crypt"
	"github.com/tunnels-is/tunnels/types"
)

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
	ControlServerID string
	DeviceID        string
	ServerID        string
	SendStats       bool
}

type configV2 struct {
	OpenUI bool

	ControlServers    []*ControlServer
	DisableBlockLists bool
	CLIConfig         *CLIConfig

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

type mapwrap struct {
	atomic.Pointer[Mapping]
}

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
	// ServerCertBytes []byte `json:"-"`

	// Connection Requests + Response
	CR             *ConnectionRequest
	ServerResponse *types.ServerConnectResponse

	// NEW MAPPING STUFF
	pingTime                atomic.Pointer[time.Time]
	localInterfaceNetIP     net.IP
	localDNSClient          *dns.Client
	localInterfaceIP4bytes  [4]byte
	serverInterfaceNetIP    net.IP
	serverInterfaceIP4bytes [4]byte
	startPort               uint16
	endPort                 uint16

	// Network Natting
	NATEgress  map[[4]byte][4]byte `json:"-"`
	NATIngress map[[4]byte][4]byte `json:"-"`

	// Nonce ?
	Nonce2Bytes []byte

	// VPL
	serverVPLIP [4]byte
	dhcp        *types.DHCPRecord
	VPLNetwork  *types.Network
	VPLEgress   map[[4]byte]struct{} `json:"-"`
	VPLIngress  map[[4]byte]struct{} `json:"-"` // TCP and UDP Natting
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
