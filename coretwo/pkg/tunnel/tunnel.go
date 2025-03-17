package tunnel

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/crypto"
)

// Connect establishes a tunnel connection
func (t *Tunnel) Connect() error {
	t.log.Info("Connecting to server", map[string]any{
		"server": fmt.Sprintf("%s:%d", t.ServerIP, t.ServerPort),
	})

	// Initialize packet channels
	t.packetChan = make(chan *Packet, 1000)
	t.stopChan = make(chan struct{})

	// Initialize encryption if enabled
	if t.encryption.enabled {
		key, err := crypto.NewKey(t.encryption.keyString)
		if err != nil {
			t.log.Error("Failed to initialize encryption", map[string]any{
				"error": err,
			})
			return fmt.Errorf("failed to initialize encryption: %w", err)
		}
		t.encryption.key = key
	}

	// Create network connection
	conn, err := net.Dial(t.Protocol, fmt.Sprintf("%s:%d", t.ServerIP, t.ServerPort))
	if err != nil {
		t.log.Error("Failed to connect to server", map[string]any{
			"error": err,
		})
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	// Set connection options
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(true); err != nil {
			t.log.Warn("Failed to set TCP keepalive", map[string]any{
				"error": err,
			})
		}
		if err := tcpConn.SetKeepAlivePeriod(30 * time.Second); err != nil {
			t.log.Warn("Failed to set TCP keepalive period", map[string]any{
				"error": err,
			})
		}
	}

	t.Connected = true
	t.LastPing = time.Now()

	// Start packet handling
	go t.handlePackets()
	go t.readPackets(conn)
	go t.writePackets(conn)

	return nil
}

// Close closes the tunnel connection
func (t *Tunnel) Close() error {
	t.log.Info("Closing tunnel connection")

	// Signal stop
	close(t.stopChan)

	// Close packet channel
	close(t.packetChan)

	t.Connected = false
	return nil
}

// updateStats updates tunnel statistics
func (t *Tunnel) updateStats() {
	// Update packet and byte counters
	if bytesIn := t.metrics.GetCounter("tunnel_bytes_in"); bytesIn != nil {
		bytesIn.Add(float64(t.Stats.BytesIn))
	}
	if bytesOut := t.metrics.GetCounter("tunnel_bytes_out"); bytesOut != nil {
		bytesOut.Add(float64(t.Stats.BytesOut))
	}
	if packetsIn := t.metrics.GetCounter("tunnel_packets_in"); packetsIn != nil {
		packetsIn.Add(float64(t.Stats.PacketsIn))
	}
	if packetsOut := t.metrics.GetCounter("tunnel_packets_out"); packetsOut != nil {
		packetsOut.Add(float64(t.Stats.PacketsOut))
	}

	// Update latency gauge
	if latency := t.metrics.GetGauge("tunnel_latency"); latency != nil {
		latency.Set(float64(t.Stats.Latency.Milliseconds()))
	}

	// Update encryption error counters
	if t.encryption.enabled {
		if encErrors := t.metrics.GetCounter("tunnel_encryption_errors"); encErrors != nil {
			encErrors.Add(float64(t.Stats.Encryption.EncryptionErrors))
		}
		if decErrors := t.metrics.GetCounter("tunnel_decryption_errors"); decErrors != nil {
			decErrors.Add(float64(t.Stats.Encryption.DecryptionErrors))
		}
	}

	t.Stats.LastUpdated = time.Now()
}

// handlePackets handles incoming and outgoing packets
func (t *Tunnel) handlePackets() {
	t.log.Info("Starting packet handler")

	for {
		select {
		case <-t.stopChan:
			t.log.Info("Packet handler stopped")
			return
		case packet := <-t.packetChan:
			if err := t.processPacket(packet); err != nil {
				t.log.Error("Failed to process packet", map[string]any{
					"error":     err,
					"direction": packet.Direction,
				})
				t.Stats.Errors++
			}
		}
	}
}

// readPackets reads packets from the network connection
func (t *Tunnel) readPackets(conn net.Conn) {
	buffer := make([]byte, 65536) // Maximum UDP packet size
	for {
		select {
		case <-t.stopChan:
			return
		default:
			n, err := conn.Read(buffer)
			if err != nil {
				if err != io.EOF {
					t.log.Error("Failed to read packet", map[string]any{
						"error": err,
					})
				}
				return
			}

			packet := &Packet{
				Data:      make([]byte, n),
				Direction: PacketIncoming,
				Timestamp: time.Now(),
			}
			copy(packet.Data, buffer[:n])

			select {
			case t.packetChan <- packet:
				t.Stats.BytesIn += uint64(n)
				t.Stats.PacketsIn++
			default:
				t.log.Warn("Packet channel full, dropping packet")
			}
		}
	}
}

// writePackets writes packets to the network connection
func (t *Tunnel) writePackets(conn net.Conn) {
	for {
		select {
		case <-t.stopChan:
			return
		case packet := <-t.packetChan:
			if packet.Direction != PacketOutgoing {
				continue
			}

			n, err := conn.Write(packet.Data)
			if err != nil {
				t.log.Error("Failed to write packet", map[string]any{
					"error": err,
				})
				continue
			}

			t.Stats.BytesOut += uint64(n)
			t.Stats.PacketsOut++
		}
	}
}

// processPacket processes a single packet
func (t *Tunnel) processPacket(packet *Packet) error {
	// Update last ping time
	t.LastPing = time.Now()

	// Process packet based on direction
	switch packet.Direction {
	case PacketIncoming:
		if t.encryption.enabled {
			// Decrypt incoming packet
			decrypted, err := t.encryption.key.Decrypt(packet.Data)
			if err != nil {
				t.log.Error("Failed to decrypt packet", map[string]any{
					"error": err,
				})
				t.Stats.Encryption.DecryptionErrors++
				return fmt.Errorf("failed to decrypt packet: %w", err)
			}
			packet.Data = decrypted
		}

		// TODO: Implement protocol-specific handling
		// TODO: Implement routing to appropriate destination
		return nil

	case PacketOutgoing:
		if t.encryption.enabled {
			// Encrypt outgoing packet
			encrypted, err := t.encryption.key.Encrypt(packet.Data)
			if err != nil {
				t.log.Error("Failed to encrypt packet", map[string]any{
					"error": err,
				})
				t.Stats.Encryption.EncryptionErrors++
				return fmt.Errorf("failed to encrypt packet: %w", err)
			}
			packet.Data = encrypted
		}

		// TODO: Implement protocol-specific handling
		// TODO: Implement routing to appropriate destination
		return nil

	default:
		return fmt.Errorf("invalid packet direction: %v", packet.Direction)
	}
}
