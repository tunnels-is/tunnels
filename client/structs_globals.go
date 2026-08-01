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
	"github.com/tunnels-is/tunnels/types"
	"golang.zx2c4.com/wireguard/device"
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
	DefaultControllerIP = "89.147.109.61"

	DefaultTunnelName = "tunnels"

	STATE  atomic.Pointer[stateV2]
	CONFIG atomic.Pointer[configV2]

	TunnelMetaMap *xsync.MapOf[string, *TunnelMETA]
	TunnelMap     *xsync.MapOf[string, *TUN]

	LogQueue      = make(chan string, 1000)
	APILogQueue   = make(chan string, 1000)
	logRecordHash *xsync.MapOf[string, bool]
	PollLogMu     sync.Mutex
	PollLogBuf    []string

	concurrencyMonitor = make(chan *goSignal, 1000)
	tunnelMonitor      = make(chan *TUN, 1000)

	highPriorityChannel   = make(chan *event, 100)
	mediumPriorityChannel = make(chan *event, 100)
	lowPriorityChannel    = make(chan *event, 100)

	quit          = make(chan os.Signal, 10)
	GlobalContext = context.Background()
	CancelContext context.Context
	CancelFunc    context.CancelFunc

	DNSGlobalBlock atomic.Bool
	DNSBlockList   atomic.Pointer[DomainSet]
	DNSWhiteList   atomic.Pointer[DomainSet]
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
	Server *ControlServer

	DeviceKey string `json:"DeviceKey"`

	DeviceToken string `json:"DeviceToken"`
	UserID      string `json:"UserID"`

	Tag      string `json:"Tag"`
	ServerID string `json:"ServerID"`

	ServerIP   string `json:"ServerIP"`
	ServerPort string `json:"ServerPort"`
}

type ErrorResponse struct {
	Error string `json:"Error"`
}

var (
	DIST_EMBED  embed.FS
	DLL_EMBED   embed.FS
	EnableTLS   bool
	DevMode     bool
	EnablePprof bool // expose net/http/pprof on the local API server
)

var (
	DNSClient = new(dns.Client)

	API_SERVER http.Server

	TAG_ERROR    = "ERROR"
	LogFile      *os.File
	TraceFile    *os.File
	UDPDNSServer atomic.Pointer[dns.Server]
)

type DNSReply struct {
	A       []dns.RR
	Expires time.Time
}

var letterRunes = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")

type DisconnectForm struct {
	ID string `json:"ID"`

	Tag string `json:"Tag"`
}

type TunnelMETA struct {
	ConfigFormat string

	DNSBlocking   bool
	LocalhostNat  bool
	AutoReconnect bool
	AutoConnect   bool
	KillSwitch    bool

	TxQueueLen int32
	MTU        int32
	IFName     string

	Tag      string
	ServerID string

	EnableDefaultRoute bool
	DNSServers         []string
	DNSRecords         []*types.DNSRecord
	Networks           []*types.Network
	Routes             []*types.Route
	BlockedPorts       []uint16

	AllowedHosts []string

	AllowAll bool

	EnableWAN bool

	WireGuardPrivKey string
}

type FORWARD_REQUEST struct {
	Server *ControlServer

	Path     string
	Method   string
	Timeout  int
	JSONData any
	Headers  map[string]string
}

type CreateDeviceWithKeysForm struct {
	Server      *ControlServer
	Tag         string
	ServerID    string
	DeviceToken string
	UID         string
}

type TWO_FACTOR_CONFIRM struct {
	Email  string
	Code   string
	Digits string
}

type QR_CODE struct {
	Value string
}

type DEVICE_TOKEN struct {
	DT      string    `json:"DT"`
	N       string    `json:"N"`
	Created time.Time `json:"C"`
}

type DelUserForm struct {
	Hash string
}

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
	Enabled      bool
	Count        int
	LastDownload time.Time
}

type ControlServer struct {
	ID                  string
	Host                string
	Port                string
	CertificatePath     string
	ValidateCertificate bool
}

// effectivePort returns the port used for outbound controller connections.
// Temporary: while old and new controllers co-exist, force api.tunnels.is:443 → :444
// at request time only — config on disk is left unchanged.
func (c *ControlServer) effectivePort() string {
	if c == nil {
		return ""
	}
	if c.Host == "api.tunnels.is" && c.Port == "443" {
		return "444"
	}
	return c.Port
}

func (c *ControlServer) GetHostAndPort() string {
	hostPort := c.Host
	if p := c.effectivePort(); p != "" {
		hostPort += ":" + p
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
	ControlServerID string
	DeviceToken     string
	UserID          string
	ServerID        string
	SendStats       bool
}

type configV2 struct {
	OpenUI bool

	ControlServers    []*ControlServer
	DisableBlockLists bool
	CLIConfig         *CLIConfig

	APIIP          string
	APIPort        string
	APICert        string
	APIKey         string
	APICertDomains []string
	APICertIPs     []string
	APICertType    certs.CertType

	LogBlockedPorts  bool
	DebugLogging     bool
	ConsoleLogging   bool
	InfoLogging      bool
	ErrorLogging     bool
	ConsoleLogOnly   bool
	ConnectionTracer bool
	BandwidthGraphs  bool

	DisableDNS        bool
	LogBlockedDomains bool
	LogAllDomains     bool
	DNS1Default       string
	DNS2Default       string
	DNSOverHTTPS      bool
	DNSHTTPSAutomatic bool
	DNSstats          bool
	DNSServerIP       string
	DNSServerPort     string
	DNSBlockLists     []*BlockList
	DNSWhiteLists     []*BlockList
	DNSRecords        []*types.DNSRecord
}

type stateV2 struct {
	adminState bool

	DefaultGateway       atomic.Pointer[net.IP] `json:"-"`
	DefaultInterface     atomic.Pointer[net.IP] `json:"-"`
	DefaultInterfaceID   atomic.Int32           `json:"-"`
	DefaultInterfaceName atomic.Pointer[string] `json:"-"`

	Debug         bool
	RequireConfig bool
	TunnelType    string

	BlockListPath  string
	WhiteListPath  string
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
	TUN_NotReady
	TUN_Ready
	TUN_Connecting
	TUN_Connected
)

const (
	MaxBandwidthRecords = 24 * 60 * 60
)

type BandwidthRecord struct {
	Timestamp    time.Time `json:"ts"`
	EgressBytes  int64     `json:"eg"`
	IngressBytes int64     `json:"ig"`
}

type BandwidthHistory struct {
	mu      sync.RWMutex
	records []BandwidthRecord
}

func (bh *BandwidthHistory) Append(r BandwidthRecord) {
	bh.mu.Lock()
	defer bh.mu.Unlock()

	bh.records = append(bh.records, r)

	if len(bh.records) > MaxBandwidthRecords {
		excess := len(bh.records) - MaxBandwidthRecords
		bh.records = bh.records[excess:]
	}
}

func (bh *BandwidthHistory) Snapshot() []BandwidthRecord {
	bh.mu.RLock()
	defer bh.mu.RUnlock()
	out := make([]BandwidthRecord, len(bh.records))
	copy(out, bh.records)
	return out
}

type TUN struct {
	ID    string
	state atomic.Pointer[TunnelState] `json:"-"`

	meta atomic.Pointer[TunnelMETA] `json:"-"`

	tunnel atomic.Pointer[TInterface] `json:"-"`

	wgDevice *device.Device

	CR             *ConnectionRequest
	ServerResponse *types.ServerConnectResponse

	blockedPortsSet map[[2]byte]uint16 `json:"-"`

	localInterfaceNetIP     net.IP
	localDNSClient          *dns.Client
	localInterfaceIP4bytes  [4]byte
	serverInterfaceNetIP    net.IP
	serverInterfaceIP4bytes [4]byte

	natMu      sync.RWMutex        `json:"-"`
	NATEgress  map[[4]byte][4]byte `json:"-"`
	NATIngress map[[4]byte][4]byte `json:"-"`

	egressBytes      atomic.Int64
	ingressBytes     atomic.Int64
	BandwidthHistory atomic.Pointer[BandwidthHistory] `json:"-"`

	PingInt atomic.Int64

	EP_Protocol         byte
	EP_DstIP            [4]byte
	EP_IPv4HeaderLength byte
	EP_IPv4Header       []byte
	EP_TPHeader         []byte
	EP_DstPort          [2]byte
	EP_NAT_IP           [4]byte
	EP_NAT_OK           bool

	IP_SrcIP            [4]byte
	IP_IPv4HeaderLength byte
	IP_IPv4Header       []byte
	IP_TPHeader         []byte
	IP_NAT_IP           [4]byte
	IP_NAT_OK           bool
}

type event struct {
	method func()
}

type goSignal struct {
	monitor chan *goSignal
	ctx     context.Context

	method func()
	tag    string
}

func init() {
	STATE.Store(&stateV2{})
	CONFIG.Store(&configV2{})

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

func (t *TUN) RecordBandwidth() {
	defer RecoverAndLog()

	bh := &BandwidthHistory{
		records: make([]BandwidthRecord, 0, MaxBandwidthRecords),
	}
	t.BandwidthHistory.Store(bh)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastEgress, lastIngress int64

	for {
		select {
		case <-ticker.C:
			if t.GetState() < TUN_Connected {
				return
			}

			if !CONFIG.Load().BandwidthGraphs {
				continue
			}

			currentEgress := t.egressBytes.Load()
			currentIngress := t.ingressBytes.Load()

			deltaEgress := currentEgress - lastEgress
			deltaIngress := currentIngress - lastIngress

			lastEgress = currentEgress
			lastIngress = currentIngress

			bh.Append(BandwidthRecord{
				Timestamp:    time.Now(),
				EgressBytes:  deltaEgress,
				IngressBytes: deltaIngress,
			})
		}
	}
}

func (t *TUN) MarshalJSON() ([]byte, error) {
	eb := BandwidthBytesToString(t.egressBytes.Load())
	ib := BandwidthBytesToString(t.ingressBytes.Load())

	var bwHistory []BandwidthRecord
	if bh := t.BandwidthHistory.Load(); bh != nil {
		bwHistory = bh.Snapshot()
	}

	return json.Marshal(struct {
		ID               string
		CR               *ConnectionRequest
		CRResponse       *types.ServerConnectResponse
		Egress           string
		Ingress          string
		BandwidthHistory []BandwidthRecord `json:"BandwidthHistory,omitempty"`
	}{
		t.ID,
		t.CR,
		t.ServerResponse,
		eb,
		ib,
		bwHistory,
	})
}

func (t *TUN) InitBlockedPorts(ports []uint16) {
	if len(ports) == 0 {
		return
	}

	t.blockedPortsSet = make(map[[2]byte]uint16)
	for _, port := range ports {
		var portBytes [2]byte

		portBytes[0] = byte(port >> 8)
		portBytes[1] = byte(port & 0xFF)

		t.blockedPortsSet[portBytes] = port
	}
}
