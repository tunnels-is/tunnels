package tunnel

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/crypto"
	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
	"github.com/tunnels-is/tunnels/coretwo/pkg/metrics"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	readData  []byte
	writeData []byte
	closed    bool
}

func newMockConn() *mockConn {
	return &mockConn{}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if len(m.readData) == 0 {
		return 0, nil
	}
	n = copy(b, m.readData)
	m.readData = m.readData[n:]
	return n, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestNewService(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Error("NewService returned nil")
	}
	if service.tunnels == nil {
		t.Error("tunnels map is nil")
	}
}

func TestService_Connect(t *testing.T) {
	service := NewService()
	config := &TunnelConfig{
		ServerIP:   "127.0.0.1",
		ServerPort: 8080,
		Protocol:   "tcp",
		Encryption: struct {
			Enabled bool
			Key     string
		}{
			Enabled: true,
			Key:     "test-key",
		},
	}

	err := service.Connect("test-tunnel", config)
	if err != nil {
		t.Errorf("Connect failed: %v", err)
	}

	tunnel, err := service.GetTunnel("test-tunnel")
	if err != nil {
		t.Errorf("GetTunnel failed: %v", err)
	}
	if tunnel.ID != "test-tunnel" {
		t.Errorf("Expected tunnel ID 'test-tunnel', got '%s'", tunnel.ID)
	}
	if !tunnel.encryption.enabled {
		t.Error("Expected encryption to be enabled")
	}
}

func TestService_Disconnect(t *testing.T) {
	service := NewService()
	config := &TunnelConfig{
		ServerIP:   "127.0.0.1",
		ServerPort: 8080,
		Protocol:   "tcp",
	}

	// Connect a tunnel
	err := service.Connect("test-tunnel", config)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Disconnect the tunnel
	err = service.Disconnect("test-tunnel")
	if err != nil {
		t.Errorf("Disconnect failed: %v", err)
	}

	// Verify tunnel is removed
	_, err = service.GetTunnel("test-tunnel")
	if err == nil {
		t.Error("Expected error when getting disconnected tunnel")
	}
}

func TestService_StartStop(t *testing.T) {
	service := NewService()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start service
	err := service.Start(ctx)
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}

	// Stop service
	err = service.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestTunnel_ConnectClose(t *testing.T) {
	tunnel := &Tunnel{
		ID:         "test-tunnel",
		ServerIP:   "127.0.0.1",
		ServerPort: 8080,
		Protocol:   "tcp",
		log:        logger.Default(),
	}

	// Connect should fail since there's no server
	err := tunnel.Connect()
	if err == nil {
		t.Error("Expected Connect to fail with no server")
	}

	// Close should succeed even if not connected
	err = tunnel.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestTunnel_UpdateStats(t *testing.T) {
	log := logger.Default()
	metrics := metrics.NewRegistry(log, time.Second)

	tunnel := &Tunnel{
		ID:        "test-tunnel",
		Connected: true,
		log:       log,
		metrics:   metrics,
	}

	// Register metrics
	metrics.RegisterCounter("tunnel_bytes_in", "Bytes received", nil)
	metrics.RegisterCounter("tunnel_bytes_out", "Bytes sent", nil)
	metrics.RegisterCounter("tunnel_packets_in", "Packets received", nil)
	metrics.RegisterCounter("tunnel_packets_out", "Packets sent", nil)
	metrics.RegisterGauge("tunnel_latency", "Latency", nil)
	metrics.RegisterCounter("tunnel_encryption_errors", "Encryption errors", nil)
	metrics.RegisterCounter("tunnel_decryption_errors", "Decryption errors", nil)

	// Set some stats
	tunnel.Stats.BytesIn = 100
	tunnel.Stats.BytesOut = 200
	tunnel.Stats.PacketsIn = 10
	tunnel.Stats.PacketsOut = 20
	tunnel.Stats.Latency = 50 * time.Millisecond
	tunnel.Stats.Encryption.EncryptionErrors = 1
	tunnel.Stats.Encryption.DecryptionErrors = 2

	// Update stats
	tunnel.updateStats()

	// Verify metrics were updated
	if bytesIn := metrics.GetCounter("tunnel_bytes_in"); bytesIn != nil {
		if bytesIn.Value != float64(100) {
			t.Errorf("Expected bytes_in to be 100, got %f", bytesIn.Value)
		}
	}
	if bytesOut := metrics.GetCounter("tunnel_bytes_out"); bytesOut != nil {
		if bytesOut.Value != float64(200) {
			t.Errorf("Expected bytes_out to be 200, got %f", bytesOut.Value)
		}
	}
}

func TestTunnel_ProcessPacket(t *testing.T) {
	log := logger.Default()
	metrics := metrics.NewRegistry(log, time.Second)

	tunnel := &Tunnel{
		ID:        "test-tunnel",
		Connected: true,
		log:       log,
		metrics:   metrics,
	}

	// Enable encryption
	tunnel.encryption.enabled = true
	tunnel.encryption.keyString = "test-key"
	key, err := crypto.NewKey("test-key")
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}
	tunnel.encryption.key = key

	// Test data
	testData := []byte("Hello, World!")

	// Test outgoing packet encryption
	t.Run("Outgoing Packet Encryption", func(t *testing.T) {
		packet := &Packet{
			Data:      testData,
			Direction: PacketOutgoing,
			Timestamp: time.Now(),
		}

		err := tunnel.processPacket(packet)
		if err != nil {
			t.Fatalf("Failed to process outgoing packet: %v", err)
		}

		// Verify packet was encrypted
		if len(packet.Data) <= len(testData) {
			t.Error("Expected encrypted data to be longer than plaintext")
		}

		// Try to decrypt the data
		decrypted, err := key.Decrypt(packet.Data)
		if err != nil {
			t.Fatalf("Failed to decrypt data: %v", err)
		}
		if string(decrypted) != string(testData) {
			t.Errorf("Expected decrypted data to be '%s', got '%s'", testData, decrypted)
		}
	})

	// Test incoming packet decryption
	t.Run("Incoming Packet Decryption", func(t *testing.T) {
		// First encrypt some data
		encrypted, err := key.Encrypt(testData)
		if err != nil {
			t.Fatalf("Failed to encrypt data: %v", err)
		}

		packet := &Packet{
			Data:      encrypted,
			Direction: PacketIncoming,
			Timestamp: time.Now(),
		}

		err = tunnel.processPacket(packet)
		if err != nil {
			t.Fatalf("Failed to process incoming packet: %v", err)
		}

		// Verify packet was decrypted correctly
		if string(packet.Data) != string(testData) {
			t.Errorf("Expected decrypted data to be '%s', got '%s'", testData, packet.Data)
		}
	})

	// Test invalid packet direction
	t.Run("Invalid Packet Direction", func(t *testing.T) {
		packet := &Packet{
			Data:      testData,
			Direction: PacketDirection(999), // Invalid direction
			Timestamp: time.Now(),
		}

		err := tunnel.processPacket(packet)
		if err == nil {
			t.Error("Expected error for invalid packet direction")
		}
	})

	// Test encryption error handling
	t.Run("Encryption Error Handling", func(t *testing.T) {
		packet := &Packet{
			Data:      []byte("invalid encrypted data"),
			Direction: PacketIncoming,
			Timestamp: time.Now(),
		}

		err := tunnel.processPacket(packet)
		if err == nil {
			t.Error("Expected error when processing invalid encrypted data")
		}

		if tunnel.Stats.Encryption.DecryptionErrors == 0 {
			t.Error("Expected decryption error to be counted")
		}
	})
}

func TestTunnel_PacketHandling(t *testing.T) {
	log := logger.Default()
	metrics := metrics.NewRegistry(log, time.Second)

	tunnel := &Tunnel{
		ID:        "test-tunnel",
		Connected: true,
		log:       log,
		metrics:   metrics,
	}

	// Initialize channels
	tunnel.packetChan = make(chan *Packet, 100)
	tunnel.stopChan = make(chan struct{})

	// Create mock connection
	mockConn := newMockConn()

	// Test packet reading
	t.Run("Read Packets", func(t *testing.T) {
		// Set up test data
		testData := []byte("test packet")
		mockConn.readData = testData

		// Start packet reader
		go tunnel.readPackets(mockConn)

		// Wait for packet to be processed
		time.Sleep(100 * time.Millisecond)

		// Verify packet was read
		select {
		case packet := <-tunnel.packetChan:
			if string(packet.Data) != string(testData) {
				t.Errorf("Expected packet data '%s', got '%s'", testData, packet.Data)
			}
			if packet.Direction != PacketIncoming {
				t.Error("Expected incoming packet direction")
			}
		default:
			t.Error("No packet received")
		}
	})

	// Test packet writing
	t.Run("Write Packets", func(t *testing.T) {
		// Start packet writer
		go tunnel.writePackets(mockConn)

		// Send test packet
		testData := []byte("test packet")
		tunnel.packetChan <- &Packet{
			Data:      testData,
			Direction: PacketOutgoing,
			Timestamp: time.Now(),
		}

		// Wait for packet to be processed
		time.Sleep(100 * time.Millisecond)

		// Verify packet was written
		if string(mockConn.writeData) != string(testData) {
			t.Errorf("Expected written data '%s', got '%s'", testData, mockConn.writeData)
		}
	})

	// Clean up
	close(tunnel.stopChan)
}
