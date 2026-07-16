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
	// Server record. It is currently vestigial: the WireGuard socket uses the
	// default wildcard bind (all local IPs, default routing) and egress NAT uses
	// MASQUERADE (default-route source), so neither consults PublicIP. It is
	// still read only to drain a legacy SNAT --to-source rule left by older
	// binaries on upgrade (see flushWGRules). Safe to remove once no deployed
	// host carries that legacy rule.
	PublicIP string

	WireGuardPort     int
	WireGuardMeshPort int    // server-to-server mesh UDP port (0 = mesh disabled)
	WireGuardPrivKey  []byte // raw 32-byte Curve25519 private key; zeroed after setup
	WireGuardSubnet   string
	WireGuardSubnet6  string
	WireGuardIface    string

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

// pkpath holds the server's long-lived Curve25519 private key. Persisting it
// is a deliberate design choice (the server keeps a stable WG identity across
// restarts, so peers reconnect without re-provisioning) and reverses the
// earlier "ephemeral per boot" property: anyone who can read this file (disk
// access, backup leak) can impersonate the server and identify peers in future
// handshakes. Data forward-secrecy is unaffected (per-session ephemerals).
// The file must be 0600 and owned by the wg-server user; both are enforced
// below. Protect the working directory / backups accordingly.
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
			if ownErr := checkKeyFileOwner(info); ownErr != nil {
				return nil, fmt.Errorf(".pk file: %w", ownErr)
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

	// A zero-length .pk (e.g. a crash between the O_EXCL create and the write
	// below) would otherwise wedge startup forever: the load branch is skipped
	// and the O_EXCL create fails with "file exists". Treat it as absent.
	if _, statErr := os.Stat(pkpath); statErr == nil {
		if err := os.Remove(pkpath); err != nil {
			return nil, fmt.Errorf("remove empty ./.pk file: %w", err)
		}
	}

	priv, err := generateWGPrivKey()
	if err != nil {
		zeroBytes(priv)
		return nil, err
	}

	pk := base64.StdEncoding.EncodeToString(priv)

	// O_EXCL: fail if the file already exists rather than truncating it. Writing
	// with os.WriteFile into a pre-existing file does NOT apply the 0600 mode, so
	// an attacker who pre-plants ./.pk (world-readable, owned by them) would
	// otherwise receive our freshly generated private key. O_EXCL forces us to
	// create it fresh with the right mode, or refuse.
	f, err := os.OpenFile(pkpath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		zeroBytes(priv)
		return nil, fmt.Errorf("create ./.pk file %q: %w", pkpath, err)
	}
	if _, err := f.WriteString(pk); err != nil {
		f.Close()
		zeroBytes(priv)
		return nil, fmt.Errorf("write ./.pk file %q: %w", pkpath, err)
	}
	if err := f.Close(); err != nil {
		zeroBytes(priv)
		return nil, fmt.Errorf("close ./.pk file %q: %w", pkpath, err)
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
		ControllerURL:     controllerURL,
		APIKey:            apiKey,
		ServerID:          r.ServerID,
		PublicIP:          r.ServerIP,
		WireGuardPort:     r.WireGuardPort,
		WireGuardMeshPort: r.WireGuardMeshPort,
		WireGuardPrivKey:  privKey,
		WireGuardSubnet:   r.WireGuardSubnet,
		WireGuardSubnet6:  r.WireGuardSubnet6,
		WireGuardIface:    r.WireGuardIface,
		InternetIface:     r.InternetIface,
		EnableFirewall:    r.EnableFirewall,
		// The initial fetch above was governed by the bootstrap flag
		// (insecureSkipVerify) — the wg-server must decide controller trust
		// before it can fetch anything. For the ongoing controller calls
		// (per-handshake /wg/peer) we also honor the server record's setting
		// (r.InsecureSkipVerify), so the admin-UI toggle propagates here. Skip
		// only if either says so; the default (both false) verifies.
		InsecureSkipVerify: insecureSkipVerify || r.InsecureSkipVerify,
	}

	if cfg.WireGuardPort == 0 {
		zeroBytes(privKey)
		return nil, fmt.Errorf("no port set in config during fetch")
	}
	if cfg.WireGuardIface == "" {
		zeroBytes(privKey)
		return nil, fmt.Errorf("no wg interface set in config during fetch")
	}
	if cfg.WireGuardSubnet == "" {
		zeroBytes(privKey)
		return nil, fmt.Errorf("no subnet set in config during fetch")
	}
	if cfg.HandshakeBufferSize <= 0 {
		cfg.HandshakeBufferSize = 1000
	}
	if cfg.HandshakeRatePerIP <= 0 {
		cfg.HandshakeRatePerIP = 100
	}

	if cfg.InsecureSkipVerify {
		WARN("wg-server: controller TLS certificate verification is DISABLED " +
			"(InsecureSkipVerify) — only use this with a self-signed/trusted setup on a " +
			"trusted network; the peer-authorization channel is exposed to MITM otherwise")
	}

	return cfg, nil
}
