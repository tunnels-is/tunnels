package wgserver

import (
	"crypto/rand"
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
	WireGuardPrivKey []byte // raw 32-byte Curve25519 private key; zeroed after setup
	WireGuardSubnet  string
	WireGuardIface   string

	InternetIface string

	LogLevel string
	LogJSON  bool
	Silent   bool

	InsecureSkipVerify bool
	PacketInspection   bool
}

// generateWGPrivKey generates a new Curve25519 private key with proper clamping.
// Returns the raw 32-byte key as a []byte so it can be zeroed after use.
func generateWGPrivKey() ([]byte, error) {
	priv := make([]byte, 32)
	if _, err := rand.Read(priv); err != nil {
		return nil, fmt.Errorf("rand.Read failed: %w", err)
	}
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	return priv, nil
}

// derivePubKey derives the Curve25519 public key from a raw 32-byte private key
// and returns it as a base64-encoded string.
func derivePubKey(privKey []byte) (string, error) {
	if len(privKey) != 32 {
		return "", fmt.Errorf("private key must be 32 bytes, got %d", len(privKey))
	}
	pubBytes, err := curve25519.X25519(privKey, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("derive public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(pubBytes), nil
}

func FetchConfig(controllerURL, apiKey string, insecureSkipVerify bool) (*Config, error) {
	// Generate a fresh key pair locally.
	privKey, err := generateWGPrivKey()
	if err != nil {
		return nil, fmt.Errorf("generate wg private key: %w", err)
	}
	pubKeyB64, err := derivePubKey(privKey)
	if err != nil {
		zeroBytes(privKey)
		return nil, fmt.Errorf("derive wg public key: %w", err)
	}

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
		zeroBytes(privKey)
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-WG-KEY", apiKey)
	req.Header.Set("X-WG-PubKey", pubKeyB64)

	resp, err := client.Do(req)
	if err != nil {
		zeroBytes(privKey)
		return nil, fmt.Errorf("fetch config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		zeroBytes(privKey)
		return nil, fmt.Errorf("controller returned %d", resp.StatusCode)
	}

	var r types.WGServerConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		zeroBytes(privKey)
		return nil, fmt.Errorf("decode response: %w", err)
	}

	cfg := &Config{
		ControllerURL:      controllerURL,
		APIKey:             apiKey,
		ServerID:           r.ServerID,
		WireGuardPort:      r.WireGuardPort,
		WireGuardPrivKey:   privKey,
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
