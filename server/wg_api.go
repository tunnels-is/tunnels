package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/curve25519"
)

// API_WGPeers handles GET /v3/wg/peers.
// Returns all devices with a WireGuard key registered (DeviceID + hex public key).
// IP assignment is owned by each wg-server; AllowedIPs are not included here.
// Protected by AdminAPIKey.
func API_WGPeers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if !HTTP_validateKey(r) {
		senderr(w, 401, "Unauthorized")
		return
	}

	devices, err := DB_GetDevices(100000, 0)
	if err != nil {
		senderr(w, 500, "Failed to fetch devices", slog.Any("err", err))
		return
	}

	resp := types.WGPeersResponse{
		Peers: make([]types.WGPeer, 0),
	}
	for _, d := range devices {
		if d.WireGuardKey == "" {
			continue
		}
		hexKey, err := b64KeyToHex(d.WireGuardKey)
		if err != nil {
			logger.Warn("skipping device with invalid WireGuard key",
				slog.String("device", d.ID.Hex()), slog.Any("err", err))
			continue
		}
		resp.Peers = append(resp.Peers, types.WGPeer{
			PublicKeyHex: hexKey,
			DeviceID:     d.ID.Hex(),
			WireGuardIP:  d.WireGuardIP,
		})
	}

	sendObject(w, resp)
}

// API_WGConfig handles GET /v3/wg/config.
// Returns the WireGuard server's public key and connection details for a given server.
func API_WGConfig(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	serverIDStr := r.URL.Query().Get("serverID")
	if serverIDStr == "" {
		senderr(w, 400, "serverID query parameter is required")
		return
	}
	serverID, err := primitive.ObjectIDFromHex(serverIDStr)
	if err != nil {
		senderr(w, 400, "Invalid serverID")
		return
	}

	server, err := DB_FindServerByID(serverID)
	if err != nil || server == nil {
		senderr(w, 404, "Server not found")
		return
	}

	sendObject(w, map[string]string{
		"WireGuardPubKey": server.WireGuardPubKey,
		"WireGuardPort":   server.WireGuardPort,
		"ServerIP":        server.IP,
	})
}

// assignNextWireGuardIP finds the next available IP in the server's WireGuard subnet.
// It scans all devices' WireGuardIP fields to find used IPs and returns the next
// unallocated address. The server interface occupies .1; device IPs start at .2.
func assignNextWireGuardIP(serverID primitive.ObjectID) (string, error) {
	server, err := DB_FindServerByID(serverID)
	if err != nil || server == nil {
		return "", fmt.Errorf("server not found")
	}
	if server.WireGuardSubnet == "" {
		return "", fmt.Errorf("server has no WireGuard subnet configured")
	}
	_, ipNet, err := net.ParseCIDR(server.WireGuardSubnet)
	if err != nil {
		return "", fmt.Errorf("invalid subnet %q: %w", server.WireGuardSubnet, err)
	}

	devices, err := DB_GetDevices(100000, 0)
	if err != nil {
		return "", fmt.Errorf("list devices: %w", err)
	}

	used := make(map[uint32]bool)
	for _, d := range devices {
		if d.WireGuardIP == "" {
			continue
		}
		ip4 := net.ParseIP(d.WireGuardIP).To4()
		if ip4 != nil && ipNet.Contains(ip4) {
			used[wgIPToU32(ip4)] = true
		}
	}

	base := wgIPToU32(ipNet.IP.To4()) + 2 // .1 is server, start at .2
	for {
		next := wgU32ToIP(base)
		if !ipNet.Contains(next) {
			return "", fmt.Errorf("WireGuard subnet %s is exhausted", server.WireGuardSubnet)
		}
		if !used[base] {
			return next.String(), nil
		}
		base++
	}
}

func wgIPToU32(ip net.IP) uint32 {
	ip = ip.To4()
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

func wgU32ToIP(n uint32) net.IP {
	return net.IP{byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)}
}

// b64KeyToHex converts a base64-encoded 32-byte key to a hex string for UAPI.
func b64KeyToHex(b64 string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}
	if len(b) != 32 {
		return "", fmt.Errorf("expected 32 bytes, got %d", len(b))
	}
	return fmt.Sprintf("%x", b), nil
}

// HTTP_validateWGKey authenticates a request using the X-WG-KEY header.
// Returns the matching WGServerConfig and true, or nil and false on failure.
func HTTP_validateWGKey(r *http.Request) (*types.WGServerConfig, bool) {
	key := r.Header.Get("X-WG-KEY")
	if key == "" {
		return nil, false
	}
	cfg, err := DB_FindWGServerConfigByAPIKey(key)
	if err != nil || cfg == nil {
		return nil, false
	}
	return cfg, true
}

// generateWGPrivKey creates a new clamped Curve25519 private key, base64-encoded.
func generateWGPrivKey() string {
	privKey := make([]byte, 32)
	if _, err := rand.Read(privKey); err != nil {
		panic("rand.Read failed: " + err.Error())
	}
	privKey[0] &= 248
	privKey[31] = (privKey[31] & 127) | 64
	return base64.StdEncoding.EncodeToString(privKey)
}

// deriveWGPubKey computes the Curve25519 public key from a base64-encoded private key.
func deriveWGPubKey(privKeyB64 string) (string, error) {
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

// FORM_WG_SERVER_CONFIG_CREATE is the body for POST /v3/wg/server-config.
type FORM_WG_SERVER_CONFIG_CREATE struct {
	UID         primitive.ObjectID `json:"UID"`
	DeviceToken string             `json:"DeviceToken"`

	Tag             string `json:"Tag"`
	AdminAPIKey     string `json:"AdminAPIKey"`
	WireGuardPort   int    `json:"WireGuardPort"`
	WireGuardSubnet string `json:"WireGuardSubnet"`
	WireGuardIface  string `json:"WireGuardIface"`
	InternetIface   string `json:"InternetIface"`

	PacketInspection   bool `json:"PacketInspection"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify"`
}

// API_WGServerConfigCreate handles POST /v3/wg/server-config.
// Requires admin user auth. Generates APIKey (UUID) and WireGuardPrivKey (Curve25519).
func API_WGServerConfigCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodPost {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	F := new(FORM_WG_SERVER_CONFIG_CREATE)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if !HTTP_validateKey(r) {
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			senderr(w, 401, err.Error())
			return
		}
		if !user.IsAdmin {
			senderr(w, 401, "Admin required")
			return
		}
	}

	privKeyB64 := generateWGPrivKey()
	pubKeyB64, err := deriveWGPubKey(privKeyB64)
	if err != nil {
		senderr(w, 500, "Failed to derive WireGuard public key", slog.Any("err", err))
		return
	}

	cfg := &types.WGServerConfig{
		ID:               primitive.NewObjectID(),
		Tag:              F.Tag,
		APIKey:           uuid.NewString(),
		AdminAPIKey:      F.AdminAPIKey,
		WireGuardPort:    F.WireGuardPort,
		WireGuardPrivKey: privKeyB64,
		WireGuardSubnet:  F.WireGuardSubnet,
		WireGuardIface:   F.WireGuardIface,
		InternetIface:    F.InternetIface,
		PacketInspection:   F.PacketInspection,
		InsecureSkipVerify: F.InsecureSkipVerify,
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

	if err := DB_CreateWGServerConfig(cfg); err != nil {
		senderr(w, 500, "Failed to create WGServerConfig", slog.Any("err", err))
		return
	}

	sendObject(w, map[string]any{
		"ID":              cfg.ID.Hex(),
		"APIKey":          cfg.APIKey,
		"WireGuardPubKey": pubKeyB64,
		"Tag":             cfg.Tag,
		"WireGuardPort":   cfg.WireGuardPort,
		"WireGuardSubnet": cfg.WireGuardSubnet,
		"WireGuardIface":  cfg.WireGuardIface,
		"InternetIface":   cfg.InternetIface,
	})
}

// API_WGServerConfigGet handles GET /v3/wg/server-config/get?id=<hex>.
// Returns the config without the private key (redacted for UI display).
// Requires admin user auth via X-API-KEY or UID+DeviceToken query params.
func API_WGServerConfigGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if !HTTP_validateKey(r) {
		senderr(w, 401, "Unauthorized")
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		senderr(w, 400, "id query parameter is required")
		return
	}
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		senderr(w, 400, "Invalid id")
		return
	}

	cfg, err := DB_FindWGServerConfigByID(id)
	if err != nil || cfg == nil {
		senderr(w, 404, "WGServerConfig not found")
		return
	}

	pubKey, _ := deriveWGPubKey(cfg.WireGuardPrivKey)

	// Return config with privkey redacted
	sendObject(w, map[string]any{
		"ID":                 cfg.ID.Hex(),
		"Tag":                cfg.Tag,
		"APIKey":             cfg.APIKey,
		"WireGuardPubKey":    pubKey,
		"WireGuardPort":      cfg.WireGuardPort,
		"WireGuardSubnet":    cfg.WireGuardSubnet,
		"WireGuardIface":     cfg.WireGuardIface,
		"InternetIface":      cfg.InternetIface,
		"PacketInspection":   cfg.PacketInspection,
		"InsecureSkipVerify": cfg.InsecureSkipVerify,
	})
}

// API_WGServerConfigFetch handles GET /v3/wg/server-config/fetch.
// Authenticated via X-WG-KEY header. Returns the full WGServerConfigResponse
// (including privkey and AdminAPIKey) and refreshes the Server's cached WG fields.
func API_WGServerConfigFetch(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	wgCfg, ok := HTTP_validateWGKey(r)
	if !ok {
		senderr(w, 401, "Unauthorized")
		return
	}

	pubKeyB64, err := deriveWGPubKey(wgCfg.WireGuardPrivKey)
	if err != nil {
		senderr(w, 500, "Failed to derive public key", slog.Any("err", err))
		return
	}

	// Find the server linked to this config and refresh its cached WG fields.
	var serverID string
	servers, err := DB_FindAllServers()
	if err == nil {
		for _, s := range servers {
			if s.WGConfigID == wgCfg.ID {
				serverID = s.ID.Hex()
				_ = DB_SetServerWGConfigID(s.ID, wgCfg, pubKeyB64)
				break
			}
		}
	}

	resp := &types.WGServerConfigResponse{
		ServerID:           serverID,
		AdminAPIKey:        wgCfg.AdminAPIKey,
		WireGuardPort:      wgCfg.WireGuardPort,
		WireGuardPrivKey:   wgCfg.WireGuardPrivKey,
		WireGuardSubnet:    wgCfg.WireGuardSubnet,
		WireGuardIface:     wgCfg.WireGuardIface,
		InternetIface:      wgCfg.InternetIface,
		PacketInspection:   wgCfg.PacketInspection,
		InsecureSkipVerify: wgCfg.InsecureSkipVerify,
	}

	sendObject(w, resp)
}

// FORM_WG_SERVER_CONFIG_ASSIGN is the body for POST /v3/wg/server-config/assign.
type FORM_WG_SERVER_CONFIG_ASSIGN struct {
	UID         primitive.ObjectID `json:"UID"`
	DeviceToken string             `json:"DeviceToken"`

	ServerID primitive.ObjectID `json:"ServerID"`
	ConfigID primitive.ObjectID `json:"ConfigID"`
}

// API_WGServerConfigAssign handles POST /v3/wg/server-config/assign.
// Links a Server to a WGServerConfig and caches the WG fields on the Server.
func API_WGServerConfigAssign(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodPost {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	F := new(FORM_WG_SERVER_CONFIG_ASSIGN)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if !HTTP_validateKey(r) {
		user, err := authenticateUserFromEmailOrIDAndToken("", F.UID, F.DeviceToken)
		if err != nil {
			senderr(w, 401, err.Error())
			return
		}
		if !user.IsAdmin {
			senderr(w, 401, "Admin required")
			return
		}
	}

	if F.ServerID == primitive.NilObjectID || F.ConfigID == primitive.NilObjectID {
		senderr(w, 400, "ServerID and ConfigID are required")
		return
	}

	wgCfg, err := DB_FindWGServerConfigByID(F.ConfigID)
	if err != nil || wgCfg == nil {
		senderr(w, 404, "WGServerConfig not found")
		return
	}

	pubKeyB64, err := deriveWGPubKey(wgCfg.WireGuardPrivKey)
	if err != nil {
		senderr(w, 500, "Failed to derive WireGuard public key", slog.Any("err", err))
		return
	}

	if err := DB_SetServerWGConfigID(F.ServerID, wgCfg, pubKeyB64); err != nil {
		senderr(w, 500, "Failed to update server", slog.Any("err", err))
		return
	}

	sendObject(w, map[string]any{
		"WireGuardPubKey": pubKeyB64,
		"WireGuardPort":   wgCfg.WireGuardPort,
		"WireGuardSubnet": wgCfg.WireGuardSubnet,
	})
}

// API_WGServers handles GET /v3/wg/servers.
// Returns all wg-servers (excluding the caller) that have a WireGuardSubnet,
// so a wg-server can discover peers for cross-server routing.
// Protected by AdminAPIKey.
// Optional query param: excludeID=<serverID hex>
func API_WGServers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !HTTP_validateKey(r) {
		senderr(w, 401, "Unauthorized")
		return
	}

	excludeID := r.URL.Query().Get("excludeID")

	servers, err := DB_FindAllServers()
	if err != nil {
		senderr(w, 500, "Failed to fetch servers", slog.Any("err", err))
		return
	}

	resp := types.WGServersResponse{Servers: make([]types.WGServerInfo, 0)}
	for _, s := range servers {
		if s.WireGuardSubnet == "" || s.WireGuardPubKey == "" {
			continue
		}
		if excludeID != "" && s.ID.Hex() == excludeID {
			continue
		}
		resp.Servers = append(resp.Servers, types.WGServerInfo{
			WireGuardPubKey: s.WireGuardPubKey,
			WireGuardPort:   s.WireGuardPort,
			WireGuardSubnet: s.WireGuardSubnet,
			IP:              s.IP,
		})
	}

	sendObject(w, resp)
}

// buildWGConf generates a ready-to-use WireGuard .conf file for a device.
func buildWGConf(assignedIP, clientPubKeyB64 string, server *types.Server) string {
	return fmt.Sprintf(`[Interface]
Address = %s/32
# PrivateKey = <paste your private key here>
DNS = 1.1.1.1

[Peer]
PublicKey = %s
Endpoint = %s:%s
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
`, assignedIP, server.WireGuardPubKey, server.IP, server.WireGuardPort)
}
