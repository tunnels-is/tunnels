package main

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

func API_WGPeers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if _, ok := HTTP_validateWGKey(r); !ok {
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
				slog.String("device", d.ID.String()), slog.Any("err", err))
			continue
		}
		resp.Peers = append(resp.Peers, types.WGPeer{
			PublicKeyHex: hexKey,
			DeviceID:     d.ID.String(),
			WireGuardIP:  d.WireGuardIP,
		})
	}

	sendObject(w, resp)
}

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
	serverID, err := uuid.Parse(serverIDStr)
	if err != nil {
		senderr(w, 400, "Invalid serverID")
		return
	}

	server, err := DB_FindServerByID(serverID)
	if err != nil || server == nil {
		senderr(w, 404, "Server not found")
		return
	}

	var clientWireGuardIP string
	if pubKey := r.URL.Query().Get("pubKey"); pubKey != "" {
		devices, devErr := DB_GetDevices(100000, 0)
		if devErr == nil {
			for _, d := range devices {
				if d.WireGuardKey == pubKey {
					clientWireGuardIP = d.WireGuardIP
					break
				}
			}
		}
	}

	sendObject(w, map[string]string{
		"WireGuardPubKey": server.WireGuardPubKey,
		"WireGuardPort":   server.WireGuardPort,
		"ServerIP":        server.IP,
		"WireGuardIP":     clientWireGuardIP,
	})
}

func assignNextWireGuardIP(serverID uuid.UUID) (string, error) {
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

	base := wgIPToU32(ipNet.IP.To4()) + 2
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

type FORM_WG_SERVER_CONFIG_CREATE struct {
	Tag            string             `json:"Tag"`
	WireGuardPort  int                `json:"WireGuardPort"`
	NetworkID      uuid.UUID `json:"NetworkID"`
	WireGuardIface string    `json:"WireGuardIface"`
	InternetIface  string    `json:"InternetIface"`

	PacketInspection   bool `json:"PacketInspection"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify"`
}

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

	if !isAdminAPIKeyFromContext(r.Context()) {
		user := getUserFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			senderr(w, 401, "Admin required")
			return
		}
	}

	var subnet string
	if F.NetworkID != uuid.Nil {
		network, nerr := DB_FindNetworkByID(F.NetworkID)
		if nerr != nil || network == nil {
			senderr(w, 404, "Network not found")
			return
		}
		subnet = network.CIDR
	}

	cfg := &types.WGServerConfig{
		ID:                 uuid.New(),
		Tag:                F.Tag,
		APIKey:             uuid.NewString(),
		WireGuardPort:      F.WireGuardPort,
		NetworkID:          F.NetworkID,
		WireGuardIface:     F.WireGuardIface,
		InternetIface:      F.InternetIface,
		PacketInspection:   F.PacketInspection,
		InsecureSkipVerify: F.InsecureSkipVerify,
	}
	if cfg.WireGuardPort == 0 {
		cfg.WireGuardPort = 51820
	}
	if cfg.WireGuardIface == "" {
		cfg.WireGuardIface = "wg0"
	}

	if err := DB_CreateWGServerConfig(cfg); err != nil {
		senderr(w, 500, "Failed to create WGServerConfig", slog.Any("err", err))
		return
	}

	if F.NetworkID != uuid.Nil {
		if network, _ := DB_FindNetworkByID(F.NetworkID); network != nil {
			network.WGConfigID = cfg.ID
			_ = DB_UpdateNetwork(network)
		}
	}

	sendObject(w, map[string]any{
		"ID":             cfg.ID.String(),
		"APIKey":         cfg.APIKey,
		"Tag":            cfg.Tag,
		"WireGuardPort":  cfg.WireGuardPort,
		"WireGuardSubnet": subnet,
		"WireGuardIface": cfg.WireGuardIface,
		"InternetIface":  cfg.InternetIface,
	})
}

func API_WGServerConfigGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		senderr(w, 400, "id query parameter is required")
		return
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		senderr(w, 400, "Invalid id")
		return
	}

	cfg, err := DB_FindWGServerConfigByID(id)
	if err != nil || cfg == nil {
		senderr(w, 404, "WGServerConfig not found")
		return
	}

	var cfgSubnet string
	if cfg.NetworkID != uuid.Nil {
		if net, _ := DB_FindNetworkByID(cfg.NetworkID); net != nil {
			cfgSubnet = net.CIDR
		}
	}

	sendObject(w, map[string]any{
		"ID":                 cfg.ID.String(),
		"Tag":                cfg.Tag,
		"APIKey":             cfg.APIKey,
		"WireGuardPubKey":    cfg.WireGuardPubKey,
		"WireGuardPort":      cfg.WireGuardPort,
		"WireGuardSubnet":    cfgSubnet,
		"WireGuardIface":     cfg.WireGuardIface,
		"InternetIface":      cfg.InternetIface,
		"PacketInspection":   cfg.PacketInspection,
		"InsecureSkipVerify": cfg.InsecureSkipVerify,
		"NetworkID":          cfg.NetworkID.String(),
	})
}

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

	// Accept the wg-server's public key from the request header.
	pubKeyB64 := r.Header.Get("X-WG-PubKey")
	if pubKeyB64 != "" {
		wgCfg.WireGuardPubKey = pubKeyB64
		_ = DB_UpdateWGServerConfig(wgCfg)
	}

	var fetchSubnet string
	if wgCfg.NetworkID != uuid.Nil {
		if network, _ := DB_FindNetworkByID(wgCfg.NetworkID); network != nil {
			fetchSubnet = network.CIDR
		}
	}

	var serverID string
	servers, err := DB_FindAllServers()
	if err == nil {
		for _, s := range servers {
			if s.WGConfigID == wgCfg.ID {
				serverID = s.ID.String()
				_ = DB_SetServerWGConfigID(s.ID, wgCfg, wgCfg.WireGuardPubKey, fetchSubnet)
				break
			}
		}
	}

	resp := &types.WGServerConfigResponse{
		ServerID:           serverID,
		WireGuardPort:      wgCfg.WireGuardPort,
		WireGuardSubnet:    fetchSubnet,
		WireGuardIface:     wgCfg.WireGuardIface,
		InternetIface:      wgCfg.InternetIface,
		PacketInspection:   wgCfg.PacketInspection,
		InsecureSkipVerify: wgCfg.InsecureSkipVerify,
	}

	sendObject(w, resp)
}

type FORM_WG_SERVER_CONFIG_UPDATE struct {
	ID             uuid.UUID `json:"ID"`
	Tag            string    `json:"Tag"`
	WireGuardPort  int       `json:"WireGuardPort"`
	NetworkID      uuid.UUID `json:"NetworkID"`
	WireGuardIface string    `json:"WireGuardIface"`
	InternetIface  string    `json:"InternetIface"`

	PacketInspection   bool `json:"PacketInspection"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify"`
}

func API_WGServerConfigUpdate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	F := new(FORM_WG_SERVER_CONFIG_UPDATE)
	if err := decodeBody(r, F); err != nil {
		senderr(w, 400, "Invalid request body", slog.Any("error", err))
		return
	}

	if !isAdminAPIKeyFromContext(r.Context()) {
		user := getUserFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			senderr(w, 401, "Admin required")
			return
		}
	}

	existing, err := DB_FindWGServerConfigByID(F.ID)
	if err != nil || existing == nil {
		senderr(w, 404, "WGServerConfig not found")
		return
	}

	if existing.NetworkID != F.NetworkID {
		if existing.NetworkID != uuid.Nil {
			if oldNet, _ := DB_FindNetworkByID(existing.NetworkID); oldNet != nil {
				oldNet.WGConfigID = uuid.Nil
				_ = DB_UpdateNetwork(oldNet)
			}
		}
		if F.NetworkID != uuid.Nil {
			if newNet, _ := DB_FindNetworkByID(F.NetworkID); newNet != nil {
				newNet.WGConfigID = F.ID
				_ = DB_UpdateNetwork(newNet)
			}
		}
	}

	existing.Tag = F.Tag
	existing.WireGuardPort = F.WireGuardPort
	existing.NetworkID = F.NetworkID
	existing.WireGuardIface = F.WireGuardIface
	existing.InternetIface = F.InternetIface
	existing.PacketInspection = F.PacketInspection
	existing.InsecureSkipVerify = F.InsecureSkipVerify

	if err := DB_UpdateWGServerConfig(existing); err != nil {
		senderr(w, 500, "Failed to update WGServerConfig", slog.Any("err", err))
		return
	}

	w.WriteHeader(200)
}

type FORM_WG_SERVER_CONFIG_ASSIGN struct {
	ServerID uuid.UUID `json:"ServerID"`
	ConfigID uuid.UUID `json:"ConfigID"`
}

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

	if !isAdminAPIKeyFromContext(r.Context()) {
		user := getUserFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			senderr(w, 401, "Admin required")
			return
		}
	}

	if F.ServerID == uuid.Nil || F.ConfigID == uuid.Nil {
		senderr(w, 400, "ServerID and ConfigID are required")
		return
	}

	wgCfg, err := DB_FindWGServerConfigByID(F.ConfigID)
	if err != nil || wgCfg == nil {
		senderr(w, 404, "WGServerConfig not found")
		return
	}

	var assignSubnet string
	if wgCfg.NetworkID != uuid.Nil {
		if network, _ := DB_FindNetworkByID(wgCfg.NetworkID); network != nil {
			assignSubnet = network.CIDR
		}
	}

	if err := DB_SetServerWGConfigID(F.ServerID, wgCfg, wgCfg.WireGuardPubKey, assignSubnet); err != nil {
		senderr(w, 500, "Failed to update server", slog.Any("err", err))
		return
	}

	sendObject(w, map[string]any{
		"WireGuardPubKey": wgCfg.WireGuardPubKey,
		"WireGuardPort":   wgCfg.WireGuardPort,
		"WireGuardSubnet": assignSubnet,
	})
}

func API_WGServers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()
	if r.Method != http.MethodGet {
		senderr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if _, ok := HTTP_validateWGKey(r); !ok {
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
		if excludeID != "" && s.ID.String() == excludeID {
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

func buildWGConf(assignedIP, clientPubKeyB64 string, server *types.Server) string {
	return fmt.Sprintf(`[Interface]
Address = %s/32
# PrivateKey = <paste your private key here>
DNS = 1.1.1.1

[Peer]
PublicKey = %s
Endpoint = %s:%s
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 15
`, assignedIP, server.WireGuardPubKey, server.IP, server.WireGuardPort)
}
