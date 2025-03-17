package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
	"github.com/tunnels-is/tunnels/coretwo/pkg/tunnel"
)

func main() {
	// Parse command line flags
	serverIP := flag.String("server", "127.0.0.1", "Server IP address")
	serverPort := flag.Int("port", 8080, "Server port")
	protocol := flag.String("protocol", "tcp", "Protocol (tcp or udp)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	jsonLogs := flag.Bool("json-logs", false, "Output logs in JSON format")
	enableEncryption := flag.Bool("encrypt", false, "Enable encryption")
	encryptionKey := flag.String("key", "default-secure-key", "Encryption key")
	flag.Parse()

	// Initialize logger
	var level logger.Level
	switch *logLevel {
	case "debug":
		level = logger.DebugLevel
	case "info":
		level = logger.InfoLevel
	case "warn":
		level = logger.WarnLevel
	case "error":
		level = logger.ErrorLevel
	default:
		level = logger.InfoLevel
	}

	log := logger.New(level, nil, *jsonLogs)
	log.Info("Starting tunnel test client", map[string]any{
		"server":     *serverIP,
		"port":       *serverPort,
		"protocol":   *protocol,
		"encryption": *enableEncryption,
	})

	// Create tunnel service
	service := tunnel.NewService()

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signalCh
		log.Info("Received shutdown signal")
		cancel()
	}()

	// Start tunnel service
	if err := service.Start(ctx); err != nil {
		log.Error("Failed to start tunnel service", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}

	// Create tunnel configuration
	config := &tunnel.TunnelConfig{
		ServerIP:   *serverIP,
		ServerPort: *serverPort,
		Protocol:   *protocol,
		MTU:        1500,
		BufferSize: 4096,
	}

	// Configure encryption if enabled
	if *enableEncryption {
		config.Encryption.Enabled = true
		config.Encryption.Key = *encryptionKey
	}

	// Connect tunnel
	tunnelID := "test-tunnel"
	log.Info("Connecting tunnel", map[string]any{
		"tunnel_id": tunnelID,
	})

	if err := service.Connect(tunnelID, config); err != nil {
		log.Error("Failed to connect tunnel", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}

	log.Info("Tunnel connected successfully", map[string]any{
		"tunnel_id": tunnelID,
	})

	// Display tunnel information
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tunnel, err := service.GetTunnel(tunnelID)
				if err != nil {
					log.Error("Failed to get tunnel", map[string]any{
						"error": err,
					})
					continue
				}

				log.Info("Tunnel statistics", map[string]any{
					"tunnel_id":    tunnelID,
					"bytes_in":     tunnel.Stats.BytesIn,
					"bytes_out":    tunnel.Stats.BytesOut,
					"packets_in":   tunnel.Stats.PacketsIn,
					"packets_out":  tunnel.Stats.PacketsOut,
					"latency":      tunnel.Stats.Latency.String(),
					"enc_errors":   tunnel.Stats.Encryption.EncryptionErrors,
					"dec_errors":   tunnel.Stats.Encryption.DecryptionErrors,
					"reconnects":   tunnel.Stats.Reconnects,
					"last_updated": tunnel.Stats.LastUpdated.Format(time.RFC3339),
				})
			}
		}
	}()

	// Wait for context cancellation (signal)
	<-ctx.Done()

	// Disconnect tunnel
	log.Info("Disconnecting tunnel", map[string]any{
		"tunnel_id": tunnelID,
	})

	if err := service.Disconnect(tunnelID); err != nil {
		log.Error("Failed to disconnect tunnel", map[string]any{
			"error": err,
		})
	}

	// Stop service
	log.Info("Stopping tunnel service")
	if err := service.Stop(); err != nil {
		log.Error("Failed to stop tunnel service", map[string]any{
			"error": err,
		})
		os.Exit(1)
	}

	log.Info("Tunnel test client stopped")
}
