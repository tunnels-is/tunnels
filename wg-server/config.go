package wgserver

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tunnels-is/tunnels/types"
	"golang.org/x/crypto/curve25519"
)

// errControllerRedirect is returned by the controller HTTP client when the
// server answers with a redirect. Redirects are refused so X-WG-KEY is never
// re-sent to a different Location (Go does not strip custom auth headers).
var errControllerRedirect = errors.New("refusing to follow controller redirect (X-WG-KEY must not leave the configured controller URL)")

// requireHTTPSControllerURL rejects non-https controller base URLs.
func requireHTTPSControllerURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("controller URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid controller URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("controller URL must use https:// (got scheme %q)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("controller URL missing host")
	}
	return nil
}

// newControllerHTTPClient builds the client used for all controller calls that
// carry X-WG-KEY. Redirects are never followed. InsecureSkipVerify remains a
// supported local option for self-signed controllers.
func newControllerHTTPClient(insecureSkipVerify bool) *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return errControllerRedirect
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // intentional for self-signed deployments
			},
		},
	}
}

type Config struct {
	ControllerURL string
	APIKey        string

	ServerID string

	PublicIP string

	WireGuardPort     int
	WireGuardMeshPort int
	WireGuardPrivKey  []byte
	WireGuardSubnet   string
	WireGuardSubnet6  string
	WireGuardIface    string

	InternetIface string

	LogLevel string
	LogJSON  bool
	Silent   bool

	InsecureSkipVerify bool

	EnableFirewall bool

	HandshakeBufferSize int
	HandshakeRatePerIP  int

	ConfigPath string
}

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

var localPrivKeyPath string

func validIfaceName(name string) bool {
	return types.ValidIfaceName(name)
}

func setPKPathFromConfig(configPath string) {
	dir := "."
	if configPath != "" {
		dir = filepath.Dir(configPath)
	}
	p, err := filepath.Abs(filepath.Join(dir, ".pk"))
	if err != nil {
		localPrivKeyPath = filepath.Join(dir, ".pk")
		return
	}
	localPrivKeyPath = p
}

func pkPath() string {
	if localPrivKeyPath != "" {
		return localPrivKeyPath
	}
	p, err := filepath.Abs(".pk")
	if err != nil {
		return ".pk"
	}
	return p
}

func loadOrGenerateLocalPrivKey(allowGenerate bool) ([]byte, error) {
	path := pkPath()
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read pk from %s: %w", path, err)
	}
	if len(data) == 0 && !allowGenerate {
		return nil, fmt.Errorf("%s missing; refusing to mint a new WireGuard key", path)
	}
	if len(data) != 0 {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return nil, fmt.Errorf("stat %s: %w", path, statErr)
		}
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			return nil, fmt.Errorf("%s has insecure permissions %o (want 0600); refusing to start", path, mode)
		}
		if ownErr := checkKeyFileOwner(path, info); ownErr != nil {
			return nil, fmt.Errorf("%s: %w", path, ownErr)
		}
		priv, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
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

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		zeroBytes(priv)
		return nil, fmt.Errorf("create .pk file %q: %w", path, err)
	}
	if _, err := f.WriteString(pk); err != nil {
		f.Close()
		zeroBytes(priv)
		return nil, fmt.Errorf("write .pk file %q: %w", path, err)
	}
	if err := f.Close(); err != nil {
		zeroBytes(priv)
		return nil, fmt.Errorf("close .pk file %q: %w", path, err)
	}

	return priv, nil
}

func FetchConfig(controllerURL, apiKey, configPath string, insecureSkipVerify bool) (*Config, error) {
	return fetchConfig(controllerURL, apiKey, configPath, insecureSkipVerify, true)
}

func fetchConfig(controllerURL, apiKey, configPath string, insecureSkipVerify bool, generateKey bool) (*Config, error) {
	if err := requireHTTPSControllerURL(controllerURL); err != nil {
		return nil, err
	}

	setPKPathFromConfig(configPath)
	privKey, err := loadOrGenerateLocalPrivKey(generateKey)
	if err != nil {
		return nil, fmt.Errorf("load wg private key: %w", err)
	}
	pubKeyB64, err := derivePubKey(privKey)
	if err != nil {
		zeroBytes(privKey)
		return nil, fmt.Errorf("derive wg public key: %w", err)
	}

	client := newControllerHTTPClient(insecureSkipVerify)

	fetchURL := strings.TrimRight(controllerURL, "/") + "/wg/server-config/fetch"
	req, err := http.NewRequest(http.MethodGet, fetchURL, nil)
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
		ConfigPath:        configPath,

		// Local wg-config.json / CLI only. The controller must not remotely
		// disable TLS verification for X-WG-KEY calls.
		InsecureSkipVerify: insecureSkipVerify,
	}

	if cfg.WireGuardPort == 0 {
		zeroBytes(privKey)
		return nil, fmt.Errorf("no port set in config during fetch")
	}
	if !validIfaceName(cfg.WireGuardIface) {
		zeroBytes(privKey)
		return nil, fmt.Errorf("invalid WireGuardIface %q", cfg.WireGuardIface)
	}
	if !validIfaceName(cfg.InternetIface) {
		zeroBytes(privKey)
		return nil, fmt.Errorf("invalid or empty InternetIface %q", cfg.InternetIface)
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
