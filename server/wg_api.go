package main

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"

	"github.com/google/uuid"
	"github.com/tunnels-is/tunnels/types"
)

const (
	wgPeersDefaultLimit = 500
	wgPeersMaxLimit     = 5000
)

func API_WGPeers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	if _, ok := HTTP_validateWGKey(r); !ok {
		senderr(w, 401, "Unauthorized")
		return
	}

	limit := wgPeersDefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			senderr(w, 400, "limit must be a positive integer")
			return
		}
		if n > wgPeersMaxLimit {
			n = wgPeersMaxLimit
		}
		limit = n
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			senderr(w, 400, "offset must be a non-negative integer")
			return
		}
		offset = n
	}

	devices, err := DB_GetDevices(int64(limit), int64(offset))
	if err != nil {
		senderr(w, 500, "Failed to fetch devices", slog.Any("err", err))
		return
	}

	resp := types.WGPeersResponse{
		Peers:  make([]types.WGPeer, 0, len(devices)),
		Limit:  limit,
		Offset: offset,
	}
	for _, d := range devices {
		if d.WireGuardKey == "" {
			continue
		}
		hexKey, err := b64KeyToHex(d.WireGuardKey)
		if err != nil {
			continue
		}
		resp.Peers = append(resp.Peers, types.WGPeer{
			PublicKeyHex:  hexKey,
			DeviceID:      d.ID.String(),
			WireGuardIP:   d.WireGuardIP,
			WireGuardIPv6: d.WireGuardIPv6,
		})
	}

	if len(devices) < limit {
		resp.NextOffset = -1
	} else {
		resp.NextOffset = offset + limit
	}

	sendObject(w, resp)
}

func API_WGPeer(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

	if _, ok := HTTP_validateWGKey(r); !ok {
		senderr(w, 401, "Unauthorized")
		return
	}

	pubKeyB64 := r.URL.Query().Get("pubkey")
	if pubKeyB64 == "" {
		senderr(w, 400, "pubkey query parameter is required")
		return
	}
	raw, err := base64.StdEncoding.DecodeString(pubKeyB64)
	if err != nil || len(raw) != 32 {
		senderr(w, 400, "pubkey must be a base64-encoded 32-byte key")
		return
	}

	dev, err := DB_FindDeviceByWGKey(pubKeyB64)
	if err != nil {
		senderr(w, 500, "Failed to look up device", slog.Any("err", err))
		return
	}
	if dev == nil {
		senderr(w, 404, "peer not found")
		return
	}

	hexKey, err := b64KeyToHex(dev.WireGuardKey)
	if err != nil {
		senderr(w, 500, "Failed to encode device key", slog.Any("err", err))
		return
	}

	sendObject(w, types.WGPeer{
		PublicKeyHex:  hexKey,
		DeviceID:      dev.ID.String(),
		WireGuardIP:   dev.WireGuardIP,
		WireGuardIPv6: dev.WireGuardIPv6,
	})
}

func API_WGConfig(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

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
	var clientWireGuardIPv6 string
	if pubKey := r.URL.Query().Get("pubKey"); pubKey != "" {
		devices, devErr := DB_GetDevices(100000, 0)
		if devErr == nil {
			for _, d := range devices {
				if d.WireGuardKey == pubKey {
					clientWireGuardIP = d.WireGuardIP
					clientWireGuardIPv6 = d.WireGuardIPv6
					break
				}
			}
		}
	}

	sendObject(w, map[string]string{
		"WireGuardPubKey":  server.WireGuardPubKey,
		"WireGuardPort":    server.WireGuardPort,
		"ServerIP":         server.IP,
		"WireGuardIP":      clientWireGuardIP,
		"WireGuardIPv6":    clientWireGuardIPv6,
		"WireGuardSubnet":  server.WireGuardSubnet,
		"WireGuardSubnet6": server.WireGuardSubnet6,
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

// assignNextWireGuardIPv6 allocates the next free IPv6 address from the
// server's WireGuardSubnet6. Returns "" (no error) if no IPv6 subnet is
// configured on the server.
func assignNextWireGuardIPv6(serverID uuid.UUID) (string, error) {
	server, err := DB_FindServerByID(serverID)
	if err != nil || server == nil {
		return "", fmt.Errorf("server not found")
	}
	if server.WireGuardSubnet6 == "" {
		return "", nil
	}
	prefix, err := netip.ParsePrefix(server.WireGuardSubnet6)
	if err != nil {
		return "", fmt.Errorf("invalid IPv6 subnet %q: %w", server.WireGuardSubnet6, err)
	}

	devices, err := DB_GetDevices(100000, 0)
	if err != nil {
		return "", fmt.Errorf("list devices: %w", err)
	}

	used := make(map[netip.Addr]bool)
	for _, d := range devices {
		if d.WireGuardIPv6 == "" {
			continue
		}
		if addr, parseErr := netip.ParseAddr(d.WireGuardIPv6); parseErr == nil && prefix.Contains(addr) {
			used[addr] = true
		}
	}

	// Start at base+2 (skip network address and server address).
	candidate := prefix.Addr().Next().Next()
	for prefix.Contains(candidate) {
		if !used[candidate] {
			return candidate.String(), nil
		}
		candidate = candidate.Next()
	}
	return "", fmt.Errorf("WireGuard IPv6 subnet %s is exhausted", server.WireGuardSubnet6)
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
	Tag            string    `json:"Tag"`
	WireGuardPort  int       `json:"WireGuardPort"`
	NetworkID      uuid.UUID `json:"NetworkID"`
	NetworkID6     uuid.UUID `json:"NetworkID6"`
	WireGuardIface string    `json:"WireGuardIface"`
	InternetIface  string    `json:"InternetIface"`

	PacketInspection   bool `json:"PacketInspection"`
	InsecureSkipVerify bool `json:"InsecureSkipVerify"`
}

func API_WGServerConfigCreate(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

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

	var subnet6 string
	if F.NetworkID6 != uuid.Nil {
		network6, nerr := DB_FindNetworkByID(F.NetworkID6)
		if nerr != nil || network6 == nil {
			senderr(w, 404, "IPv6 network not found")
			return
		}
		subnet6 = network6.CIDR
	}

	cfg := &types.WGServerConfig{
		ID:                 uuid.New(),
		Tag:                F.Tag,
		APIKey:             uuid.NewString(),
		WireGuardPort:      F.WireGuardPort,
		NetworkID:          F.NetworkID,
		NetworkID6:         F.NetworkID6,
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
	if F.NetworkID6 != uuid.Nil {
		if network6, _ := DB_FindNetworkByID(F.NetworkID6); network6 != nil {
			network6.WGConfigID = cfg.ID
			_ = DB_UpdateNetwork(network6)
		}
	}

	sendObject(w, map[string]any{
		"ID":               cfg.ID.String(),
		"APIKey":           cfg.APIKey,
		"Tag":              cfg.Tag,
		"WireGuardPort":    cfg.WireGuardPort,
		"WireGuardSubnet":  subnet,
		"WireGuardSubnet6": subnet6,
		"WireGuardIface":   cfg.WireGuardIface,
		"InternetIface":    cfg.InternetIface,
	})
}

func API_WGServerConfigGet(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

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
	var cfgSubnet6 string
	if cfg.NetworkID6 != uuid.Nil {
		if net6, _ := DB_FindNetworkByID(cfg.NetworkID6); net6 != nil {
			cfgSubnet6 = net6.CIDR
		}
	}

	sendObject(w, map[string]any{
		"ID":                 cfg.ID.String(),
		"Tag":                cfg.Tag,
		"APIKey":             cfg.APIKey,
		"WireGuardPubKey":    cfg.WireGuardPubKey,
		"WireGuardPort":      cfg.WireGuardPort,
		"WireGuardSubnet":    cfgSubnet,
		"WireGuardSubnet6":   cfgSubnet6,
		"WireGuardIface":     cfg.WireGuardIface,
		"InternetIface":      cfg.InternetIface,
		"PacketInspection":   cfg.PacketInspection,
		"InsecureSkipVerify": cfg.InsecureSkipVerify,
		"NetworkID":          cfg.NetworkID.String(),
		"NetworkID6":         cfg.NetworkID6.String(),
	})
}

func API_WGServerConfigFetch(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

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
	var fetchSubnet6 string
	if wgCfg.NetworkID6 != uuid.Nil {
		if network6, _ := DB_FindNetworkByID(wgCfg.NetworkID6); network6 != nil {
			fetchSubnet6 = network6.CIDR
		}
	}

	var serverID, serverIP string
	servers, err := DB_FindAllServers()
	if err == nil {
		for _, s := range servers {
			if s.WGConfigID == wgCfg.ID {
				serverID = s.ID.String()
				serverIP = s.IP
				_ = DB_SetServerWGConfigID(s.ID, wgCfg, wgCfg.WireGuardPubKey, fetchSubnet, fetchSubnet6)
				break
			}
		}
	}

	resp := &types.WGServerConfigResponse{
		ServerID:           serverID,
		ServerIP:           serverIP,
		WireGuardPort:      wgCfg.WireGuardPort,
		WireGuardSubnet:    fetchSubnet,
		WireGuardSubnet6:   fetchSubnet6,
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
	NetworkID6     uuid.UUID `json:"NetworkID6"`
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

	if existing.NetworkID6 != F.NetworkID6 {
		if existing.NetworkID6 != uuid.Nil {
			if oldNet6, _ := DB_FindNetworkByID(existing.NetworkID6); oldNet6 != nil {
				oldNet6.WGConfigID = uuid.Nil
				_ = DB_UpdateNetwork(oldNet6)
			}
		}
		if F.NetworkID6 != uuid.Nil {
			if newNet6, _ := DB_FindNetworkByID(F.NetworkID6); newNet6 != nil {
				newNet6.WGConfigID = F.ID
				_ = DB_UpdateNetwork(newNet6)
			}
		}
	}

	existing.Tag = F.Tag
	existing.WireGuardPort = F.WireGuardPort
	existing.NetworkID = F.NetworkID
	existing.NetworkID6 = F.NetworkID6
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
	var assignSubnet6 string
	if wgCfg.NetworkID6 != uuid.Nil {
		if network6, _ := DB_FindNetworkByID(wgCfg.NetworkID6); network6 != nil {
			assignSubnet6 = network6.CIDR
		}
	}

	if err := DB_SetServerWGConfigID(F.ServerID, wgCfg, wgCfg.WireGuardPubKey, assignSubnet, assignSubnet6); err != nil {
		senderr(w, 500, "Failed to update server", slog.Any("err", err))
		return
	}

	sendObject(w, map[string]any{
		"WireGuardPubKey":  wgCfg.WireGuardPubKey,
		"WireGuardPort":    wgCfg.WireGuardPort,
		"WireGuardSubnet":  assignSubnet,
		"WireGuardSubnet6": assignSubnet6,
	})
}

func API_WGServers(w http.ResponseWriter, r *http.Request) {
	defer BasicRecover()

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
			WireGuardPubKey:  s.WireGuardPubKey,
			WireGuardPort:    s.WireGuardPort,
			WireGuardSubnet:  s.WireGuardSubnet,
			WireGuardSubnet6: s.WireGuardSubnet6,
			IP:               s.IP,
		})
	}

	sendObject(w, resp)
}

func buildWGConf(assignedIP, assignedIPv6, clientPubKeyB64 string, server *types.Server) string {
	address := assignedIP + "/32"
	if assignedIPv6 != "" {
		address += ", " + assignedIPv6 + "/128"
	}

	allowedIPs := "0.0.0.0/0"
	if assignedIPv6 != "" {
		allowedIPs += ", ::/0"
	}

	dns := "1.1.1.1"
	if assignedIPv6 != "" {
		dns += ", 2606:4700:4700::1111"
	}

	return fmt.Sprintf(`[Interface]
Address = %s
# PrivateKey = <paste your private key here>
DNS = %s

[Peer]
PublicKey = %s
Endpoint = %s:%s
AllowedIPs = %s
PersistentKeepalive = 15
`, address, dns, server.WireGuardPubKey, server.IP, server.WireGuardPort, allowedIPs)
}
