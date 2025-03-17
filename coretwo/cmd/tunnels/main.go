package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tunnels-is/tunnels/coretwo/internal/api"
	"github.com/tunnels-is/tunnels/coretwo/internal/config"
	"github.com/tunnels-is/tunnels/coretwo/internal/platform"
	"github.com/tunnels-is/tunnels/coretwo/pkg/dns"
	iface "github.com/tunnels-is/tunnels/coretwo/pkg/interface"
	"github.com/tunnels-is/tunnels/coretwo/pkg/logger"
	"github.com/tunnels-is/tunnels/coretwo/pkg/tunnel"
)

var (
	basePath   = flag.String("basePath", "", "Base directory for logs and configs")
	configPath = flag.String("config", "", "Path to config file")
	logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	jsonLogs   = flag.Bool("json-logs", false, "Output logs in JSON format")
)

func main() {
	flag.Parse()

	// Initialize logger
	log := logger.Default()
	if *jsonLogs {
		log = logger.New(getLogLevel(*logLevel), os.Stdout, true)
	} else {
		log = logger.New(getLogLevel(*logLevel), os.Stdout, false)
	}

	// Initialize context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize core components
	if err := initialize(ctx, log); err != nil {
		log.Error("Failed to initialize", map[string]any{"error": err})
		os.Exit(1)
	}

	// Start the tunnel service
	tunnelService := tunnel.NewService()
	if err := tunnelService.Start(ctx); err != nil {
		log.Error("Failed to start tunnel service", map[string]any{"error": err})
		os.Exit(1)
	}

	// Start the API server
	apiServer := api.NewServer(tunnelService, &api.Config{
		Host: "127.0.0.1",
		Port: "8080",
	})
	if err := apiServer.Start(ctx); err != nil {
		log.Error("Failed to start API server", map[string]any{"error": err})
		os.Exit(1)
	}

	log.Info("Service started successfully")

	// Wait for shutdown signal
	<-sigChan
	log.Info("Shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop services with timeout context
	if err := tunnelService.Stop(); err != nil {
		log.Error("Error stopping tunnel service", map[string]any{"error": err})
	}

	if err := apiServer.Stop(); err != nil {
		log.Error("Error stopping API server", map[string]any{"error": err})
	}

	// Wait for shutdown timeout or completion
	<-shutdownCtx.Done()
	log.Info("Shutdown complete")
}

func initialize(ctx context.Context, log *logger.Logger) error {
	// Initialize platform-specific components
	if err := platform.Initialize(ctx); err != nil {
		return fmt.Errorf("platform initialization failed: %w", err)
	}

	// Load configuration
	cfg, err := config.Load(*configPath, *basePath)
	if err != nil {
		return fmt.Errorf("config loading failed: %w", err)
	}

	// Initialize platform-specific network interfaces
	if err := platform.InitializeNetwork(cfg); err != nil {
		return fmt.Errorf("network initialization failed: %w", err)
	}

	// Initialize interface manager
	ifaceManager := iface.NewManager()
	if err := ifaceManager.RefreshInterfaces(); err != nil {
		return fmt.Errorf("interface refresh failed: %w", err)
	}

	// Initialize DNS resolver with default servers
	_ = dns.NewResolver([]string{"8.8.8.8:53", "1.1.1.1:53"})

	return nil
}

func getLogLevel(level string) logger.Level {
	switch level {
	case "debug":
		return logger.DebugLevel
	case "info":
		return logger.InfoLevel
	case "warn":
		return logger.WarnLevel
	case "error":
		return logger.ErrorLevel
	default:
		return logger.InfoLevel
	}
}
