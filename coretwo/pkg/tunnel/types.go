package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/crypto"
	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
	"github.com/tunnels-is/tunnels/coretwo/pkg/metrics"
)

// Packet represents a network packet
type Packet struct {
	Data      []byte
	Direction PacketDirection
	Timestamp time.Time
}

// PacketDirection indicates the direction of packet flow
type PacketDirection int

const (
	PacketIncoming PacketDirection = iota
	PacketOutgoing
)

// String returns the string representation of the packet direction
func (d PacketDirection) String() string {
	switch d {
	case PacketIncoming:
		return "incoming"
	case PacketOutgoing:
		return "outgoing"
	default:
		return fmt.Sprintf("unknown(%d)", d)
	}
}

// PacketHandler is an interface for handling network packets
type PacketHandler interface {
	// HandlePacket processes a single packet
	HandlePacket(packet *Packet) error
	// Start starts the packet handler
	Start() error
	// Stop stops the packet handler
	Stop() error
}

// TunnelConfig contains configuration for a tunnel
type TunnelConfig struct {
	ServerIP   string
	ServerPort int
	Protocol   string
	MTU        int // Maximum Transmission Unit
	BufferSize int // Size of packet buffer
	Encryption struct {
		Enabled bool
		Key     string
	}
}

// Tunnel represents a single VPN tunnel
type Tunnel struct {
	ID         string
	Name       string
	ServerIP   string
	ServerPort int
	Protocol   string
	Connected  bool
	LastPing   time.Time
	Stats      TunnelStats
	log        *logger.Logger
	metrics    *metrics.Registry
	handler    PacketHandler
	packetChan chan *Packet
	stopChan   chan struct{}
	encryption struct {
		enabled   bool
		keyString string
		key       *crypto.Key
	}
}

// TunnelStats contains statistics for a tunnel
type TunnelStats struct {
	BytesIn     uint64
	BytesOut    uint64
	PacketsIn   uint64
	PacketsOut  uint64
	Latency     time.Duration
	LastUpdated time.Time
	Errors      uint64
	Reconnects  uint64
	Encryption  struct {
		EncryptionErrors uint64
		DecryptionErrors uint64
	}
}

// Service represents the VPN tunnel service
type Service struct {
	mu      sync.RWMutex
	tunnels map[string]*Tunnel
	config  *TunnelConfig
	ctx     context.Context
	cancel  context.CancelFunc
	log     *logger.Logger
	metrics *metrics.Registry
}
