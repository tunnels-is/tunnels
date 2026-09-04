package client

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/tunnels-is/tunnels/types"
	"github.com/tunnels-is/tunnels/version"
)

// Config is the exported alias for the on-disk client configuration.
type Config = configV2

// State is the exported alias for runtime client state.
type State = stateV2

// ErrTunnelConnected is returned when a tunnel cannot be modified while up.
var ErrTunnelConnected = errors.New("tunnel is connected")

// ValidationError is a set of field-level problems (tunnel save, etc.).
type ValidationError struct {
	Messages []string
}

func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	return strings.Join(e.Messages, "; ")
}

// CreateDeviceResult is the one-shot WireGuard config returned when creating
// a remote device from this machine.
type CreateDeviceResult struct {
	WGConfig string        `json:"WGConfig"`
	Device   *types.Device `json:"Device"`
}

var uiLogHandler atomic.Value // func(string)

// SetUILogHandler registers an in-process log sink. The callback must not
// block: the log processor calls it on every line. Pass nil to clear.
func SetUILogHandler(fn func(string)) {
	if fn == nil {
		uiLogHandler.Store(func(string) {})
		return
	}
	uiLogHandler.Store(fn)
}

func emitUILog(line string) {
	v := uiLogHandler.Load()
	if v == nil {
		return
	}
	fn, ok := v.(func(string))
	if !ok || fn == nil {
		return
	}
	fn(line)
}

// SnapshotLogs returns a copy of the in-memory log buffer.
func SnapshotLogs() []string {
	PollLogMu.Lock()
	defer PollLogMu.Unlock()
	out := make([]string, len(PollLogBuf))
	copy(out, PollLogBuf)
	return out
}

// GetUsers returns every saved account on disk.
func GetUsers() ([]*User, error) {
	return getUsers()
}

// SaveUser writes the account file and activates its workspace.
func SaveUser(u *User) error {
	return saveUser(u)
}

// DeleteUser removes a saved account by its folder hash.
func DeleteUser(hash string) error {
	return delUser(hash)
}

// ActivateAccount switches the on-disk workspace to userID's account.
func ActivateAccount(userID string) error {
	return activateAccountByUserID(userID)
}

// CloneConfig returns a shallow copy of the live config with copied slices.
// Nested pointer elements (servers, lists, records) are still shared; copy
// those before mutating an individual item.
func CloneConfig() *configV2 {
	src := CONFIG.Load()
	if src == nil {
		return &configV2{}
	}
	dst := *src
	dst.ControlServers = append([]*ControlServer(nil), src.ControlServers...)
	dst.DNSBlockLists = append([]*BlockList(nil), src.DNSBlockLists...)
	dst.DNSWhiteLists = append([]*BlockList(nil), src.DNSWhiteLists...)
	dst.DNSRecords = append([]*types.DNSRecord(nil), src.DNSRecords...)
	return &dst
}

// CreateTunnel allocates a new random tunnel and persists it.
func CreateTunnel() (*TunnelMETA, error) {
	return createRandomTunnel()
}

// SaveTunnel validates and writes tunnel metadata. oldTag is the previous
// identifier when renaming.
func SaveTunnel(meta *TunnelMETA, oldTag string) error {
	if meta == nil {
		return errors.New("tunnel metadata is required")
	}

	connected := false
	tunnelMapRange(func(t *TUN) bool {
		if t.CR != nil && t.CR.Tag == meta.Tag {
			connected = true
			return false
		}
		return true
	})
	if connected {
		return ErrTunnelConnected
	}

	msgs := validateTunnelMeta(meta, oldTag)
	if len(msgs) > 0 {
		return &ValidationError{Messages: msgs}
	}

	TunnelMetaMap.Store(meta.Tag, meta)
	if err := writeTunnelsToDisk(meta.Tag); err != nil {
		return err
	}

	if oldTag != "" && oldTag != meta.Tag {
		TunnelMetaMap.Delete(oldTag)
		state := STATE.Load()
		ext := meta.ConfigFormat
		if ext == "" {
			ext = tunnelFileSuffix
		}
		if err := os.Remove(state.TunnelsPath + oldTag + ext); err != nil {
			return err
		}
	}
	return nil
}

// DeleteTunnel removes a tunnel from disk and memory.
func DeleteTunnel(tag string) error {
	if !safeTunnelTag(tag) {
		return errors.New("invalid tunnel tag")
	}
	state := STATE.Load()
	ext := tunnelFileSuffix
	if stored, ok := TunnelMetaMap.Load(tag); ok {
		if stored.ConfigFormat != "" {
			ext = stored.ConfigFormat
		}
	}
	_ = os.Remove(state.TunnelsPath + tag + ext)
	TunnelMetaMap.Delete(tag)
	return nil
}

// SetTunnelPeers replaces the allow-list for a tunnel and announces it if
// the tunnel is currently connected.
func SetTunnelPeers(tag string, allowedHosts []string, allowAll bool) ([]string, error) {
	meta, ok := TunnelMetaMap.Load(tag)
	if !ok {
		return nil, errors.New("tunnel not found")
	}

	seen := make(map[string]struct{}, len(allowedHosts))
	hosts := make([]string, 0, len(allowedHosts))
	for _, h := range allowedHosts {
		entry, err := NormalizeAllowedHost(h)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[entry]; dup {
			continue
		}
		seen[entry] = struct{}{}
		hosts = append(hosts, entry)
	}

	meta.AllowedHosts = hosts
	meta.AllowAll = allowAll
	TunnelMetaMap.Store(meta.Tag, meta)
	if err := writeTunnelsToDisk(meta.Tag); err != nil {
		return nil, err
	}

	tunnelMapRange(func(t *TUN) bool {
		m := t.meta.Load()
		if m == nil || m.Tag != tag {
			return true
		}
		if t.GetState() >= TUN_Connected {
			if err := t.AnnounceAllowedHosts(hosts, allowAll); err != nil {
				DEBUG("peer list announce failed: ", err)
			}
		}
		return false
	})

	return hosts, nil
}

// DisconnectTunnel stops reconnects and tears down the tunnel.
func DisconnectTunnel(id, tag string) error {
	if tag == "" {
		tunnelMapRange(func(t *TUN) bool {
			if t.ID == id {
				if m := t.meta.Load(); m != nil {
					tag = m.Tag
				}
				return false
			}
			return true
		})
	}
	if tag != "" {
		stopReconnect(tag)
	} else {
		stopAllReconnects()
	}
	return Disconnect(id, false)
}

// UpdateBlockLists re-downloads every configured DNS block list.
func UpdateBlockLists() []*BlockList {
	forceReloadBlockLists()
	return CONFIG.Load().DNSBlockLists
}

// UpdateWhiteLists re-downloads every configured DNS white list.
func UpdateWhiteLists() []*BlockList {
	forceReloadWhiteLists()
	return CONFIG.Load().DNSWhiteLists
}

// GetDNSListContent returns the local custom block/white list file.
func GetDNSListContent(kind string) (*DNSListContent, error) {
	return getCustomDNSListContent(kind)
}

// SetDNSListContent writes the local custom block/white list file.
func SetDNSListContent(kind, content string) (*DNSListContent, error) {
	return setCustomDNSListContent(kind, content)
}

// GetDNSStats snapshots resolver statistics.
func GetDNSStats() map[string]*DNSStats {
	stats := make(map[string]*DNSStats)
	DNSStatsMap.Range(func(key string, value any) bool {
		if s, ok := value.(*DNSStats); ok {
			stats[key] = s
		}
		return true
	})
	return stats
}

// GetLocalDevices lists devices created on this machine for userID's account.
func GetLocalDevices(userID string) ([]LocalDeviceInfo, error) {
	if userID != "" {
		if err := activateAccountByUserID(userID); err != nil {
			return nil, err
		}
	}
	list, err := listLocalDeviceInfo()
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []LocalDeviceInfo{}
	}
	return list, nil
}

// ControllerRequest posts JSON to a configured control server. When
// deviceToken is set, X-Device-Token / X-UID are sent.
func ControllerRequest(server *ControlServer, path string, body any, uid, deviceToken string) ([]byte, int, error) {
	if err := authorizeControlServer(server); err != nil {
		return nil, 403, err
	}

	var extra map[string]string
	if deviceToken != "" {
		extra = map[string]string{
			"X-Device-Token": deviceToken,
			"X-UID":          uid,
		}
	}

	url := server.GetURL(path)
	resp, code, err := SendRequestToURL(
		nil,
		"POST",
		url,
		body,
		20000,
		server.ValidateCertificate,
		server.CertificatePath,
		extra,
	)
	if err != nil {
		return resp, code, err
	}
	if code == 0 {
		return resp, 500, errors.New("unable to contact controller")
	}
	return resp, code, nil
}

// ControllerError extracts an Error field from a controller JSON body.
func ControllerError(body []byte, fallback string) string {
	if len(body) == 0 {
		if fallback != "" {
			return fallback
		}
		return "unknown error"
	}
	var er ErrorResponse
	if err := json.Unmarshal(body, &er); err == nil && er.Error != "" {
		return er.Error
	}
	s := strings.TrimSpace(string(body))
	if s != "" && s[0] != '{' && s[0] != '[' {
		return s
	}
	if fallback != "" {
		return fallback
	}
	return s
}

func (t *TUN) IngressBytes() int64 {
	if t == nil {
		return 0
	}
	return t.ingressBytes.Load()
}

func (t *TUN) EgressBytes() int64 {
	if t == nil {
		return 0
	}
	return t.egressBytes.Load()
}

func (t *TUN) IngressString() string {
	return BandwidthBytesToString(t.IngressBytes())
}

func (t *TUN) EgressString() string {
	return BandwidthBytesToString(t.EgressBytes())
}

func (t *TUN) Meta() *TunnelMETA {
	if t == nil {
		return nil
	}
	return t.meta.Load()
}

// CloneTunnelMETA deep-copies list fields used by the editor.
func CloneTunnelMETA(src *TunnelMETA) *TunnelMETA {
	if src == nil {
		return nil
	}
	dst := *src
	dst.DNSServers = append([]string(nil), src.DNSServers...)
	dst.AllowedHosts = append([]string(nil), src.AllowedHosts...)
	dst.BlockedPorts = append([]uint16(nil), src.BlockedPorts...)
	if len(src.Routes) > 0 {
		dst.Routes = make([]*types.Route, len(src.Routes))
		for i, r := range src.Routes {
			if r == nil {
				continue
			}
			cp := *r
			dst.Routes[i] = &cp
		}
	}
	if len(src.Networks) > 0 {
		dst.Networks = make([]*types.Network, len(src.Networks))
		for i, n := range src.Networks {
			if n == nil {
				continue
			}
			cp := *n
			dst.Networks[i] = &cp
		}
	}
	if len(src.DNSRecords) > 0 {
		dst.DNSRecords = make([]*types.DNSRecord, len(src.DNSRecords))
		for i, rec := range src.DNSRecords {
			if rec == nil {
				continue
			}
			cp := *rec
			cp.IP = append([]string(nil), rec.IP...)
			cp.TXT = append([]string(nil), rec.TXT...)
			dst.DNSRecords[i] = &cp
		}
	}
	return &dst
}

// FindTunnel returns tunnel metadata by tag.
func FindTunnel(tag string) *TunnelMETA {
	if t, ok := TunnelMetaMap.Load(tag); ok {
		return t
	}
	return nil
}

// StateResponse is the snapshot Fyne (and tests) use for the live client.
type StateResponse struct {
	Version       string
	APIVersion    int
	Timezone      string
	Config        *configV2
	State         *stateV2
	Tunnels       []*TunnelMETA
	ActiveTunnels []*TUN
}

func getSystemTimezone() string {
	if tz := os.Getenv("TZ"); tz != "" && tz != ":/etc/localtime" {
		return strings.TrimPrefix(tz, ":")
	}

	if b, err := os.ReadFile("/etc/timezone"); err == nil {
		if name := strings.TrimSpace(string(b)); name != "" {
			return name
		}
	}
	if link, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(link, "zoneinfo/"); i != -1 {
			return link[i+len("zoneinfo/"):]
		}
	}

	if resolved, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
		if i := strings.Index(resolved, "zoneinfo/"); i != -1 {
			return resolved[i+len("zoneinfo/"):]
		}
	}
	return ""
}

// GetFullState snapshots config, tunnels, and runtime state.
func GetFullState() (s *StateResponse) {
	defer RecoverAndLog()
	state := STATE.Load()
	s = new(StateResponse)
	s.Version = version.Version
	s.APIVersion = version.ApiVersion
	s.Timezone = getSystemTimezone()
	s.Config = CONFIG.Load()
	s.State = state

	tunnelMetaMapRange(func(tun *TunnelMETA) bool {
		s.Tunnels = append(s.Tunnels, tun)
		return true
	})

	tunnelMapRange(func(tun *TUN) bool {
		s.ActiveTunnels = append(s.ActiveTunnels, tun)
		return true
	})
	return
}
