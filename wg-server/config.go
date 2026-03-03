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

type Config struct {
	// Bootstrap fields (from CLI flags)
	ControllerURL string
	APIKey        string

	// ServerID is the hex ObjectID of this server's record in the controller DB.
	ServerID string

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

	url := controllerURL + "/wg/server-config/fetch"
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
		ControllerURL:      controllerURL,
		APIKey:             apiKey,
		ServerID:           r.ServerID,
		WireGuardPort:      r.WireGuardPort,
		WireGuardPrivKey:   r.WireGuardPrivKey,
		WireGuardSubnet:    r.WireGuardSubnet,
		WireGuardIface:     r.WireGuardIface,
		InternetIface:      r.InternetIface,
		PacketInspection:   r.PacketInspection,
		InsecureSkipVerify: insecureSkipVerify,
	}

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
