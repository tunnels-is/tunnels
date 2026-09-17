package client

import (
	"context"
	"embed"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/puzpuzpuz/xsync/v3"
)

const (
	tunnelFileSuffix = ".conf"
	configFileSuffix = ".conf"
	backupFileSuffix = ".bak"

	DefaultDNSIP   = "127.0.0.1"
	DefaultDNSPort = "53"
)

var (
	DefaultControllerIP = "89.147.109.61"

	DefaultTunnelName = "tunnels"

	STATE  atomic.Pointer[State]
	CONFIG atomic.Pointer[Config]

	TunnelMetaMap *xsync.MapOf[string, *TunnelMeta]
	TunnelMap     *xsync.MapOf[string, *TUN]

	LogQueue      = make(chan string, 1000)
	logRecordHash *xsync.MapOf[string, bool]
	PollLogMu     sync.Mutex
	PollLogBuf    []string

	concurrencyMonitor = make(chan *backgroundTask, 1000)
	tunnelMonitor      = make(chan *TUN, 1000)

	highPriorityChannel   = make(chan *event, 100)
	mediumPriorityChannel = make(chan *event, 100)
	lowPriorityChannel    = make(chan *event, 100)

	quit          = make(chan os.Signal, 10)
	GlobalContext = context.Background()
	CancelContext context.Context
	CancelFunc    context.CancelFunc

	DNSGlobalBlock atomic.Bool

	DNSBlockList atomic.Pointer[DomainCatalog]
	DNSWhiteList atomic.Pointer[DomainCatalog]
	DNSCache     *xsync.MapOf[string, any]
	DNSStatsMap  *xsync.MapOf[string, any]
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

var (
	WintunDLL embed.FS
)

var (
	DNSClient = new(dns.Client)

	tagError     = "ERROR"
	LogFile      *os.File
	TraceFile    *os.File
	UDPDNSServer atomic.Pointer[dns.Server]
)

type DNSReply struct {
	A       []dns.RR
	Expires time.Time
}

var totpAlphabet = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567")

type event struct {
	method func()
}

type backgroundTask struct {
	monitor chan *backgroundTask
	ctx     context.Context

	method func()
	tag    string
}

func init() {
	STATE.Store(&State{})
	CONFIG.Store(&Config{})

	TunnelMetaMap = xsync.NewMapOf[string, *TunnelMeta]()
	TunnelMap = xsync.NewMapOf[string, *TUN]()
	logRecordHash = xsync.NewMapOf[string, bool]()
	DNSCache = xsync.NewMapOf[string, any]()
	DNSStatsMap = xsync.NewMapOf[string, any]()
}
