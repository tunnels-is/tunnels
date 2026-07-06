package wgserver

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
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

	// PublicIP is this server's public IP as recorded on the controller's
	// Server record. When non-empty, the WireGuard UDP socket is bound to this
	// address (see pinnedBind) so a host with several local IPs listens on the
	// intended one. It is bind-only: egress NAT always uses MASQUERADE, which
	// auto-selects the single external IP under the one-IP-per-host topology.
	PublicIP string

	WireGuardPort    int
	WireGuardPrivKey []byte // raw 32-byte Curve25519 private key; zeroed after setup
	WireGuardSubnet  string
	WireGuardSubnet6 string
	WireGuardIface   string

	InternetIface string

	LogLevel string
	LogJSON  bool
	Silent   bool

	InsecureSkipVerify bool

	// EnableFirewall enables the peer-to-peer firewall on the WG interface.
	// When true, all peer-to-peer ingress is denied by default and peers must
	// announce their allowlist via the ACL control port to accept traffic.
	// Independent of this setting, the packet inspector is always active and
	// blocks all peer traffic to the server's own WG IP.
	EnableFirewall bool

	HandshakeBufferSize int
	HandshakeRatePerIP  int
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

const pkpath = "./.pk"

func loadOrGenerateLocalPrivKey() ([]byte, error) {
	data, err := os.ReadFile(pkpath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read pk from .pk %w", err)
		}
	}
	if len(data) != 0 {
		if info, statErr := os.Stat(pkpath); statErr == nil {
			if mode := info.Mode().Perm(); mode&0o077 != 0 {
				return nil, fmt.Errorf(".pk file has insecure permissions")
			}
		}

		priv, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			zeroBytes(priv)
			return nil, fmt.Errorf("decode PrivateKey: %w", err)
		}
		if len(priv) != 32 {
			zeroBytes(priv)
			return nil, fmt.Errorf("PrivateKey has wrong length: got %d, want 32", len(priv))
		}
		return priv, nil
	}

	priv, err := generateWGPrivKey()
	if err != nil {
		zeroBytes(priv)
		return nil, err
	}

	pk := base64.StdEncoding.EncodeToString(priv)

	if err := os.WriteFile(pkpath, []byte(pk), 0o600); err != nil {
		zeroBytes(priv)
		return nil, fmt.Errorf("write ./.pk file %q: %w", pkpath, err)
	}

	return priv, nil
}

func FetchConfig(controllerURL, apiKey, configPath string, insecureSkipVerify bool) (*Config, error) {
	privKey, err := loadOrGenerateLocalPrivKey()
	if err != nil {
		return nil, fmt.Errorf("load wg private key: %w", err)
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
		PublicIP:           r.ServerIP,
		WireGuardPort:      r.WireGuardPort,
		WireGuardPrivKey:   privKey,
		WireGuardSubnet:    r.WireGuardSubnet,
		WireGuardSubnet6:   r.WireGuardSubnet6,
		WireGuardIface:     r.WireGuardIface,
		InternetIface:      r.InternetIface,
		EnableFirewall:     r.EnableFirewall,
		InsecureSkipVerify: insecureSkipVerify,
	}

	if cfg.WireGuardPort == 0 {
		return nil, fmt.Errorf("no port set in config during fetch")
	}
	if cfg.WireGuardIface == "" {
		return nil, fmt.Errorf("no wg interface set in config during fetch")
	}
	if cfg.WireGuardSubnet == "" {
		return nil, fmt.Errorf("no subnet set in config during fetch")
	}
	if cfg.HandshakeBufferSize <= 0 {
		cfg.HandshakeBufferSize = 1000
	}
	if cfg.HandshakeRatePerIP <= 0 {
		cfg.HandshakeRatePerIP = 100
	}

	return cfg, nil
}
