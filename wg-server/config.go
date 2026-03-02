package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tunnels-is/tunnels/types"
	"golang.org/x/crypto/curve25519"
)

// Config holds all wg-server runtime configuration.
// Bootstrap fields (ControllerURL, APIKey) come from CLI flags.
// All operational fields are populated by FetchConfig from the controller.
type Config struct {
	// Bootstrap fields (from CLI flags)
	ControllerURL string
	APIKey        string

	// ServerID is the hex ObjectID of this server's record in the controller DB.
	ServerID string

	// AdminAPIKey is the controller's admin key used to call /v3/wg/peers etc.
	AdminAPIKey string

	WireGuardPort    int
	WireGuardPrivKey string
	WireGuardSubnet  string
	WireGuardIface   string

	InternetIface string

	LogLevel string
	LogJSON  bool
	Silent   bool

	InsecureSkipVerify bool
	PacketInspection   bool
}

// FetchConfig fetches the operational configuration from the controller using
// the per-server APIKey. It calls GET /v3/wg/server-config/fetch with the
// X-WG-KEY header and maps the response to a Config.
func FetchConfig(controllerURL, apiKey string, insecureSkipVerify bool) (*Config, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify,
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	url := controllerURL + "/v3/wg/server-config/fetch"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-WG-KEY", apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("controller returned %d", resp.StatusCode)
	}

	var r types.WGServerConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	cfg := &Config{
		ControllerURL:    controllerURL,
		APIKey:           apiKey,
		ServerID:         r.ServerID,
		AdminAPIKey:      r.AdminAPIKey,
		WireGuardPort:    r.WireGuardPort,
		WireGuardPrivKey: r.WireGuardPrivKey,
		WireGuardSubnet:  r.WireGuardSubnet,
		WireGuardIface:   r.WireGuardIface,
		InternetIface:    r.InternetIface,
		PacketInspection: r.PacketInspection,
		InsecureSkipVerify: insecureSkipVerify,
	}

	// Apply defaults for any zero values
	if cfg.WireGuardPort == 0 {
		cfg.WireGuardPort = 51820
	}
	if cfg.WireGuardIface == "" {
		cfg.WireGuardIface = "wg0"
	}
	if cfg.WireGuardSubnet == "" {
		cfg.WireGuardSubnet = "10.1.0.0/16"
	}

	return cfg, nil
}

// derivePubKey derives the Curve25519 public key from a base64-encoded private key.
func derivePubKey(privKeyB64 string) (string, error) {
	privBytes, err := base64.StdEncoding.DecodeString(privKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}
	if len(privBytes) != 32 {
		return "", fmt.Errorf("private key must be 32 bytes, got %d", len(privBytes))
	}
	pubBytes, err := curve25519.X25519(privBytes, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("derive public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pubBytes), nil
}
