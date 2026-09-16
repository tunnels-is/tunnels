package client

import (
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tunnels-is/tunnels/types"
)

const officialControllerHost = "api.tunnels.is"

func isOfficialControllerHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return false
	}
	if name, _, err := net.SplitHostPort(h); err == nil {
		h = strings.ToLower(name)
	}
	h = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
	return h == officialControllerHost
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

type Config struct {
	// KillSwitchIPv4 blackholes 0.0.0.0/0 until the user turns it off.
	// Off by default. When on, controller/VPN endpoints are pinned /32.
	KillSwitchIPv4 bool
	// KillSwitchIPv6 blackholes ::/0 until the user turns it off.
	// Off by default in newly generated configs (existing configs that omit
	// the key are still migrated to on via applyMissingKillSwitchDefaults).
	KillSwitchIPv6 bool

	ControlServers    []*ControlServer
	DisableBlockLists bool

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

type State struct {
	adminState bool

	DefaultGateway       atomic.Pointer[net.IP] `json:"-"`
	DefaultInterface     atomic.Pointer[net.IP] `json:"-"`
	DefaultInterfaceID   atomic.Int32           `json:"-"`
	DefaultInterfaceName atomic.Pointer[string] `json:"-"`

	Debug         bool
	RequireConfig bool
	TunnelType    string
	Pprof         bool
	PprofAddr     string

	BlockListPath  string
	WhiteListPath  string
	LogPath        string
	ConfigFileName string
	BasePath       string
	AccountsPath   string
	TunnelsPath    string
	DevicesPath    string
	LogFileName    string
	UserPath       string

	ActiveAccountHash string
}
