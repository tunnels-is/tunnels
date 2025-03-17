package tunnel

import (
	"context"
	"testing"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
	"github.com/tunnels-is/tunnels/coretwo/pkg/metrics"
)

func TestTunnelMetrics(t *testing.T) {
	// Create a new service
	service := NewService()
	if service == nil {
		t.Fatal("Failed to create service")
	}

	// Connect a tunnel
	config := &TunnelConfig{
		ServerIP:   "127.0.0.1",
		ServerPort: 8080,
		Protocol:   "tcp",
	}

	tunnelID := "test-tunnel"
	if err := service.Connect(tunnelID, config); err != nil {
		t.Fatalf("Failed to connect tunnel: %v", err)
	}

	// Get the tunnel
	tunnel, err := service.GetTunnel(tunnelID)
	if err != nil {
		t.Fatalf("Failed to get tunnel: %v", err)
	}

	// Wait for some metrics to be collected
	time.Sleep(2 * time.Second)

	// Check tunnel-specific metrics
	bytesIn := tunnel.metrics.GetCounter("tunnel_bytes_in")
	if bytesIn == nil {
		t.Error("bytes_in metric not found")
	}

	bytesOut := tunnel.metrics.GetCounter("tunnel_bytes_out")
	if bytesOut == nil {
		t.Error("bytes_out metric not found")
	}

	packetsIn := tunnel.metrics.GetCounter("tunnel_packets_in")
	if packetsIn == nil {
		t.Error("packets_in metric not found")
	}

	packetsOut := tunnel.metrics.GetCounter("tunnel_packets_out")
	if packetsOut == nil {
		t.Error("packets_out metric not found")
	}

	latency := tunnel.metrics.GetGauge("tunnel_latency")
	if latency == nil {
		t.Error("latency metric not found")
	}

	// Check service-level metrics
	tunnelsTotal := service.metrics.GetCounter("tunnels_total")
	if tunnelsTotal == nil {
		t.Error("tunnels_total metric not found")
	}
	if tunnelsTotal.Value.(float64) != 1 {
		t.Errorf("Expected tunnels_total to be 1, got %v", tunnelsTotal.Value)
	}

	tunnelsActive := service.metrics.GetGauge("tunnels_active")
	if tunnelsActive == nil {
		t.Error("tunnels_active metric not found")
	}
	if tunnelsActive.Value.(float64) != 1 {
		t.Errorf("Expected tunnels_active to be 1, got %v", tunnelsActive.Value)
	}

	// Disconnect the tunnel
	if err := service.Disconnect(tunnelID); err != nil {
		t.Fatalf("Failed to disconnect tunnel: %v", err)
	}

	// Check that active tunnels metric is updated
	if tunnelsActive.Value.(float64) != 0 {
		t.Errorf("Expected tunnels_active to be 0 after disconnect, got %v", tunnelsActive.Value)
	}

	// Stop the service
	if err := service.Stop(); err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}
}

func TestMetricsCollection(t *testing.T) {
	// Create a new service with a shorter collection interval for testing
	logger := logger.New(logger.DebugLevel, nil, false)
	ctx, cancel := context.WithCancel(context.Background())
	service := &Service{
		tunnels: make(map[string]*Tunnel),
		ctx:     ctx,
		cancel:  cancel,
		log:     logger,
		metrics: metrics.NewRegistry(logger, 100*time.Millisecond),
	}

	// Start metrics collection
	service.metrics.Start()

	// Register and connect a tunnel
	config := &TunnelConfig{
		ServerIP:   "127.0.0.1",
		ServerPort: 8080,
		Protocol:   "tcp",
	}

	tunnelID := "test-tunnel"
	if err := service.Connect(tunnelID, config); err != nil {
		t.Fatalf("Failed to connect tunnel: %v", err)
	}

	// Wait for metrics collection
	time.Sleep(300 * time.Millisecond)

	// Get the tunnel
	tunnel, err := service.GetTunnel(tunnelID)
	if err != nil {
		t.Fatalf("Failed to get tunnel: %v", err)
	}

	// Check that metrics are being updated
	bytesIn := tunnel.metrics.GetCounter("tunnel_bytes_in")
	if bytesIn == nil {
		t.Fatal("bytes_in metric not found")
	}
	if bytesIn.Value.(float64) == 0 {
		t.Error("Expected bytes_in to be non-zero")
	}

	bytesOut := tunnel.metrics.GetCounter("tunnel_bytes_out")
	if bytesOut == nil {
		t.Fatal("bytes_out metric not found")
	}
	if bytesOut.Value.(float64) == 0 {
		t.Error("Expected bytes_out to be non-zero")
	}

	// Stop the service
	if err := service.Stop(); err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}
}
