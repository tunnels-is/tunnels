package client

import (
	"encoding/json"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
	"github.com/tunnels-is/tunnels/types"
	wgconn "golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
)

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

type DisconnectForm struct {
	ID string `json:"ID"`

	Tag string `json:"Tag"`
}

type TunnelMeta struct {
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

type TunnelState int

const (
	TunnelError TunnelState = iota
	TunnelDisconnecting
	TunnelDisconnected
	TunnelNotReady
	TunnelReady
	TunnelConnecting
	TunnelConnected
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

	if bh.records == nil {
		bh.records = make([]BandwidthRecord, 0, MaxBandwidthRecords)
	}
	if len(bh.records) >= MaxBandwidthRecords {
		copy(bh.records, bh.records[1:])
		bh.records[MaxBandwidthRecords-1] = r
		bh.records = bh.records[:MaxBandwidthRecords]
		return
	}
	bh.records = append(bh.records, r)
}

func (bh *BandwidthHistory) Snapshot() []BandwidthRecord {
	return bh.SnapshotSince(time.Time{})
}

// SnapshotSince copies records at or after cutoff. A zero cutoff copies the
// whole buffer. The returned slice has its own backing array, so the caller
// can retain it without pinning discarded samples.
func (bh *BandwidthHistory) SnapshotSince(cutoff time.Time) []BandwidthRecord {
	bh.mu.RLock()
	defer bh.mu.RUnlock()
	recs := bh.records
	i := 0
	if !cutoff.IsZero() {
		i = sort.Search(len(recs), func(i int) bool {
			return recs[i].Timestamp.After(cutoff)
		})
	}
	if i > len(recs) {
		i = len(recs)
	}
	out := make([]BandwidthRecord, len(recs)-i)
	copy(out, recs[i:])
	return out
}

type TUN struct {
	ID    string
	state atomic.Pointer[TunnelState] `json:"-"`

	meta atomic.Pointer[TunnelMeta] `json:"-"`

	tunnel atomic.Pointer[adapter] `json:"-"`

	wgDevice *device.Device
	wgBind   wgconn.Bind
	osTUN    *stickyTUN
	procTUN  *processingTUN

	CR             *ConnectionRequest
	ServerResponse *types.ServerConnectResponse

	blockedPortsSet map[[2]byte]uint16 `json:"-"`

	localInterfaceNetIP     net.IP
	localDNSClient          *dns.Client
	localInterfaceIP4bytes  [4]byte
	serverInterfaceNetIP    net.IP
	serverInterfaceIP4bytes [4]byte
	wgEndpointSet           bool
	wgLoopDropLogged        atomic.Bool
	protectHosts            []string

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

func (t *TUN) GetState() TunnelState {
	ts := t.state.Load()
	if ts == nil {
		return TunnelNotReady
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
			if t.GetState() < TunnelConnected {
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
