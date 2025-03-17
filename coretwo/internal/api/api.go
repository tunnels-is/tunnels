package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/tunnels-is/tunnels/coretwo/pkg/tunnel"
)

// Server represents the API server
type Server struct {
	mu            sync.RWMutex
	tunnelService *tunnel.Service
	httpServer    *http.Server
	config        *Config
}

// Config contains API server configuration
type Config struct {
	Host string
	Port string
}

// NewServer creates a new API server
func NewServer(tunnelService *tunnel.Service, config *Config) *Server {
	return &Server{
		tunnelService: tunnelService,
		config:        config,
	}
}

// Start starts the API server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/api/v1/tunnels", s.handleTunnels)
	mux.HandleFunc("/api/v1/tunnels/", s.handleTunnel)
	mux.HandleFunc("/api/v1/status", s.handleStatus)

	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%s", s.config.Host, s.config.Port),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the API server
func (s *Server) Stop() error {
	if s.httpServer != nil {
		return s.httpServer.Close()
	}
	return nil
}

// handleTunnels handles tunnel list and creation
func (s *Server) handleTunnels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listTunnels(w, r)
	case http.MethodPost:
		s.createTunnel(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTunnel handles individual tunnel operations
func (s *Server) handleTunnel(w http.ResponseWriter, r *http.Request) {
	tunnelID := r.URL.Path[len("/api/v1/tunnels/"):]
	if tunnelID == "" {
		http.Error(w, "Tunnel ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getTunnel(w, r, tunnelID)
	case http.MethodDelete:
		s.deleteTunnel(w, r, tunnelID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStatus handles status requests
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := struct {
		Status    string `json:"status"`
		Version   string `json:"version"`
		Uptime    string `json:"uptime"`
		Connected int    `json:"connected_tunnels"`
	}{
		Status:    "running",
		Version:   "1.0.0",
		Uptime:    "0", // TODO: Implement uptime tracking
		Connected: 0,   // TODO: Implement tunnel counting
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// listTunnels returns a list of all tunnels
func (s *Server) listTunnels(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement tunnel listing
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]interface{}{})
}

// createTunnel creates a new tunnel
func (s *Server) createTunnel(w http.ResponseWriter, r *http.Request) {
	var config tunnel.TunnelConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// TODO: Generate tunnel ID
	tunnelID := "tunnel-1"

	if err := s.tunnelService.Connect(tunnelID, &config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id": tunnelID,
	})
}

// getTunnel returns information about a specific tunnel
func (s *Server) getTunnel(w http.ResponseWriter, r *http.Request, tunnelID string) {
	tunnel, err := s.tunnelService.GetTunnel(tunnelID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tunnel)
}

// deleteTunnel deletes a tunnel
func (s *Server) deleteTunnel(w http.ResponseWriter, r *http.Request, tunnelID string) {
	if err := s.tunnelService.Disconnect(tunnelID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
