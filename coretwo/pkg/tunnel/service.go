package tunnel

import (
	"context"
	"fmt"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
	"github.com/tunnels-is/tunnels/coretwo/pkg/metrics"
)

// NewService creates a new tunnel service
func NewService() *Service {
	ctx, cancel := context.WithCancel(context.Background())
	logger := logger.Default()

	service := &Service{
		tunnels: make(map[string]*Tunnel),
		ctx:     ctx,
		cancel:  cancel,
		log:     logger,
		metrics: metrics.NewRegistry(logger, 5*time.Second),
	}

	// Register service-level metrics
	service.metrics.RegisterCounter("tunnels_total", "Total number of tunnels created", nil)
	service.metrics.RegisterCounter("tunnels_active", "Number of currently active tunnels", nil)
	service.metrics.RegisterGauge("tunnels_uptime", "Service uptime in seconds", nil)
	service.metrics.RegisterHistogram("tunnel_connection_time", "Time taken to establish tunnel connections", nil)

	// Start metrics collection
	service.metrics.Start()

	return service
}

// Start starts the tunnel service
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Info("Starting tunnel service")

	// Start background tasks
	go s.monitorTunnels(ctx)
	go s.collectStats(ctx)

	// Update uptime metric
	go func() {
		startTime := time.Now()
		uptime := s.metrics.RegisterGauge("service_uptime", "Service uptime in seconds", nil)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				uptime.Set(time.Since(startTime).Seconds())
			}
		}
	}()

	return nil
}

// Stop stops the tunnel service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Info("Stopping tunnel service")
	s.cancel()

	// Stop metrics collection
	s.metrics.Stop()

	// Close all tunnels
	for _, t := range s.tunnels {
		if err := t.Close(); err != nil {
			s.log.Error("Failed to close tunnel", map[string]any{
				"tunnel_id": t.ID,
				"error":     err,
			})
			return fmt.Errorf("failed to close tunnel %s: %w", t.ID, err)
		}
	}

	return nil
}

// Connect establishes a new tunnel connection
func (s *Service) Connect(tunnelID string, config *TunnelConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Info("Connecting tunnel", map[string]any{
		"tunnel_id": tunnelID,
		"server":    fmt.Sprintf("%s:%d", config.ServerIP, config.ServerPort),
	})

	startTime := time.Now()
	tunnel := &Tunnel{
		ID:         tunnelID,
		ServerIP:   config.ServerIP,
		ServerPort: config.ServerPort,
		Protocol:   config.Protocol,
		log:        s.log.WithFields(map[string]any{"tunnel_id": tunnelID}),
		metrics:    s.metrics,
	}

	// Initialize encryption configuration
	tunnel.encryption.enabled = config.Encryption.Enabled
	if config.Encryption.Enabled {
		tunnel.encryption.keyString = config.Encryption.Key
	}

	// Register tunnel-specific metrics
	tunnel.metrics.RegisterCounter("tunnel_bytes_in", "Total bytes received", map[string]string{"tunnel_id": tunnelID})
	tunnel.metrics.RegisterCounter("tunnel_bytes_out", "Total bytes sent", map[string]string{"tunnel_id": tunnelID})
	tunnel.metrics.RegisterCounter("tunnel_packets_in", "Total packets received", map[string]string{"tunnel_id": tunnelID})
	tunnel.metrics.RegisterCounter("tunnel_packets_out", "Total packets sent", map[string]string{"tunnel_id": tunnelID})
	tunnel.metrics.RegisterGauge("tunnel_latency", "Current tunnel latency", map[string]string{"tunnel_id": tunnelID})

	if config.Encryption.Enabled {
		tunnel.metrics.RegisterCounter("tunnel_encryption_errors", "Total encryption errors", map[string]string{"tunnel_id": tunnelID})
		tunnel.metrics.RegisterCounter("tunnel_decryption_errors", "Total decryption errors", map[string]string{"tunnel_id": tunnelID})
	}

	if err := tunnel.Connect(); err != nil {
		s.log.Error("Failed to connect tunnel", map[string]any{
			"tunnel_id": tunnelID,
			"error":     err,
		})
		return fmt.Errorf("failed to connect tunnel: %w", err)
	}

	// Update metrics
	if counter := s.metrics.GetCounter("tunnels_total"); counter != nil {
		counter.Inc()
	}
	if gauge := s.metrics.GetGauge("tunnels_active"); gauge != nil {
		gauge.Set(float64(len(s.tunnels) + 1))
	}
	if hist := s.metrics.GetHistogram("tunnel_connection_time"); hist != nil {
		hist.Add(time.Since(startTime).Seconds())
	}

	s.tunnels[tunnelID] = tunnel
	s.log.Info("Tunnel connected successfully", map[string]any{"tunnel_id": tunnelID})
	return nil
}

// Disconnect closes a tunnel connection
func (s *Service) Disconnect(tunnelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Info("Disconnecting tunnel", map[string]any{"tunnel_id": tunnelID})

	tunnel, exists := s.tunnels[tunnelID]
	if !exists {
		s.log.Warn("Tunnel not found", map[string]any{"tunnel_id": tunnelID})
		return fmt.Errorf("tunnel %s not found", tunnelID)
	}

	if err := tunnel.Close(); err != nil {
		s.log.Error("Failed to close tunnel", map[string]any{
			"tunnel_id": tunnelID,
			"error":     err,
		})
		return fmt.Errorf("failed to close tunnel: %w", err)
	}

	delete(s.tunnels, tunnelID)
	if gauge := s.metrics.GetGauge("tunnels_active"); gauge != nil {
		gauge.Set(float64(len(s.tunnels)))
	}
	s.log.Info("Tunnel disconnected successfully", map[string]any{"tunnel_id": tunnelID})
	return nil
}

// GetTunnel returns a tunnel by ID
func (s *Service) GetTunnel(tunnelID string) (*Tunnel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tunnel, exists := s.tunnels[tunnelID]
	if !exists {
		s.log.Warn("Tunnel not found", map[string]any{"tunnel_id": tunnelID})
		return nil, fmt.Errorf("tunnel %s not found", tunnelID)
	}

	return tunnel, nil
}

// monitorTunnels monitors tunnel health and reconnects if needed
func (s *Service) monitorTunnels(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			for _, t := range s.tunnels {
				if !t.Connected || time.Since(t.LastPing) > 2*time.Minute {
					s.log.Warn("Tunnel needs reconnection", map[string]any{
						"tunnel_id": t.ID,
						"connected": t.Connected,
						"last_ping": t.LastPing,
					})
					go s.reconnectTunnel(t)
				}
			}
			s.mu.RUnlock()
		}
	}
}

// collectStats collects statistics from all tunnels
func (s *Service) collectStats(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.RLock()
			for _, t := range s.tunnels {
				if t.Connected {
					t.updateStats()
				}
			}
			s.mu.RUnlock()
		}
	}
}

// reconnectTunnel attempts to reconnect a tunnel
func (s *Service) reconnectTunnel(t *Tunnel) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t.log.Info("Attempting tunnel reconnection")

	if err := t.Close(); err != nil {
		t.log.Error("Failed to close tunnel during reconnection", map[string]any{
			"error": err,
		})
	}

	if err := t.Connect(); err != nil {
		t.log.Error("Failed to reconnect tunnel", map[string]any{
			"error": err,
		})
	} else {
		t.log.Info("Tunnel reconnected successfully")
	}
}
