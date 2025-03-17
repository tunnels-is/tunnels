package tunnel

import (
	"net"
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
	"github.com/tunnels-is/tunnels/coretwo/pkg/metrics"
)

func TestPacketProcessing(t *testing.T) {
	// Create a test server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	defer listener.Close()

	// Create a new tunnel
	logger := logger.New(logger.DebugLevel, nil, false)
	tunnel := &Tunnel{
		ID:         "test-tunnel",
		ServerIP:   "127.0.0.1",
		ServerPort: 8080,
		Protocol:   "tcp",
		log:        logger,
		metrics:    metrics.NewRegistry(logger, time.Second),
	}

	// Start the test server
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Errorf("Failed to accept connection: %v", err)
			return
		}
		defer conn.Close()

		// Send test data
		testData := []byte("test packet")
		if _, err := conn.Write(testData); err != nil {
			t.Errorf("Failed to write test data: %v", err)
		}
	}()

	// Connect the tunnel
	if err := tunnel.Connect(); err != nil {
		t.Fatalf("Failed to connect tunnel: %v", err)
	}

	// Wait for packet processing
	time.Sleep(100 * time.Millisecond)

	// Check tunnel stats
	if tunnel.Stats.BytesIn == 0 {
		t.Error("Expected bytes_in to be non-zero")
	}
	if tunnel.Stats.PacketsIn == 0 {
		t.Error("Expected packets_in to be non-zero")
	}

	// Close the tunnel
	if err := tunnel.Close(); err != nil {
		t.Fatalf("Failed to close tunnel: %v", err)
	}
}

func TestPacketChannels(t *testing.T) {
	// Create a new tunnel
	logger := logger.New(logger.DebugLevel, nil, false)
	tunnel := &Tunnel{
		ID:         "test-tunnel",
		ServerIP:   "127.0.0.1",
		ServerPort: 8080,
		Protocol:   "tcp",
		log:        logger,
		metrics:    metrics.NewRegistry(logger, time.Second),
	}

	// Initialize channels
	tunnel.packetChan = make(chan *Packet, 1000)
	tunnel.stopChan = make(chan struct{})

	// Test packet channel capacity
	testData := []byte("test packet")
	packet := &Packet{
		Data:      testData,
		Direction: PacketIncoming,
		Timestamp: time.Now(),
	}

	// Fill the channel
	for i := 0; i < 1000; i++ {
		select {
		case tunnel.packetChan <- packet:
			// Success
		default:
			t.Fatal("Failed to write to packet channel")
		}
	}

	// Try to write one more packet (should fail)
	select {
	case tunnel.packetChan <- packet:
		t.Error("Expected packet channel to be full")
	default:
		// Success
	}

	// Close the tunnel
	close(tunnel.stopChan)
	close(tunnel.packetChan)
}

func TestPacketDirection(t *testing.T) {
	// Test packet direction constants
	if PacketIncoming != 0 {
		t.Error("Expected PacketIncoming to be 0")
	}
	if PacketOutgoing != 1 {
		t.Error("Expected PacketOutgoing to be 1")
	}

	// Test packet direction string representation
	if PacketIncoming.String() != "incoming" {
		t.Errorf("Expected PacketIncoming.String() to be 'incoming', got '%s'", PacketIncoming.String())
	}
	if PacketOutgoing.String() != "outgoing" {
		t.Errorf("Expected PacketOutgoing.String() to be 'outgoing', got '%s'", PacketOutgoing.String())
	}
}
