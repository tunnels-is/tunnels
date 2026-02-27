package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/curve25519"
	"gopkg.in/yaml.v3"
)

// Config holds all wg-server configuration.
type Config struct {
	ControllerURL string `json:"ControllerURL" yaml:"ControllerURL"`
	AdminAPIKey   string `json:"AdminAPIKey" yaml:"AdminAPIKey"`

	WireGuardPort    int    `json:"WireGuardPort" yaml:"WireGuardPort"`
	WireGuardPrivKey string `json:"WireGuardPrivKey" yaml:"WireGuardPrivKey"`
	WireGuardSubnet  string `json:"WireGuardSubnet" yaml:"WireGuardSubnet"`
	WireGuardIface   string `json:"WireGuardIface" yaml:"WireGuardIface"`

	// InternetIface is the outbound NIC for iptables MASQUERADE (e.g. "eth0").
	InternetIface string `json:"InternetIface" yaml:"InternetIface"`

	// SyncIntervalSecs controls how often peers are fetched from the controller.
	SyncIntervalSecs int `json:"SyncIntervalSecs" yaml:"SyncIntervalSecs"`

	LogLevel string `json:"LogLevel" yaml:"LogLevel"`
	LogJSON  bool   `json:"LogJSON" yaml:"LogJSON"`
	Silent   bool   `json:"Silent" yaml:"Silent"`

	// InsecureSkipVerify disables TLS cert verification for the controller.
	// Set to true only when using self-signed certificates in dev/test.
	InsecureSkipVerify bool `json:"InsecureSkipVerify" yaml:"InsecureSkipVerify"`

	// SyncListenAddr is the local HTTP address for the instant-sync endpoint.
	// The controller POSTs to <SyncListenAddr>/v3/wg/sync after a new peer
	// registers, triggering an immediate SyncPeers() call. Defaults to
	// "127.0.0.1:8181". Set to "" to disable.
	SyncListenAddr string `json:"SyncListenAddr" yaml:"SyncListenAddr"`
}

func defaultConfig() *Config {
	return &Config{
		WireGuardPort:    51820,
		WireGuardSubnet:  "10.1.0.0/16",
		WireGuardIface:   "wg0",
		SyncIntervalSecs: 30,
		LogLevel:         "debug",
		SyncListenAddr:   "127.0.0.1:8181",
	}
}

// LoadConfig reads a JSON or YAML config file.
func LoadConfig(path string) (*Config, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, cfg)
	default:
		err = json.Unmarshal(data, cfg)
	}
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes cfg to path as pretty JSON.
func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// generatePrivKey creates a new Curve25519 private key and returns it base64-encoded.
func generatePrivKey() string {
	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		panic("rand.Read failed: " + err.Error())
	}
	// Clamp for Curve25519
	privKey[0] &= 248
	privKey[31] = (privKey[31] & 127) | 64
	return base64.StdEncoding.EncodeToString(privKey)
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
